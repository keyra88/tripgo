//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package listpositions

import (
	"context"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type positionRepository interface {
	GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	ListPositions(ctx context.Context, id uuid.UUID) ([]model.Position, error)
}
