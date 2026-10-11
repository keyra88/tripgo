package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/database/txmanager"
	"github.com/keyra88/tripgo/internal/repository"
)

type TripService struct {
	repo      *repository.Repository
	txManager txmanager.TxManager
}

func NewTripService(repo *repository.Repository, txManager txmanager.TxManager) *TripService {
	return &TripService{
		repo:      repo,
		txManager: txManager,
	}
}

func computeRequestHash(data api.TripData) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s:%s:%s:%s:%s:%d",
		data.UserId.String(),
		data.DriverId.String(),
		strconv.FormatFloat(data.StartPoint.Latitude, 'g', -1, 64),
		strconv.FormatFloat(data.StartPoint.Longitude, 'g', -1, 64),
		strconv.FormatFloat(data.EndPoint.Latitude, 'g', -1, 64),
		strconv.FormatFloat(data.EndPoint.Longitude, 'g', -1, 64),
		data.Price,
	)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *TripService) CreateTrip(ctx context.Context, data api.TripData, idempotencyKey *uuid.UUID) (*api.Trip, bool, error) {
	if idempotencyKey == nil {
		trip, err := s.createTripInternal(ctx, data)
		return trip, false, err
	}

	reqHash := computeRequestHash(data)
	var (
		resTrip  *api.Trip
		isReplay bool
	)

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := s.repo.LockIdempotencyKey(txCtx, *idempotencyKey); err != nil {
			return fmt.Errorf("lock idempotency key: %w", err)
		}

		rec, err := s.repo.GetIdempotencyRecord(txCtx, *idempotencyKey)
		if err != nil {
			return fmt.Errorf("get idempotency record: %w", err)
		}

		now := time.Now().UTC()

		if rec != nil {
			if rec.ExpiresAt.After(now) {
				if rec.RequestHash == reqHash {
					existingTrip, err := s.repo.GetTripByID(txCtx, rec.TripID)
					if err != nil {
						return fmt.Errorf("get existing trip by idempotency: %w", err)
					}
					resTrip = existingTrip
					isReplay = true
					return nil
				}
				return ErrIdempotencyConflict
			}
		}

		tripID := uuid.New()
		trip := &api.Trip{
			Id:         tripID,
			UserId:     data.UserId,
			DriverId:   data.DriverId,
			StartPoint: data.StartPoint,
			EndPoint:   data.EndPoint,
			Price:      data.Price,
			Status:     api.Active,
			StartedAt:  now,
			FinishedAt: nil,
		}

		if err := s.repo.CreateTrip(txCtx, trip); err != nil {
			return err
		}

		if err := s.repo.AddStatusHistory(txCtx, tripID, nil, string(api.Active), nil, now); err != nil {
			return fmt.Errorf("add status history: %w", err)
		}

		newRec := &repository.IdempotencyRecord{
			Key:         *idempotencyKey,
			RequestHash: reqHash,
			TripID:      tripID,
			CreatedAt:   now,
			ExpiresAt:   now.Add(24 * time.Hour),
		}
		if err := s.repo.SaveIdempotencyRecord(txCtx, newRec); err != nil {
			return fmt.Errorf("save idempotency record: %w", err)
		}

		resTrip = trip
		isReplay = false
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return resTrip, isReplay, nil
}

func (s *TripService) createTripInternal(ctx context.Context, data api.TripData) (*api.Trip, error) {
	tripID := uuid.New()
	now := time.Now().UTC()

	trip := &api.Trip{
		Id:         tripID,
		UserId:     data.UserId,
		DriverId:   data.DriverId,
		StartPoint: data.StartPoint,
		EndPoint:   data.EndPoint,
		Price:      data.Price,
		Status:     api.Active,
		StartedAt:  now,
		FinishedAt: nil,
	}

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateTrip(txCtx, trip); err != nil {
			return err
		}

		if err := s.repo.AddStatusHistory(txCtx, tripID, nil, string(api.Active), nil, now); err != nil {
			return fmt.Errorf("add status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return trip, nil
}

func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (*api.Trip, error) {
	return s.repo.GetTripByID(ctx, id)
}

func (s *TripService) FinishTrip(ctx context.Context, id uuid.UUID) (*api.Trip, error) {
	var finishedTrip *api.Trip
	now := time.Now().UTC()

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		trip, err := s.repo.FinishTrip(txCtx, id, now)
		if err != nil {
			return err
		}

		fromStatus := string(api.Active)
		if err := s.repo.AddStatusHistory(txCtx, id, &fromStatus, string(api.Completed), nil, now); err != nil {
			return fmt.Errorf("add status history: %w", err)
		}

		finishedTrip = trip
		return nil
	})
	if err != nil {
		return nil, err
	}

	return finishedTrip, nil
}
