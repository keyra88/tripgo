package service

import (
	"context"
	"fmt"
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

func (s *TripService) CreateTrip(ctx context.Context, data api.TripData) (*api.Trip, error) {
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
