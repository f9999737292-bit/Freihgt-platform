package sourceverify

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var ErrUnavailable = errors.New("source verification unavailable")

type Verifier interface {
	Owns(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (bool, error)
}

type MapVerifier struct {
	Owned map[string]bool
}

func (m MapVerifier) Owns(_ context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (bool, error) {
	if m.Owned == nil {
		return false, nil
	}
	return m.Owned[fmt.Sprintf("%s|%s|%s", tenantID, sourceType, sourceID)], nil
}

type Unavailable struct{}

func (Unavailable) Owns(context.Context, uuid.UUID, string, uuid.UUID) (bool, error) {
	return false, ErrUnavailable
}
