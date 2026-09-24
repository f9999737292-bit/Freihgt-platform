package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPPort             int
	DatabaseURL          string
	TransportOrderURL    string
	ShipmentURL          string
	TrackingURL          string
	InternalServiceToken string
	Prediction           PredictionConfig
}

type PredictionConfig struct {
	Unload          time.Duration
	Uncertainty     time.Duration
	MaxETAAge       time.Duration
	ConfidenceFloor float64
	AutoActivate    bool
}

func Load() (Config, error) {
	portRaw := os.Getenv("HTTP_PORT")
	if portRaw == "" {
		portRaw = "8096"
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 {
		return Config{}, fmt.Errorf("invalid HTTP_PORT")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	unload, err := optionalDuration("BNO_PREDICTION_DEFAULT_UNLOAD_DURATION")
	if err != nil {
		return Config{}, err
	}
	uncertainty, err := optionalDuration("BNO_PREDICTION_AVAILABILITY_UNCERTAINTY")
	if err != nil {
		return Config{}, err
	}
	maxETAAge, err := optionalDuration("BNO_PREDICTION_MAX_ETA_AGE")
	if err != nil {
		return Config{}, err
	}
	floor := 0.5
	if raw := strings.TrimSpace(os.Getenv("BNO_PREDICTION_CONFIDENCE_FLOOR")); raw != "" {
		floor, err = strconv.ParseFloat(raw, 64)
		if err != nil || floor < 0 || floor > 1 {
			return Config{}, fmt.Errorf("invalid BNO_PREDICTION_CONFIDENCE_FLOOR")
		}
	}
	return Config{
		InternalServiceToken: strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN")),
		HTTPPort:             port,
		DatabaseURL:          databaseURL,
		TransportOrderURL:    os.Getenv("TRANSPORT_ORDER_SERVICE_URL"),
		ShipmentURL:          os.Getenv("SHIPMENT_SERVICE_URL"),
		TrackingURL:          os.Getenv("TRACKING_SERVICE_URL"),
		Prediction: PredictionConfig{
			Unload: unload, Uncertainty: uncertainty, MaxETAAge: maxETAAge,
			ConfidenceFloor: floor,
			AutoActivate:    strings.EqualFold(strings.TrimSpace(os.Getenv("BNO_PREDICTION_AUTO_ACTIVATE")), "true"),
		},
	}, nil
}

func optionalDuration(key string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}
