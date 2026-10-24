# syntax=docker/dockerfile:1

# Build context is the repository root.
# Module download happens during image build, before the container joins the
# internal stand network. Runtime fetches are disabled.
FROM golang:1.25-bookworm

WORKDIR /src

COPY packages/shared-go /src/packages/shared-go
COPY packages/statussnapshot /src/packages/statussnapshot
COPY services/contract-rate-service /src/services/contract-rate-service
COPY services/transport-order-service /src/services/transport-order-service
COPY services/rfx-service /src/services/rfx-service
COPY services/shipment-service /src/services/shipment-service
COPY services/document-service /src/services/document-service
COPY services/billing-register-service /src/services/billing-register-service
COPY services/control-tower-read-model-service /src/services/control-tower-read-model-service
COPY tests/bintrans-e2e-transport-flow /src/tests/bintrans-e2e-transport-flow
COPY infrastructure/migrations /src/infrastructure/migrations
COPY infrastructure/docker/bintrans-e2e-stand/runner-entrypoint.sh /usr/local/bin/runner-entrypoint.sh

ENV GOWORK=off

RUN chmod +x /usr/local/bin/runner-entrypoint.sh \
    && mkdir -p /var/lib/bintrans-e2e-stand/documents \
    && cd /src/packages/shared-go && go mod download \
    && cd /src/packages/statussnapshot && go mod download \
    && cd /src/services/contract-rate-service && go mod download \
    && cd /src/services/transport-order-service && go mod download \
    && cd /src/services/rfx-service && go mod download \
    && cd /src/services/shipment-service && go mod download \
    && cd /src/services/document-service && go mod download \
    && cd /src/services/billing-register-service && go mod download \
    && cd /src/services/control-tower-read-model-service && go mod download \
    && cd /src/tests/bintrans-e2e-transport-flow && go mod download

ENV GOPROXY=off
ENV GOSUMDB=off
ENV GOFLAGS=-mod=readonly

ENTRYPOINT ["/usr/local/bin/runner-entrypoint.sh"]
