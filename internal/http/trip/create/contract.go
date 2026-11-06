//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package create_test
package create

import (
	"context"

	"github.com/keyra88/tripgo/internal/model"
	createuc "github.com/keyra88/tripgo/internal/usecase/trip/create"
)

type usecase interface {
	Execute(ctx context.Context, in createuc.CreateTripInput) (*model.Trip, bool, error)
}
