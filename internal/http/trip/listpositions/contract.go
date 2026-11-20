//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package listpositions

import (
	"context"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) ([]model.Position, error)
}
