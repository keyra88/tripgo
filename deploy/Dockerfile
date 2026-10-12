FROM golang:alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /build/trip-service \
    ./cmd/trip-service

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /build/trip-service /app/trip-service

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/trip-service"]
