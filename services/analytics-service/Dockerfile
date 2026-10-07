# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY services/analytics-service /build/services/analytics-service

WORKDIR /build/services/analytics-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /analytics-service ./cmd/server

FROM alpine:3.19

ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG BINTRANS_IMAGE_SOURCE=https://github.com/f9999737292-bit/Freihgt-platform
LABEL org.opencontainers.image.revision="${BINTRANS_GIT_SHA}" \
      org.opencontainers.image.version="${BINTRANS_IMAGE_VERSION}" \
      org.opencontainers.image.source="${BINTRANS_IMAGE_SOURCE}"

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 65532 appuser

COPY --from=builder /analytics-service /app/analytics-service
USER 65532
EXPOSE 8097
ENTRYPOINT ["/app/analytics-service"]
