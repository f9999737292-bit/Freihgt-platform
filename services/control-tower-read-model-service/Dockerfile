# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY services/control-tower-read-model-service /build/services/control-tower-read-model-service

WORKDIR /build/services/control-tower-read-model-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /server /server

EXPOSE 8089
ENTRYPOINT ["/server"]
