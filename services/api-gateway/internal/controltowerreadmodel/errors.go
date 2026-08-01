package controltowerreadmodel

import "fmt"

type FailureReason string

const (
	ReasonTimeout            FailureReason = "TIMEOUT"
	ReasonNetworkError       FailureReason = "NETWORK_ERROR"
	ReasonNon2XX             FailureReason = "NON_2XX"
	ReasonMalformedResponse  FailureReason = "MALFORMED_RESPONSE"
	ReasonInvalidContract    FailureReason = "INVALID_CONTRACT"
	ReasonConsumerNotRunning FailureReason = "CONSUMER_NOT_RUNNING"
	ReasonCancelled          FailureReason = "CANCELLED"
	ReasonUnknown            FailureReason = "UNKNOWN"
)

type DependencyError struct {
	Reason FailureReason
	Status int
	Err    error
}

func (e *DependencyError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Reason, e.Err)
	}
	return string(e.Reason)
}

func (e *DependencyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func classifyHTTPStatus(status int) FailureReason {
	if status >= 500 {
		return ReasonNon2XX
	}
	return ReasonNon2XX
}
