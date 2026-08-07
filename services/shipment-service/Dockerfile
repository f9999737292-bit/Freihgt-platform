# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY packages/statussnapshot /build/packages/statussnapshot
COPY services/shipment-service /build/services/shipment-service

WORKDIR /build/services/shipment-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /shipment-status-snapshot-export ./cmd/shipment-status-snapshot-export

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /server /app/shipment-service
COPY --from=builder /shipment-status-snapshot-export /app/shipment-status-snapshot-export

EXPOSE 8085
ENTRYPOINT ["/app/shipment-service"]
