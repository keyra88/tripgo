//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package get_test
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
