# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY packages/statussnapshot /build/packages/statussnapshot
COPY services/control-tower-read-model-service /build/services/control-tower-read-model-service

WORKDIR /build/services/control-tower-read-model-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /control-tower-status-snapshot-import ./cmd/control-tower-status-snapshot-import

FROM alpine:3.19

ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG BINTRANS_IMAGE_SOURCE=https://github.com/freight-platform/freight-platform
LABEL org.opencontainers.image.revision="${BINTRANS_GIT_SHA}" \
      org.opencontainers.image.version="${BINTRANS_IMAGE_VERSION}" \
      org.opencontainers.image.source="${BINTRANS_IMAGE_SOURCE}"

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /server /app/control-tower-read-model-service
COPY --from=builder /control-tower-status-snapshot-import /app/control-tower-status-snapshot-import

EXPOSE 8089
ENTRYPOINT ["/app/control-tower-read-model-service"]
