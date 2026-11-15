package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/domain/trip"
)

func TestTrip_Finish(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		initial     trip.Trip
		finishAt    time.Time
		expectedErr error
	}{
		{
			name: "successful finish from active status",
			initial: trip.Trip{
				ID:        uuid.New(),
				Status:    trip.StatusActive,
				StartedAt: now.Add(-10 * time.Minute),
			},
			finishAt:    now,
			expectedErr: nil,
		},
		{
			name: "finish already completed trip returns ErrTripCompleted",
			initial: trip.Trip{
				ID:         uuid.New(),
				Status:     trip.StatusCompleted,
				StartedAt:  now.Add(-20 * time.Minute),
				FinishedAt: &now,
			},
			finishAt:    now,
			expectedErr: trip.ErrTripCompleted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := tc.initial
			err := tr.Finish(tc.finishAt)

			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if tc.expectedErr == nil {
				if tr.Status != trip.StatusCompleted {
					t.Errorf("expected status %s, got %s", trip.StatusCompleted, tr.Status)
				}
				if tr.FinishedAt == nil || !tr.FinishedAt.Equal(tc.finishAt) {
					t.Errorf("expected finished_at %v, got %v", tc.finishAt, tr.FinishedAt)
				}
			}
		})
	}
}

func TestTrip_Validation(t *testing.T) {
	validUserID := uuid.New()
	validDriverID := uuid.New()
	now := time.Now().UTC()

	tests := []struct {
		name      string
		trip      trip.Trip
		wantError bool
	}{
		{
			name: "valid active trip",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      500,
				Status:     trip.StatusActive,
				StartedAt:  now,
				FinishedAt: nil,
			},
			wantError: false,
		},
		{
			name: "nil user id",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     uuid.Nil,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      500,
				Status:     trip.StatusActive,
			},
			wantError: true,
		},
		{
			name: "nil driver id",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   uuid.Nil,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      500,
				Status:     trip.StatusActive,
			},
			wantError: true,
		},
		{
			name: "negative price",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      -100,
				Status:     trip.StatusActive,
			},
			wantError: true,
		},
		{
			name: "invalid start latitude",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 95.0, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      500,
				Status:     trip.StatusActive,
			},
			wantError: true,
		},
		{
			name: "invalid end longitude",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 190.0},
				Price:      500,
				Status:     trip.StatusActive,
			},
			wantError: true,
		},
		{
			name: "active trip with finished_at set",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      500,
				Status:     trip.StatusActive,
				FinishedAt: &now,
			},
			wantError: true,
		},
		{
			name: "completed trip without finished_at set",
			trip: trip.Trip{
				ID:         uuid.New(),
				UserID:     validUserID,
				DriverID:   validDriverID,
				StartPoint: trip.Point{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   trip.Point{Latitude: 55.76, Longitude: 37.62},
				Price:      500,
				Status:     trip.StatusCompleted,
				FinishedAt: nil,
			},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.trip.Validate()
			if (err != nil) != tc.wantError {
				t.Fatalf("Validate() error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}
