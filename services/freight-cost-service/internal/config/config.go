package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ServiceName            string
	Environment            string
	HTTPPort               int
	LogLevel               string
	DatabaseURL            string
	InternalServiceToken   string
	TransportOrderURL      string
	BillingRegisterURL     string
	PaymentServiceURL      string
}

func Load() (Config, error) {
	portRaw := os.Getenv("FREIGHT_COST_SERVICE_PORT")
	if portRaw == "" {
		portRaw = os.Getenv("HTTP_PORT")
	}
	if portRaw == "" {
		portRaw = "8092"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid FREIGHT_COST_SERVICE_PORT: %w", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://freight:freight_password@localhost:5432/freight_platform?sslmode=disable"
	}

	transportURL := os.Getenv("TRANSPORT_ORDER_SERVICE_URL")
	if transportURL == "" {
		transportURL = "http://transport-order-service:8083"
	}

	billingURL := os.Getenv("BILLING_REGISTER_SERVICE_URL")
	if billingURL == "" {
		billingURL = "http://billing-register-service:8087"
	}

	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://payment-service:8090"
	}

	return Config{
		ServiceName:          "freight-cost-service",
		Environment:          getEnv("ENVIRONMENT", "development"),
		HTTPPort:             port,
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		DatabaseURL:          databaseURL,
		InternalServiceToken: os.Getenv("INTERNAL_SERVICE_TOKEN"),
		TransportOrderURL:    transportURL,
		BillingRegisterURL:   billingURL,
		PaymentServiceURL:    paymentURL,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
