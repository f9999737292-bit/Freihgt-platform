# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY services/api-gateway /build/services/api-gateway
COPY packages/openapi /build/packages/openapi

WORKDIR /build/services/api-gateway
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /server /server
COPY --from=builder /build/packages/openapi /app/packages/openapi

ENV OPENAPI_DIR=/app/packages/openapi

EXPOSE 8080
ENTRYPOINT ["/server"]
