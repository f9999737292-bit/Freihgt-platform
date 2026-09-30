package outbox

import (
	"fmt"
	"strconv"

	"github.com/freight-platform/tracking-service/internal/repository"
)

const contentTypeJSON = "application/json"

type Header struct {
	Key   string
	Value string
}

type BusRecord struct {
	Topic   string
	Key     string
	Value   []byte
	Headers []Header
}

func BuildBusRecord(topic string, event repository.TrackingOutboxEvent) (BusRecord, error) {
	if topic == "" || event.EventType == "" || event.AggregateID.String() == "" {
		return BusRecord{}, fmt.Errorf("tracking outbox record is incomplete")
	}
	return BusRecord{
		Topic: topic,
		Key:   event.AggregateID.String(),
		Value: append([]byte(nil), event.Payload...),
		Headers: []Header{
			{Key: "event_type", Value: event.EventType},
			{Key: "schema_version", Value: strconv.Itoa(event.SchemaVersion)},
			{Key: "source_event_id", Value: event.SourceEventID.String()},
			{Key: "content_type", Value: contentTypeJSON},
		},
	}, nil
}
