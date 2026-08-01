package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

func TestResolveVerifiedTenantMissingHeaderReturns401(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := resolveVerifiedTenant(req)
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
}

func TestResolveVerifiedTenantMalformedUUIDReturns400(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	_, err := resolveVerifiedTenant(req)
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeValidation, appErr.Code)
}

func TestResolveVerifiedTenantZeroUUIDReturns400(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "00000000-0000-0000-0000-000000000000")
	_, err := resolveVerifiedTenant(req)
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeValidation, appErr.Code)
}

func TestResolveVerifiedTenantValidHeaderReturnsUUID(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	got, err := resolveVerifiedTenant(req)
	require.NoError(t, err)
	assert.Equal(t, tenantID, got)
}

func TestResolveVerifiedTenantTrimsWhitespace(t *testing.T) {
	t.Parallel()
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "  "+tenantID.String()+"  ")
	got, err := resolveVerifiedTenant(req)
	require.NoError(t, err)
	assert.Equal(t, tenantID, got)
}
