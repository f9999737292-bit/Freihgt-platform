package push

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrProviderUnavailable = errors.New("push provider unavailable")
	ErrInvalidToken        = errors.New("invalid push token")
	ErrTransient           = errors.New("transient push failure")
)

type Message struct {
	TaskID   string
	TaskType string
	Title    string
	Token    string
}

type SendResult struct {
	ProviderMessageID string
}

type Provider interface {
	Name() string
	Available() bool
	Send(ctx context.Context, msg Message) (SendResult, error)
}

func ClassifyError(err error) (permanent bool, code string) {
	if err == nil {
		return false, ""
	}
	if errors.Is(err, ErrInvalidToken) {
		return true, "INVALID_TOKEN"
	}
	if errors.Is(err, ErrProviderUnavailable) {
		return false, "PROVIDER_UNAVAILABLE"
	}
	if errors.Is(err, ErrTransient) {
		return false, "TRANSIENT"
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unregistered") || strings.Contains(msg, "invalid") {
		return true, "INVALID_TOKEN"
	}
	return false, "UNKNOWN"
}
