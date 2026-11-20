package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

const nullJSON = "null"

func Coordinates(raw json.RawMessage, fieldName string) (*model.Point, error) {
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
