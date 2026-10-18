package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	identityhttp "github.com/freight-platform/identity-service/internal/http"
	"github.com/freight-platform/identity-service/internal/http/authority"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/security"
	"github.com/freight-platform/identity-service/internal/service"
)

const authoritySecret = "identity-authority-test-secret"

var (
	tenantA = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userA   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	userB   = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	adminA  = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	roleA   = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	roleB   = uuid.MustParse("77777777-7777-7777-7777-777777777777")
)

func TestUserAndRoleRoutesRequireVerifiedAuthority(t *testing.T) {
	h := newAuthorityHarness()

	t.Run("IUSR_TEST_01 anonymous user list denied", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/users", "", nil)
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("IUSR_TEST_02 anonymous user create denied", func(t *testing.T) {
		rec := h.do(http.MethodPost, "/v1/users", "", []byte(`{"email":"a@b.c","password":"secret","full_name":"A","preferred_locale":"en-US"}`))
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
		if h.users.created != 0 {
			t.Fatal("anonymous create reached the store")
		}
	})
	t.Run("IUSR_TEST_03 anonymous user read denied", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/users/"+userB.String(), "", nil)
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("IUSR_TEST_04 anonymous user update denied", func(t *testing.T) {
		rec := h.do(http.MethodPatch, "/v1/users/"+userB.String(), "", []byte(`{"full_name":"Changed"}`))
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("IUSR_TEST_05 anonymous user delete denied", func(t *testing.T) {
		rec := h.do(http.MethodDelete, "/v1/users/"+userB.String(), "", nil)
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("IUSR_TEST_06 anonymous role list denied", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/roles", "", nil)
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("IUSR_TEST_07 anonymous role permission read denied", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/roles/"+roleB.String()+"/permissions", "", nil)
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("IUSR_TEST_08 tenant A cannot list tenant B users", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/users?tenant_id="+tenantB.String(), h.token(userA, tenantA), nil)
		assertCode(t, rec, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
		if h.users.listedTenant == tenantB {
			t.Fatal("list used the foreign tenant")
		}
	})
	t.Run("IUSR_TEST_09 tenant A cannot create a tenant B user", func(t *testing.T) {
		body := []byte(`{"tenant_id":"` + tenantB.String() + `","email":"new@example.com","password":"secret","full_name":"New","preferred_locale":"en-US"}`)
		rec := h.do(http.MethodPost, "/v1/users", h.token(userA, tenantA), body)
		assertCode(t, rec, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
		if h.users.created != 0 {
			t.Fatal("foreign create reached the store")
		}
	})
	t.Run("IUSR_TEST_10 tenant A cannot read update or delete tenant B user", func(t *testing.T) {
		token := h.token(userA, tenantA)
		read := h.do(http.MethodGet, "/v1/users/"+userB.String(), token, nil)
		assertCode(t, read, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
		update := h.do(http.MethodPatch, "/v1/users/"+userB.String(), token, []byte(`{"full_name":"Changed"}`))
		assertCode(t, update, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
		del := h.do(http.MethodDelete, "/v1/users/"+userB.String(), token, nil)
		assertCode(t, del, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
		if h.users.updated || h.users.deleted {
			t.Fatal("foreign mutation reached the store")
		}
		missing := h.do(http.MethodGet, "/v1/users/"+uuid.NewString(), token, nil)
		assertCode(t, missing, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
		adminRead := h.do(http.MethodGet, "/v1/users/"+userB.String(), h.token(adminA, tenantA), nil)
		assertCode(t, adminRead, http.StatusNotFound, string(apperrors.CodeResourceNotFound))
		adminMissing := h.do(http.MethodGet, "/v1/users/"+uuid.NewString(), h.token(adminA, tenantA), nil)
		assertCode(t, adminMissing, http.StatusNotFound, string(apperrors.CodeResourceNotFound))
	})
	t.Run("IUSR_TEST_11 tenant A cannot read tenant B role or permissions", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/roles/"+roleB.String()+"/permissions", h.token(userA, tenantA), nil)
		assertCode(t, rec, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
		admin := h.do(http.MethodGet, "/v1/roles/"+roleB.String()+"/permissions", h.token(adminA, tenantA), nil)
		assertCode(t, admin, http.StatusNotFound, string(apperrors.CodeResourceNotFound))
		list := h.do(http.MethodGet, "/v1/roles?tenant_id="+tenantB.String(), h.token(userA, tenantA), nil)
		assertCode(t, list, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
	})
	t.Run("IUSR_TEST_12 tenant A cannot assign tenant B role", func(t *testing.T) {
		body := []byte(`{"tenant_id":"` + tenantB.String() + `","role_id":"` + roleB.String() + `"}`)
		rec := h.do(http.MethodPost, "/v1/users/"+userB.String()+"/roles", h.token(userA, tenantA), body)
		assertCode(t, rec, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
		sameTenantBody := []byte(`{"role_id":"` + roleB.String() + `"}`)
		admin := h.do(http.MethodPost, "/v1/users/"+userA.String()+"/roles", h.token(adminA, tenantA), sameTenantBody)
		assertCode(t, admin, http.StatusNotFound, string(apperrors.CodeResourceNotFound))
		if h.roles.assigned {
			t.Fatal("foreign role assignment reached the store")
		}
	})
	t.Run("IUSR_TEST_13 query tenant cannot replace JWT tenant", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/users/"+userA.String()+"?tenant_id="+tenantB.String(), h.token(userA, tenantA), nil)
		assertCode(t, rec, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
	})
	t.Run("IUSR_TEST_14 body tenant cannot replace JWT tenant", func(t *testing.T) {
		body := []byte(`{"tenant_id":"` + tenantB.String() + `","email":"other@example.com","password":"secret","full_name":"Other","preferred_locale":"en-US"}`)
		rec := h.do(http.MethodPost, "/v1/users", h.token(userA, tenantA), body)
		assertCode(t, rec, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
		if h.users.created != 0 {
			t.Fatal("body tenant replaced the verified tenant")
		}
	})
	t.Run("IUSR_TEST_15 missing verified tenant fails closed", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/users", h.tokenWithoutTenant(userA), nil)
		assertCode(t, rec, http.StatusUnauthorized, string(apperrors.CodeTenantContextRequired))
		if h.users.listedTenant != uuid.Nil {
			t.Fatal("missing tenant fell through to a list")
		}
	})
	t.Run("IUSR_TEST_16 verified platform admin override follows explicit policy", func(t *testing.T) {
		ok := h.do(http.MethodGet, "/v1/users?tenant_id="+tenantB.String(), h.token(adminA, tenantA), nil)
		assertCode(t, ok, http.StatusOK, "")
		if h.users.listedTenant != tenantB {
			t.Fatalf("override list tenant=%s", h.users.listedTenant)
		}
		if len(h.audits.events) != 1 || h.audits.events[0].TargetTenantID != tenantB || h.audits.events[0].ActorUserID != adminA {
			t.Fatalf("override audit=%+v", h.audits.events)
		}
		unknown := uuid.MustParse("88888888-8888-8888-8888-888888888888")
		missing := h.do(http.MethodGet, "/v1/users?tenant_id="+unknown.String(), h.token(adminA, tenantA), nil)
		assertCode(t, missing, http.StatusNotFound, string(apperrors.CodeResourceNotFound))
		req := httptest.NewRequest(http.MethodGet, "/v1/users?tenant_id="+tenantB.String(), nil)
		req.Header.Set("Authorization", "Bearer "+h.token(userA, tenantA))
		req.Header.Set("X-Platform-Admin", "true")
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		assertCode(t, rec, http.StatusConflict, string(apperrors.CodeTenantContextConflict))
	})
	t.Run("IUSR_TEST_17 spoofed identity headers do not grant authority", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+h.token(userA, tenantA))
		req.Header.Set("X-Tenant-ID", tenantB.String())
		req.Header.Set("X-User-ID", adminA.String())
		req.Header.Set("X-User-Roles", authority.PlatformAdminRole)
		req.Header.Set("X-Platform-Admin", "true")
		req.Header.Set("X-Internal-Service-Token", "shared-token")
		req.Header.Set("X-Internal-Service-Name", "api-gateway")
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		assertCode(t, rec, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
		headerOnly := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader([]byte(`{"email":"a@b.c","password":"secret","full_name":"A","preferred_locale":"en-US"}`)))
		headerOnly.Header.Set("X-Platform-Admin", "true")
		headerOnly.Header.Set("X-Tenant-ID", tenantA.String())
		headerOnly.Header.Set("X-User-ID", adminA.String())
		anon := httptest.NewRecorder()
		h.router.ServeHTTP(anon, headerOnly)
		assertCode(t, anon, http.StatusUnauthorized, string(apperrors.CodeAuthenticationRequired))
	})
	t.Run("self company membership read uses verified tenant", func(t *testing.T) {
		rec := h.do(http.MethodGet, "/v1/users/"+userA.String()+"/companies?tenant_id="+tenantA.String(), h.token(userA, tenantA), nil)
		assertCode(t, rec, http.StatusOK, "")
		other := h.do(http.MethodGet, "/v1/users/"+userB.String()+"/companies?tenant_id="+tenantA.String(), h.token(userA, tenantA), nil)
		assertCode(t, other, http.StatusForbidden, string(apperrors.CodeAuthorizationDenied))
	})
}

type authorityHarness struct {
	router http.Handler
	jwt    *security.JWTService
	users  *memUsers
	roles  *memRoles
	audits *recordingAuditor
}

func newAuthorityHarness() *authorityHarness {
	users := &memUsers{items: map[uuid.UUID]domain.User{
		userA:  {ID: userA, TenantID: tenantA, Email: "a@example.com", FullName: "User A", PreferredLocale: "en-US", Status: domain.UserStatusActive},
		userB:  {ID: userB, TenantID: tenantB, Email: "b@example.com", FullName: "User B", PreferredLocale: "en-US", Status: domain.UserStatusActive},
		adminA: {ID: adminA, TenantID: tenantA, Email: "admin@example.com", FullName: "Admin A", PreferredLocale: "en-US", Status: domain.UserStatusActive},
	}}
	roles := &memRoles{items: map[uuid.UUID]domain.Role{
		roleA: {ID: roleA, TenantID: &tenantA, Code: "SHIPPER_ADMIN", Name: "Shipper Admin"},
		roleB: {ID: roleB, TenantID: &tenantB, Code: "CARRIER_ADMIN", Name: "Carrier Admin"},
	}}
	lookup := &memLookup{admins: map[uuid.UUID]struct{}{adminA: {}}, tenants: map[uuid.UUID]struct{}{tenantA: {}, tenantB: {}}}
	audits := &recordingAuditor{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtService := security.NewJWTService(authoritySecret, 60)
	userService := service.NewUserService(users)
	roleService := service.NewRoleService(roles, roles, users)
	membershipService := service.NewMembershipService(users, roles, &memMemberships{})
	router := identityhttp.NewRouter(log, nil, jwtService, service.NewAuthService(users, jwtService), userService, roleService, membershipService, nil, nil, "", lookup, audits)
	return &authorityHarness{router: router, jwt: jwtService, users: users, roles: roles, audits: audits}
}

func (h *authorityHarness) token(userID, tenantID uuid.UUID) string {
	signed, _, err := h.jwt.CreateAccessToken(userID, tenantID, "user@example.com", "en-US")
	if err != nil {
		panic(err)
	}
	return signed
}

func (h *authorityHarness) tokenWithoutTenant(userID uuid.UUID) string {
	claims := security.TokenClaims{
		TenantID: "",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(authoritySecret))
	if err != nil {
		panic(err)
	}
	return signed
}

func (h *authorityHarness) do(method, path, token string, body []byte) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func assertCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d want %d body=%s", rec.Code, status, rec.Body.String())
	}
	if code == "" {
		return
	}
	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v body=%s", err, rec.Body.String())
	}
	if payload.Error.Code != code {
		t.Fatalf("code=%s want %s", payload.Error.Code, code)
	}
	for _, key := range []string{"tenant_id", "user_id", "role_id", "company_id"} {
		if _, ok := payload.Error.Details[key]; ok {
			t.Fatalf("error details leaked %s", key)
		}
	}
}

type recordingAuditor struct {
	events []authority.Override
}

func (a *recordingAuditor) RecordPlatformAdminOverride(_ context.Context, event authority.Override) {
	a.events = append(a.events, event)
}

type memLookup struct {
	admins  map[uuid.UUID]struct{}
	tenants map[uuid.UUID]struct{}
}

func (m *memLookup) LookupPrincipal(_ context.Context, userID, _ uuid.UUID) (bool, []string, error) {
	if _, ok := m.admins[userID]; ok {
		return true, []string{authority.PlatformAdminRole}, nil
	}
	return false, nil, nil
}

func (m *memLookup) TenantExists(_ context.Context, tenantID uuid.UUID) (bool, error) {
	_, ok := m.tenants[tenantID]
	return ok, nil
}

type memUsers struct {
	items             map[uuid.UUID]domain.User
	created           int
	lastCreatedTenant uuid.UUID
	listedTenant      uuid.UUID
	updated           bool
	deleted           bool
}

func (m *memUsers) Create(_ context.Context, in domain.CreateUserInput, _ string) (*domain.User, error) {
	m.created++
	m.lastCreatedTenant = in.TenantID
	user := domain.User{ID: uuid.New(), TenantID: in.TenantID, Email: in.Email, FullName: in.FullName, PreferredLocale: in.PreferredLocale, Status: domain.UserStatusActive, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Version: 1}
	m.items[user.ID] = user
	return &user, nil
}
func (m *memUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	user, ok := m.items[id]
	if !ok {
		return nil, apperrors.NotFound("user not found")
	}
	return &user, nil
}
func (m *memUsers) GetByIDAndTenant(_ context.Context, id, tenantID uuid.UUID) (*domain.User, error) {
	user, ok := m.items[id]
	if !ok || user.TenantID != tenantID {
		return nil, apperrors.ResourceNotFound()
	}
	return &user, nil
}
func (m *memUsers) GetByTenantAndEmail(context.Context, uuid.UUID, string) (*domain.User, error) {
	return nil, apperrors.NotFound("user not found")
}
func (m *memUsers) List(_ context.Context, filter domain.ListUsersFilter) ([]domain.User, int, error) {
	m.listedTenant = filter.TenantID
	items := make([]domain.User, 0)
	for _, user := range m.items {
		if user.TenantID == filter.TenantID {
			items = append(items, user)
		}
	}
	return items, len(items), nil
}
func (m *memUsers) Update(context.Context, uuid.UUID, domain.UpdateUserInput) (*domain.User, error) {
	return nil, apperrors.ResourceNotFound()
}
func (m *memUsers) UpdateInTenant(_ context.Context, id, tenantID uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
	user, err := m.GetByIDAndTenant(context.Background(), id, tenantID)
	if err != nil {
		return nil, err
	}
	m.updated = true
	if in.FullName != nil {
		user.FullName = *in.FullName
	}
	m.items[id] = *user
	return user, nil
}
func (m *memUsers) SoftDelete(context.Context, uuid.UUID) error { return apperrors.ResourceNotFound() }
func (m *memUsers) SoftDeleteInTenant(_ context.Context, id, tenantID uuid.UUID) error {
	if _, err := m.GetByIDAndTenant(context.Background(), id, tenantID); err != nil {
		return err
	}
	m.deleted = true
	delete(m.items, id)
	return nil
}
func (m *memUsers) UpdateLastLogin(context.Context, uuid.UUID) error { return nil }

type memRoles struct {
	items    map[uuid.UUID]domain.Role
	assigned bool
}

func (m *memRoles) ListByTenant(_ context.Context, tenantID uuid.UUID) ([]domain.Role, error) {
	items := make([]domain.Role, 0)
	for _, role := range m.items {
		if role.TenantID == nil || *role.TenantID == tenantID {
			items = append(items, role)
		}
	}
	return items, nil
}
func (m *memRoles) GetByID(_ context.Context, roleID uuid.UUID) (*domain.Role, error) {
	role, ok := m.items[roleID]
	if !ok {
		return nil, apperrors.ResourceNotFound()
	}
	return &role, nil
}
func (m *memRoles) GetVisibleToTenant(_ context.Context, roleID, tenantID uuid.UUID) (*domain.Role, error) {
	role, ok := m.items[roleID]
	if !ok || (role.TenantID != nil && *role.TenantID != tenantID) {
		return nil, apperrors.ResourceNotFound()
	}
	return &role, nil
}
func (m *memRoles) RoleAvailableForTenant(_ context.Context, roleID, tenantID uuid.UUID) (bool, error) {
	role, ok := m.items[roleID]
	if !ok {
		return false, nil
	}
	if role.TenantID != nil && *role.TenantID != tenantID {
		return false, nil
	}
	return true, nil
}
func (m *memRoles) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (m *memRoles) AssignRole(context.Context, domain.AssignRoleInput) error {
	m.assigned = true
	return nil
}
func (m *memRoles) ListUserRoles(context.Context, uuid.UUID, uuid.UUID) ([]domain.UserRoleAssignment, error) {
	return []domain.UserRoleAssignment{}, nil
}
func (m *memRoles) ListByRoleID(context.Context, uuid.UUID) ([]domain.Permission, error) {
	return []domain.Permission{{ID: uuid.New(), Code: "users.read", Resource: "users", Action: "read"}}, nil
}

type memMemberships struct{}

func (m *memMemberships) GetUserCompanies(context.Context, domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
	return []domain.UserCompany{}, nil
}
func (m *memMemberships) ActiveMembershipExists(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (m *memMemberships) AddCompanyRoleToUser(context.Context, domain.AssignCompanyRoleInput) error {
	return nil
}
func (m *memMemberships) RemoveCompanyRoleFromUser(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return apperrors.ResourceNotFound()
}
