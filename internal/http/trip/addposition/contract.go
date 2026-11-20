//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package addposition

import (
	"context"

	"github.com/keyra88/tripgo/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, input model.Position) (*model.Position, error)
}
