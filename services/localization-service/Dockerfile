# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /build

COPY packages/shared-go /build/packages/shared-go
COPY services/localization-service /build/services/localization-service

WORKDIR /build/services/localization-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /server /server

EXPOSE 8083
USER nonroot:nonroot
ENTRYPOINT ["/server"]
