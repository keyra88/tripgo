package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/service"
)

func decodeAndValidateTripData(r *http.Request) (*api.TripData, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read request body: %v", service.ErrInvalidRequest, err)
	}
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return nil, fmt.Errorf("%w: request body cannot be empty", service.ErrInvalidRequest)
	}

	dec := json.NewDecoder(bytes.NewReader(bodyBytes))
	var rawTop map[string]json.RawMessage
	if err := dec.Decode(&rawTop); err != nil {
		return nil, fmt.Errorf("%w: invalid json object: %v", service.ErrInvalidRequest, err)
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: unexpected trailing data after json object", service.ErrInvalidRequest)
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
			return nil, fmt.Errorf("%w: unknown or invalid field: %q", service.ErrInvalidRequest, k)
		}
	}

	requiredKeys := []string{"user_id", "driver_id", "start_point", "end_point", "price"}
	for _, reqKey := range requiredKeys {
		val, exists := rawTop[reqKey]
		if !exists {
			return nil, fmt.Errorf("%w: missing required field: %q", service.ErrInvalidRequest, reqKey)
		}
		if string(bytes.TrimSpace(val)) == "null" {
			return nil, fmt.Errorf("%w: field %q cannot be null", service.ErrInvalidRequest, reqKey)
		}
	}

	var userIDStr string
	if err := json.Unmarshal(rawTop["user_id"], &userIDStr); err != nil {
		return nil, fmt.Errorf("%w: user_id must be a string", service.ErrInvalidRequest)
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil || userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id must be a valid non-empty uuid", service.ErrInvalidRequest)
	}

	var driverIDStr string
	if err := json.Unmarshal(rawTop["driver_id"], &driverIDStr); err != nil {
		return nil, fmt.Errorf("%w: driver_id must be a string", service.ErrInvalidRequest)
	}
	driverID, err := uuid.Parse(driverIDStr)
	if err != nil || driverID == uuid.Nil {
		return nil, fmt.Errorf("%w: driver_id must be a valid non-empty uuid", service.ErrInvalidRequest)
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

	return &api.TripData{
		UserId:     userID,
		DriverId:   driverID,
		StartPoint: *startPoint,
		EndPoint:   *endPoint,
		Price:      priceVal,
	}, nil
}

func parsePrice(raw json.RawMessage) (int64, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return 0, fmt.Errorf("%w: invalid price: %v", service.ErrInvalidRequest, err)
	}
	num, ok := val.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%w: price must be a number", service.ErrInvalidRequest)
	}
	p, err := num.Int64()
	if err != nil {
		return 0, fmt.Errorf("%w: price must be an integer: %v", service.ErrInvalidRequest, err)
	}
	if p < 0 {
		return 0, fmt.Errorf("%w: price must be >= 0", service.ErrInvalidRequest)
	}
	return p, nil
}

func parseCoordinates(raw json.RawMessage, fieldName string) (*api.Coordinates, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil, fmt.Errorf("%w: %s cannot be null", service.ErrInvalidRequest, fieldName)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	var rawCoords map[string]json.RawMessage
	if err := dec.Decode(&rawCoords); err != nil {
		return nil, fmt.Errorf("%w: %s must be a json object", service.ErrInvalidRequest, fieldName)
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data in %s", service.ErrInvalidRequest, fieldName)
	}

	allowed := map[string]struct{}{
		"latitude":  {},
		"longitude": {},
	}
	for k := range rawCoords {
		if _, ok := allowed[k]; !ok {
			return nil, fmt.Errorf("%w: unknown or invalid field %q in %s", service.ErrInvalidRequest, k, fieldName)
		}
	}

	latRaw, okLat := rawCoords["latitude"]
	if !okLat || string(bytes.TrimSpace(latRaw)) == "null" {
		return nil, fmt.Errorf("%w: %s.latitude is required and cannot be null", service.ErrInvalidRequest, fieldName)
	}

	lonRaw, okLon := rawCoords["longitude"]
	if !okLon || string(bytes.TrimSpace(lonRaw)) == "null" {
		return nil, fmt.Errorf("%w: %s.longitude is required and cannot be null", service.ErrInvalidRequest, fieldName)
	}

	latVal, err := parseJSONFloat(latRaw, fieldName+".latitude")
	if err != nil {
		return nil, err
	}
	if latVal < -90 || latVal > 90 {
		return nil, fmt.Errorf("%w: %s.latitude out of range [-90, 90]", service.ErrInvalidRequest, fieldName)
	}

	lonVal, err := parseJSONFloat(lonRaw, fieldName+".longitude")
	if err != nil {
		return nil, err
	}
	if lonVal < -180 || lonVal > 180 {
		return nil, fmt.Errorf("%w: %s.longitude out of range [-180, 180]", service.ErrInvalidRequest, fieldName)
	}

	return &api.Coordinates{
		Latitude:  latVal,
		Longitude: lonVal,
	}, nil
}

func parseJSONFloat(raw json.RawMessage, name string) (float64, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return 0, fmt.Errorf("%w: %s must be a number", service.ErrInvalidRequest, name)
	}
	num, ok := val.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%w: %s must be a number", service.ErrInvalidRequest, name)
	}
	f, err := num.Float64()
	if err != nil {
		return 0, fmt.Errorf("%w: %s invalid float: %v", service.ErrInvalidRequest, name, err)
	}
	return f, nil
}
