package consumer

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
	"time"
)

const (
	ErrorCodeBrokerUnavailable  = "BROKER_UNAVAILABLE"
	ErrorCodeFetchNetworkError  = "FETCH_NETWORK_ERROR"
	ErrorCodeAuthorizationError = "AUTHORIZATION_ERROR"
	ErrorCodeFetchProtocolError = "FETCH_PROTOCOL_ERROR"
	ErrorCodeUnknownPollError   = "UNKNOWN_POLL_ERROR"
)

type PollOutcome int

const (
	PollOutcomeOK PollOutcome = iota
	PollOutcomeIdle
	PollOutcomeShutdown
	PollOutcomeError
)

func ClassifyPollError(parentCtx, pollCtx context.Context, err error) (PollOutcome, string) {
	if err == nil {
		return PollOutcomeOK, ""
	}
	if parentCtx.Err() != nil {
		return PollOutcomeShutdown, ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return PollOutcomeIdle, ""
	}
	if errors.Is(err, context.Canceled) {
		if pollCtx.Err() != nil && parentCtx.Err() == nil {
			return PollOutcomeIdle, ""
		}
		return PollOutcomeShutdown, ""
	}
	return PollOutcomeError, SafePollErrorCode(err)
}

func SafePollErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "authorization"),
		strings.Contains(msg, "authentication"),
		strings.Contains(msg, "sasl"),
		strings.Contains(msg, "not authorized"):
		return ErrorCodeAuthorizationError
	case strings.Contains(msg, "protocol"):
		return ErrorCodeFetchProtocolError
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
			return ErrorCodeBrokerUnavailable
		}
		return ErrorCodeFetchNetworkError
	}
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "broker") {
		return ErrorCodeBrokerUnavailable
	}
	return ErrorCodeUnknownPollError
}

func nextPollBackoff(current, base time.Duration) time.Duration {
	if current <= 0 {
		if base <= 0 {
			return 250 * time.Millisecond
		}
		return base
	}
	next := current * 2
	const maxBackoff = 5 * time.Second
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}
