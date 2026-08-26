FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/payment-service /src/services/payment-service
WORKDIR /src/services/payment-service
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.19
ARG BINTRANS_GIT_SHA=
ARG BINTRANS_IMAGE_VERSION=
ARG BINTRANS_IMAGE_SOURCE=https://github.com/f9999737292-bit/Freihgt-platform
LABEL org.opencontainers.image.revision="${BINTRANS_GIT_SHA}" \
      org.opencontainers.image.version="${BINTRANS_IMAGE_VERSION}" \
      org.opencontainers.image.source="${BINTRANS_IMAGE_SOURCE}"
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /server /app/server
EXPOSE 8090
ENTRYPOINT ["/app/server"]
