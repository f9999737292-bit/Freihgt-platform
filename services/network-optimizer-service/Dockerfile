# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY services/network-optimizer-service /build/services/network-optimizer-service

WORKDIR /build/services/network-optimizer-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /network-optimizer-service ./cmd/server

FROM alpine:3.19

ARG SOURCE_REVISION=unknown
ARG IMAGE_SOURCE=https://github.com/f9999737292-bit/Freihgt-platform

RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 65532 -H app

COPY --from=builder /network-optimizer-service /app/network-optimizer-service
RUN chown app:app /app/network-optimizer-service

LABEL org.opencontainers.image.revision="${SOURCE_REVISION}" \
      org.opencontainers.image.source="${IMAGE_SOURCE}"

USER app
EXPOSE 8096
ENTRYPOINT ["/app/network-optimizer-service"]
