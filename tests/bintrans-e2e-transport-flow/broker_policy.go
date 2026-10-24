package bintranse2e

import (
	"errors"
	"fmt"
	"strings"
)

// brokerPlan is the fail-closed choice for BINTRANS_E2E_STAND_MODE.
// UseKfake is true only for ci mode with BINTRANS_E2E_KFAKE_ALLOW=1.
// Stand mode never sets UseKfake, including when the Redpanda dial fails.
type brokerPlan struct {
	Broker   string
	UseKfake bool
}

func brokerLogLines(mode, broker string) (string, string) {
	return "bintrans_e2e_mode=" + mode, "bintrans_e2e_broker=" + broker
}

func decideBroker(mode, broker, allow string, dial func(string) error) (brokerPlan, error) {
	mode = strings.TrimSpace(mode)
	broker = strings.TrimSpace(broker)
	allow = strings.TrimSpace(allow)
	switch mode {
	case "stand":
		if allow != "" {
			return brokerPlan{}, errors.New("stand mode rejects BINTRANS_E2E_KFAKE_ALLOW")
		}
		if broker == "" {
			return brokerPlan{}, errors.New("stand mode requires BINTRANS_E2E_KAFKA_BROKERS")
		}
		if token := forbiddenBrokerToken(broker); token != "" {
			return brokerPlan{}, fmt.Errorf("stand mode rejects broker token %s", token)
		}
		if broker != standBrokerAddress {
			return brokerPlan{}, fmt.Errorf("stand mode rejects broker %q", broker)
		}
		if dial == nil {
			return brokerPlan{}, errors.New("stand mode requires a broker dial")
		}
		if err := dial(broker); err != nil {
			return brokerPlan{}, fmt.Errorf("stand mode broker unavailable: %w", err)
		}
		return brokerPlan{Broker: broker}, nil
	case "ci":
		if allow != "1" {
			return brokerPlan{}, errors.New("ci mode requires BINTRANS_E2E_KFAKE_ALLOW=1")
		}
		return brokerPlan{UseKfake: true}, nil
	default:
		if mode == "" {
			return brokerPlan{}, errors.New("BINTRANS_E2E_STAND_MODE is required")
		}
		return brokerPlan{}, fmt.Errorf("BINTRANS_E2E_STAND_MODE %q is not stand or ci", mode)
	}
}

const standBrokerAddress = "redpanda:9092"

func forbiddenBrokerToken(broker string) string {
	lower := strings.ToLower(broker)
	switch {
	case strings.Contains(lower, "kfake"):
		return "kfake"
	case strings.Contains(lower, "localhost"):
		return "localhost"
	case strings.Contains(broker, "127.0.0.1"):
		return "127.0.0.1"
	case strings.Contains(broker, "::1"):
		return "::1"
	default:
		return ""
	}
}
