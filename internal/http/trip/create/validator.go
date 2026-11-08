package create

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
	createuc "github.com/keyra88/tripgo/internal/usecase/trip/create"
)

const nullJSON = "null"

func decodeAndValidateTripData(r *http.Request, idempotencyKey *uuid.UUID) (*createuc.CreateTripInput, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read request body: %v", domain.ErrInvalidRequest, err)
	}
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return nil, fmt.Errorf("%w: request body cannot be empty", domain.ErrInvalidRequest)
	}

	dec := json.NewDecoder(bytes.NewReader(bodyBytes))
	var rawTop map[string]json.RawMessage
	if err := dec.Decode(&rawTop); err != nil {
		return nil, fmt.Errorf("%w: invalid json object: %v", domain.ErrInvalidRequest, err)
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: unexpected trailing data after json object", domain.ErrInvalidRequest)
	}

	allowedTopKeys := map[string]struct{}{
		"user_id":     {},
		"driver_id":   {},
		"start_point": {},
		"end_point":   {},
		"price":       {},
	}
	for k := range rawTop {
		if _, ok := allowedTopKeys[k]; !ok {
			return nil, fmt.Errorf("%w: unknown or invalid field: %q", domain.ErrInvalidRequest, k)
		}
	}

	requiredKeys := []string{"user_id", "driver_id", "start_point", "end_point", "price"}
	for _, reqKey := range requiredKeys {
		val, exists := rawTop[reqKey]
		if !exists {
			return nil, fmt.Errorf("%w: missing required field: %q", domain.ErrInvalidRequest, reqKey)
		}
		if string(bytes.TrimSpace(val)) == nullJSON {
			return nil, fmt.Errorf("%w: field %q cannot be null", domain.ErrInvalidRequest, reqKey)
		}
	}

	var userIDStr string
	if err := json.Unmarshal(rawTop["user_id"], &userIDStr); err != nil {
		return nil, fmt.Errorf("%w: user_id must be a string", domain.ErrInvalidRequest)
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil || userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id must be a valid non-empty uuid", domain.ErrInvalidRequest)
	}

	var driverIDStr string
	if err := json.Unmarshal(rawTop["driver_id"], &driverIDStr); err != nil {
		return nil, fmt.Errorf("%w: driver_id must be a string", domain.ErrInvalidRequest)
	}
	driverID, err := uuid.Parse(driverIDStr)
	if err != nil || driverID == uuid.Nil {
		return nil, fmt.Errorf("%w: driver_id must be a valid non-empty uuid", domain.ErrInvalidRequest)
	}

	priceVal, err := parsePrice(rawTop["price"])
	if err != nil {
		return nil, err
	}

	startPoint, err := parseCoordinates(rawTop["start_point"], "start_point")
	if err != nil {
		return nil, err
	}

	endPoint, err := parseCoordinates(rawTop["end_point"], "end_point")
	if err != nil {
		return nil, err
	}

	return &createuc.CreateTripInput{
		UserID:         userID,
		DriverID:       driverID,
		StartPoint:     *startPoint,
		EndPoint:       *endPoint,
		Price:          priceVal,
		IdempotencyKey: idempotencyKey,
	}, nil
}

func parsePrice(raw json.RawMessage) (int64, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return 0, fmt.Errorf("%w: invalid price: %v", domain.ErrInvalidRequest, err)
	}
	num, ok := val.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%w: price must be a number", domain.ErrInvalidRequest)
	}
	p, err := num.Int64()
	if err != nil {
		return 0, fmt.Errorf("%w: price must be an integer: %v", domain.ErrInvalidRequest, err)
	}
	if p < 0 {
		return 0, fmt.Errorf("%w: price must be >= 0", domain.ErrInvalidRequest)
	}
	return p, nil
}

func parseCoordinates(raw json.RawMessage, fieldName string) (*model.Point, error) {
	if string(bytes.TrimSpace(raw)) == nullJSON {
		return nil, fmt.Errorf("%w: %s cannot be null", domain.ErrInvalidRequest, fieldName)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	var rawCoords map[string]json.RawMessage
	if err := dec.Decode(&rawCoords); err != nil {
		return nil, fmt.Errorf("%w: %s must be a json object", domain.ErrInvalidRequest, fieldName)
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data in %s", domain.ErrInvalidRequest, fieldName)
	}

	allowed := map[string]struct{}{
		"latitude":  {},
		"longitude": {},
	}
	for k := range rawCoords {
		if _, ok := allowed[k]; !ok {
			return nil, fmt.Errorf("%w: unknown or invalid field %q in %s", domain.ErrInvalidRequest, k, fieldName)
		}
	}

	latRaw, okLat := rawCoords["latitude"]
	if !okLat || string(bytes.TrimSpace(latRaw)) == nullJSON {
		return nil, fmt.Errorf("%w: %s.latitude is required and cannot be null", domain.ErrInvalidRequest, fieldName)
	}

	lonRaw, okLon := rawCoords["longitude"]
	if !okLon || string(bytes.TrimSpace(lonRaw)) == nullJSON {
		return nil, fmt.Errorf("%w: %s.longitude is required and cannot be null", domain.ErrInvalidRequest, fieldName)
	}

	latVal, err := parseJSONFloat(latRaw, fieldName+".latitude")
	if err != nil {
		return nil, err
	}
	if latVal < -90 || latVal > 90 {
		return nil, fmt.Errorf("%w: %s.latitude out of range [-90, 90]", domain.ErrInvalidRequest, fieldName)
	}

	lonVal, err := parseJSONFloat(lonRaw, fieldName+".longitude")
	if err != nil {
		return nil, err
	}
	if lonVal < -180 || lonVal > 180 {
		return nil, fmt.Errorf("%w: %s.longitude out of range [-180, 180]", domain.ErrInvalidRequest, fieldName)
	}

	return &model.Point{
		Latitude:  latVal,
		Longitude: lonVal,
	}, nil
}

func parseJSONFloat(raw json.RawMessage, name string) (float64, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return 0, fmt.Errorf("%w: %s must be a number", domain.ErrInvalidRequest, name)
	}
	num, ok := val.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%w: %s must be a number", domain.ErrInvalidRequest, name)
	}
	f, err := num.Float64()
	if err != nil {
		return 0, fmt.Errorf("%w: %s invalid float: %v", domain.ErrInvalidRequest, name, err)
	}
	return f, nil
}
