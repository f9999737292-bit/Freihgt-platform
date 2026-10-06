package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

const (
	AttachmentStatusDraft     = "DRAFT"
	AttachmentStatusFinalized = "FINALIZED"

	VerificationPending    = "PENDING"
	VerificationValid      = "VALID"
	VerificationInvalid    = "INVALID"
	VerificationUnverified = "UNVERIFIED"

	MaxAttachmentBytes = 10 << 20
	MaxSignatureBytes  = 1 << 20

	ReasonVerifierUnavailable  = "VERIFIER_UNAVAILABLE"
	VerifierVersionUnavailable = "unavailable-v0"
	PolicyQualifiedCAdES       = "QUALIFIED_CADES_BES"
	PolicyVersionV1            = "v1"
	SignatureProfileCAdESBES   = "CAdES-BES"
	SignatureMediaTypePKCS7    = "application/pkcs7-signature"

	EventAttachmentCreated   = "edo.attachment.created"
	EventAttachmentFinalized = "edo.attachment.finalized"
	EventSignatureAttached   = "edo.signature.attached"
	EventSignatureVerified   = "edo.signature.verified"
	EventVerificationFailed  = "edo.signature.verification_failed"
)

func IsSignedEvidenceClass(status string) bool {
	switch status {
	case DocumentStatusSigned, DocumentStatusSentToOperator, DocumentStatusAccepted, DocumentStatusArchived:
		return true
	default:
		return false
	}
}

func SHA256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func ValidSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) || unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func SafeFileName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.Contains(name, "\x00") || strings.Contains(name, "..") {
		return "", apperrors.Validation("file name is not allowed", map[string]any{"field": "file_name"})
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return "", apperrors.Validation("file name is not allowed", map[string]any{"field": "file_name"})
	}
	base := path.Base(name)
	if base == "." || base == "/" || base == "" || base != name {
		return "", apperrors.Validation("file name is not allowed", map[string]any{"field": "file_name"})
	}
	if len(base) > 255 {
		return "", apperrors.Validation("file name is too long", map[string]any{"field": "file_name"})
	}
	return base, nil
}

func NormalizeMediaType(raw string) (string, error) {
	media := strings.TrimSpace(strings.ToLower(raw))
	if i := strings.Index(media, ";"); i >= 0 {
		media = strings.TrimSpace(media[:i])
	}
	switch media {
	case "application/pdf", "application/xml", "text/xml", "application/json",
		"image/png", "image/jpeg", "application/octet-stream":
		return media, nil
	default:
		return "", apperrors.Validation("media type is not allowed", map[string]any{"field": "media_type"})
	}
}

func MediaMatchesContent(media string, content []byte) error {
	switch media {
	case "application/pdf":
		if !bytes.HasPrefix(content, []byte("%PDF")) {
			return apperrors.Validation("content does not match media type", map[string]any{"field": "media_type"})
		}
	case "image/png":
		if !bytes.HasPrefix(content, []byte{0x89, 'P', 'N', 'G'}) {
			return apperrors.Validation("content does not match media type", map[string]any{"field": "media_type"})
		}
	case "image/jpeg":
		if !bytes.HasPrefix(content, []byte{0xff, 0xd8, 0xff}) {
			return apperrors.Validation("content does not match media type", map[string]any{"field": "media_type"})
		}
	}
	return nil
}

func RejectPrivateKeyMaterial(value string) error {
	upper := strings.ToUpper(value)
	if strings.Contains(upper, "PRIVATE KEY") || strings.Contains(upper, "BEGIN RSA") {
		return apperrors.Validation("private key material is not accepted", map[string]any{"field": "signature_reference"})
	}
	return nil
}

type Attachment struct {
	ID                uuid.UUID
	DocumentID        uuid.UUID
	TenantID          uuid.UUID
	DocumentVersionID *uuid.UUID
	FileName          string
	MediaType         string
	SizeBytes         int64
	SHA256            string
	StorageKey        string
	Status            string
	IdempotencyKey    *string
	CreatedAt         time.Time
	FinalizedAt       *time.Time
}

type AttachmentSignature struct {
	ID                    uuid.UUID
	AttachmentID          uuid.UUID
	TenantID              uuid.UUID
	SignatureFormat       string
	SignatureReference    string
	CertificateSubject    *string
	CertificateIssuer     *string
	CertificateSerial     *string
	CertificateThumbprint *string
	SigningTime           *time.Time
	VerificationStatus    string
	VerificationTime      *time.Time
	VerificationError     *string
	IdempotencyKey        *string
	CreatedAt             time.Time
	EffectiveStatus       string
	ReasonCode            string
}

func OpaqueSignatureReference(id uuid.UUID) string {
	return "bintrans:attachment-signature:" + id.String()
}

func SignatureObjectKey(tenantID, documentID, attachmentID, signatureID uuid.UUID) string {
	// Sibling of the attachment object key. The local store maps a key to a file,
	// so a signature key must not extend the attachment key as a directory.
	return "tenants/" + tenantID.String() + "/documents/" + documentID.String() + "/attachment-signatures/" + attachmentID.String() + "/" + signatureID.String()
}
