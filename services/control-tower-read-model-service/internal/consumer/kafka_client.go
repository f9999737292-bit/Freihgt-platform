package consumer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
)

func NewKafkaClient(cfg config.KafkaConfig) (*kgo.Client, error) {
	if err := cfg.ValidateRequired(); err != nil {
		return nil, err
	}
	opts, err := buildKafkaClientOptions(cfg)
	if err != nil {
		return nil, err
	}
	return kgo.NewClient(opts...)
}

func buildKafkaClientOptions(cfg config.KafkaConfig) ([]kgo.Opt, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.DialTimeout(cfg.DialTimeout),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topic),
		kgo.DisableAutoCommit(),
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
		return nil, fmt.Errorf("unsupported CONTROL_TOWER_KAFKA_SASL_MECHANISM %q", cfg.SASLMechanism)
	}

	return opts, nil
}

func buildTLSConfig(cfg config.KafkaConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.TLSServerName != "" {
		tlsCfg.ServerName = cfg.TLSServerName
	}
	if cfg.TLSCAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read CONTROL_TOWER_KAFKA_TLS_CA_FILE: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("invalid CONTROL_TOWER_KAFKA_TLS_CA_FILE")
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load CONTROL_TOWER_KAFKA TLS cert/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

type OffsetCommitter interface {
	CommitRecords(ctx context.Context, records ...*kgo.Record) error
}
