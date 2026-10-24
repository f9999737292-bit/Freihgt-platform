#!/bin/sh
set -eu
brokers=redpanda:9092
rpk topic create shipment.status.v1 driver.events.v1 -p 1 -r 1 --brokers "$brokers"
rpk topic describe shipment.status.v1 --brokers "$brokers"
rpk topic describe driver.events.v1 --brokers "$brokers"
