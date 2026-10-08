# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY services/tracking-service /build/services/tracking-service

WORKDIR /build/services/tracking-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tracking-service ./cmd/server

FROM alpine:3.19

ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG BINTRANS_IMAGE_SOURCE=https://github.com/f9999737292-bit/Freihgt-platform

RUN set -eu; \
    i=1; \
    while [ "$i" -le 5 ]; do \
      apk add --no-cache ca-certificates wget && break; \
      i=$((i + 1)); \
      sleep 3; \
    done; \
    wget --version >/dev/null; \
    adduser -D -u 65532 -H app

COPY --from=builder /tracking-service /app/tracking-service
RUN chown app:app /app/tracking-service

LABEL org.opencontainers.image.revision="${BINTRANS_GIT_SHA}" \
      org.opencontainers.image.version="${BINTRANS_IMAGE_VERSION}" \
      org.opencontainers.image.source="${BINTRANS_IMAGE_SOURCE}"

USER app
EXPOSE 8089
ENTRYPOINT ["/app/tracking-service"]
