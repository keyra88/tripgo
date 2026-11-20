package position

import (
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/model"
)

func ToAPI(p model.Position) api.TripPosition {
	return api.TripPosition{
		Id:         p.ID,
		TripId:     p.TripID,
		RecordedAt: p.RecordedAt,
		Coordinates: api.Coordinates{
			Latitude:  p.Coordinates.Latitude,
			Longitude: p.Coordinates.Longitude,
		},
	}
}
