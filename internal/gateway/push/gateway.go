package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Gateway struct {
	endpoint string
	client   *http.Client
	tracer   trace.Tracer
}

type request struct {
	RequestID   uuid.UUID         `json:"request_id"`
	RecipientID uuid.UUID         `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data"`
}

type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("push service returned HTTP %d", e.StatusCode)
}

func New(baseURL *url.URL, timeout time.Duration) *Gateway {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Gateway{
		endpoint: baseURL.JoinPath("api/v1/push").String(),
		tracer:   otel.Tracer("tripgo"),
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (g *Gateway) RequestPosition(ctx context.Context, tripID, driverID uuid.UUID) (err error) {
	ctx, span := g.tracer.Start(ctx, "POST /api/v1/push", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("http.request.method", http.MethodPost)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "push request failed")
		}
		span.End()
	}()

	body, err := json.Marshal(request{
		RequestID:   uuid.New(),
		RecipientID: driverID,
		Kind:        "REQUEST_POSITION",
		Data:        map[string]string{"trip_id": tripID.String()},
	})
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// The endpoint comes from validated service configuration.
	resp, err := g.client.Do(req) //nolint:gosec
	if err != nil {
		return fmt.Errorf("send push request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain small replies to allow connection reuse.
	_, _ = io.CopyN(io.Discard, resp.Body, 4<<10)

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	if resp.StatusCode != http.StatusAccepted {
		return &StatusError{StatusCode: resp.StatusCode}
	}
	return nil
}

func (g *Gateway) Close() {
	g.client.CloseIdleConnections()
}
