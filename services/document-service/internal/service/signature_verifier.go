package service

import (
	"context"
	"io"
	"time"

	"github.com/freight-platform/document-service/internal/domain"
)

type VerificationInput struct {
	AttachmentSHA256 string
	Attachment       io.Reader
	Signature        []byte
	Format           string
	PolicyID         string
	PolicyVersion    string
}

type VerificationResult struct {
	Status          string
	ReasonCode      string
	VerifierVersion string
	PolicyID        string
	PolicyVersion   string
	AttemptedAt     time.Time
}

type SignatureVerifier interface {
	Verify(ctx context.Context, in VerificationInput) (VerificationResult, error)
}

type UnavailableSignatureVerifier struct{}

func (UnavailableSignatureVerifier) Verify(_ context.Context, in VerificationInput) (VerificationResult, error) {
	return VerificationResult{
		Status:          domain.VerificationPending,
		ReasonCode:      domain.ReasonVerifierUnavailable,
		VerifierVersion: domain.VerifierVersionUnavailable,
		PolicyID:        in.PolicyID,
		PolicyVersion:   in.PolicyVersion,
		AttemptedAt:     time.Now().UTC(),
	}, nil
}
