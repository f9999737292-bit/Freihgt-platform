FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/freight-cost-service /src/services/freight-cost-service
WORKDIR /src/services/freight-cost-service
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /server /app/server
EXPOSE 8092
ENTRYPOINT ["/app/server"]
