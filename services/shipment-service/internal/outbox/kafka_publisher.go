package outbox

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
	shipmentmetrics "github.com/freight-platform/shipment-service/internal/platform/metrics"
)

const kafkaContentType = "application/json"

var allowedKafkaHeaderKeys = map[string]struct{}{
	"event_type":      {},
	"schema_version":  {},
	"source_event_id": {},
	"correlation_id":  {},
	"content_type":    {},
}

type KafkaPublisher struct {
	client    *kgo.Client
	topic     string
	clock     Clock
	closeOnce sync.Once
}

type CloseablePublisher interface {
	EventPublisher
	Close(ctx context.Context) error
}

func NewKafkaPublisher(cfg config.KafkaConfig, clock Clock) (*KafkaPublisher, error) {
	if clock == nil {
		clock = NewRealClock()
	}
	if err := cfg.ValidateRequired(); err != nil {
		return nil, err
	}

	opts, err := buildKafkaClientOptions(cfg)
	if err != nil {
		return nil, &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: err}
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: err}
	}

	return &KafkaPublisher{
		client: client,
		topic:  cfg.Topic,
		clock:  clock,
	}, nil
}

func buildKafkaClientOptions(cfg config.KafkaConfig) ([]kgo.Opt, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.DialTimeout(cfg.DialTimeout),
		kgo.RequestTimeoutOverhead(cfg.WriteTimeout),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordRetries(3),
		kgo.ProduceRequestTimeout(cfg.WriteTimeout),
	}

	if cfg.TLSEnabled {
		tlsCfg, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	switch strings.ToLower(strings.TrimSpace(cfg.SASLMechanism)) {
	case "":
	case "plain":
		opts = append(opts, kgo.SASL(plain.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}.AsMechanism()))
	case "scram-sha-256":
		opts = append(opts, kgo.SASL(scram.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}.AsSha256Mechanism()))
	case "scram-sha-512":
		opts = append(opts, kgo.SASL(scram.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}.AsSha512Mechanism()))
	default:
		return nil, fmt.Errorf("unsupported SHIPMENT_KAFKA_SASL_MECHANISM %q", cfg.SASLMechanism)
	}

	return opts, nil
}

func buildTLSConfig(cfg config.KafkaConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if cfg.TLSServerName != "" {
		tlsCfg.ServerName = cfg.TLSServerName
	}
	if cfg.TLSCAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read SHIPMENT_KAFKA_TLS_CA_FILE: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("invalid SHIPMENT_KAFKA_TLS_CA_FILE")
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load SHIPMENT_KAFKA_TLS client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

func (p *KafkaPublisher) Publish(ctx context.Context, event domain.ShipmentOutboxEvent) error {
	start := p.clock.Now()
	if err := validateKafkaPublishEvent(event); err != nil {
		shipmentmetrics.ObserveKafkaPublish(event.EventType, "error", ErrorCodePayloadRejected, p.clock.Now().Sub(start))
		return err
	}

	record, err := buildKafkaRecord(p.topic, event)
	if err != nil {
		shipmentmetrics.ObserveKafkaPublish(event.EventType, "error", ErrorCodePayloadRejected, p.clock.Now().Sub(start))
		return err
	}

	results := p.client.ProduceSync(ctx, record)
	for _, result := range results {
		if result.Err == nil {
			shipmentmetrics.ObserveKafkaPublish(event.EventType, "success", "", p.clock.Now().Sub(start))
			return nil
		}
		classified := classifyKafkaError(result.Err)
		shipmentmetrics.ObserveKafkaPublish(event.EventType, "error", classified.Code, p.clock.Now().Sub(start))
		return classified
	}

	classified := &PublishError{Code: ErrorCodeUnknownPublishError, Retryable: true, Err: errors.New("kafka produce returned no results")}
	shipmentmetrics.ObserveKafkaPublish(event.EventType, "error", classified.Code, p.clock.Now().Sub(start))
	return classified
}

func (p *KafkaPublisher) Close(ctx context.Context) error {
	var closeErr error
	p.closeOnce.Do(func() {
		done := make(chan struct{})
		go func() {
			p.client.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			closeErr = ctx.Err()
		}
	})
	return closeErr
}

func validateKafkaPublishEvent(event domain.ShipmentOutboxEvent) error {
	if event.AggregateID == uuid.Nil {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: errors.New("aggregate_id must not be empty")}
	}
	if strings.TrimSpace(event.EventType) == "" || !isKnownKafkaEventType(event.EventType) {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: errors.New("unknown or empty event_type")}
	}
	if len(event.Payload) == 0 {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: errors.New("payload is required")}
	}
	return nil
}

func isKnownKafkaEventType(eventType string) bool {
	switch eventType {
	case domain.OutboxEventTypeCreated,
		domain.OutboxEventTypeStatusChanged,
		domain.OutboxEventTypeCancelled,
		domain.OutboxEventTypeReadyForBilling,
		domain.OutboxEventTypeDocumentsCompleted,
		domain.OutboxEventTypeFinanciallyClosed:
		return true
	default:
		return false
	}
}

func buildKafkaRecord(topic string, event domain.ShipmentOutboxEvent) (*kgo.Record, error) {
	var envelope domain.ShipmentStatusEventEnvelope
	if err := json.Unmarshal(event.Payload, &envelope); err != nil {
		return nil, &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: fmt.Errorf("invalid event envelope: %w", err)}
	}

	headers := []kgo.RecordHeader{
		{Key: "event_type", Value: []byte(event.EventType)},
		{Key: "schema_version", Value: []byte(strconv.Itoa(event.SchemaVersion))},
		{Key: "source_event_id", Value: []byte(event.SourceEventID.String())},
		{Key: "content_type", Value: []byte(kafkaContentType)},
	}
	if envelope.CorrelationID != nil && strings.TrimSpace(*envelope.CorrelationID) != "" {
		headers = append(headers, kgo.RecordHeader{Key: "correlation_id", Value: []byte(*envelope.CorrelationID)})
	}

	for _, header := range headers {
		if _, ok := allowedKafkaHeaderKeys[header.Key]; !ok {
			return nil, &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: fmt.Errorf("disallowed kafka header %q", header.Key)}
		}
	}

	return &kgo.Record{
		Topic:   topic,
		Key:     []byte(event.AggregateID.String()),
		Value:   append([]byte(nil), event.Payload...),
		Headers: headers,
	}, nil
}

func classifyKafkaError(err error) *PublishError {
	if err == nil {
		return nil
	}
	var publishErr *PublishError
	if errors.As(err, &publishErr) {
		return publishErr
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &PublishError{Code: ErrorCodeTransientTimeout, Retryable: true, Err: err}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return &PublishError{Code: ErrorCodeTransientTimeout, Retryable: true, Err: err}
		}
		return &PublishError{Code: ErrorCodeTransientNetwork, Retryable: true, Err: err}
	}

	var kerrTyped *kerr.Error
	if errors.As(err, &kerrTyped) && kerrTyped != nil {
		switch {
		case errors.Is(err, kerr.UnknownTopicOrPartition), errors.Is(err, kerr.InvalidTopicException),
			errors.Is(err, kerr.TopicAuthorizationFailed), errors.Is(err, kerr.ClusterAuthorizationFailed),
			errors.Is(err, kerr.SaslAuthenticationFailed), errors.Is(err, kerr.UnsupportedSaslMechanism),
			errors.Is(err, kerr.IllegalSaslState), errors.Is(err, kerr.InvalidRequiredAcks),
			errors.Is(err, kerr.PolicyViolation):
			return &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: err}
		case errors.Is(err, kerr.MessageTooLarge), errors.Is(err, kerr.CorruptMessage),
			errors.Is(err, kerr.InvalidRecord):
			return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: err}
		case errors.Is(err, kerr.NotLeaderForPartition), errors.Is(err, kerr.LeaderNotAvailable),
			errors.Is(err, kerr.NetworkException), errors.Is(err, kerr.RequestTimedOut),
			errors.Is(err, kerr.BrokerNotAvailable), errors.Is(err, kerr.CoordinatorNotAvailable):
			return &PublishError{Code: ErrorCodeBrokerUnavailable, Retryable: true, Err: err}
		default:
			if kerr.IsRetriable(err) {
				return &PublishError{Code: ErrorCodeBrokerUnavailable, Retryable: true, Err: err}
			}
			return &PublishError{Code: ErrorCodeUnknownPublishError, Retryable: true, Err: err}
		}
	}

	if strings.Contains(strings.ToLower(err.Error()), "connection refused") ||
		strings.Contains(strings.ToLower(err.Error()), "no route to host") ||
		strings.Contains(strings.ToLower(err.Error()), "broken pipe") {
		return &PublishError{Code: ErrorCodeBrokerUnavailable, Retryable: true, Err: err}
	}

	return &PublishError{Code: ErrorCodeUnknownPublishError, Retryable: true, Err: err}
}

func NewKafkaPublisherWithClient(client *kgo.Client, topic string, clock Clock) *KafkaPublisher {
	if clock == nil {
		clock = NewRealClock()
	}
	return &KafkaPublisher{client: client, topic: topic, clock: clock}
}
