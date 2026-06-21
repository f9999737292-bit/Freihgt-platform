# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build
COPY packages/shared-go /build/packages/shared-go
COPY services/document-service /build/services/document-service

WORKDIR /build/services/document-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /server /server

EXPOSE 8086
ENTRYPOINT ["/server"]
