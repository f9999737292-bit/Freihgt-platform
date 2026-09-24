package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPPort          int
	DatabaseURL       string
	TransportOrderURL string
	ShipmentURL       string
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
	return Config{
		HTTPPort:          port,
		DatabaseURL:       databaseURL,
		TransportOrderURL: os.Getenv("TRANSPORT_ORDER_SERVICE_URL"),
		ShipmentURL:       os.Getenv("SHIPMENT_SERVICE_URL"),
	}, nil
}
