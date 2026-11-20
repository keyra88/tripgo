package addposition

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/request"
	"github.com/keyra88/tripgo/internal/model"
)

var rfc3339 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(\.\d+)?([Zz]|[+-]([01]\d|2[0-3]):[0-5]\d)$`)

func decodePosition(body io.Reader, id uuid.UUID) (model.Position, error) {
	dec := json.NewDecoder(body)
	var fields map[string]json.RawMessage
	if err := dec.Decode(&fields); err != nil {
		return model.Position{}, fmt.Errorf("%w: invalid position body: %w", domain.ErrInvalidRequest, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return model.Position{}, fmt.Errorf("%w: unexpected trailing data after json object", domain.ErrInvalidRequest)
	}

	coordinates, hasCoordinates := fields["coordinates"]
	recordedAt, hasRecordedAt := fields["recorded_at"]
	if !hasCoordinates || !hasRecordedAt || len(fields) != 2 {
		return model.Position{}, fmt.Errorf("%w: expected coordinates and recorded_at", domain.ErrInvalidRequest)
	}
	point, err := request.Coordinates(coordinates, "coordinates")
	if err != nil {
		return model.Position{}, err
	}

	var timestamp string
	if err := json.Unmarshal(recordedAt, &timestamp); err != nil || !rfc3339.MatchString(timestamp) {
		return model.Position{}, fmt.Errorf("%w: recorded_at must be an RFC3339 timestamp", domain.ErrInvalidRequest)
	}
	at, err := time.Parse(time.RFC3339, strings.ToUpper(timestamp))
	if err != nil {
		return model.Position{}, fmt.Errorf("%w: recorded_at must be an RFC3339 timestamp", domain.ErrInvalidRequest)
	}
	return model.Position{TripID: id, Coordinates: *point, RecordedAt: at}, nil
}
