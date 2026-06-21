CREATE TABLE core.tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    country_code CHAR(2) NOT NULL DEFAULT 'RU',
    default_locale VARCHAR(10) NOT NULL DEFAULT 'ru-RU',
    default_currency CHAR(3) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_tenants_status ON core.tenants(status);

CREATE TABLE core.companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    legal_name VARCHAR(500) NOT NULL,
    short_name VARCHAR(255),
    legal_name_en VARCHAR(500),
    legal_name_zh VARCHAR(500),
    company_type VARCHAR(50) NOT NULL,
    tax_id VARCHAR(50),
    registration_number VARCHAR(100),
    country_code CHAR(2) NOT NULL DEFAULT 'RU',
    preferred_locale VARCHAR(10) NOT NULL DEFAULT 'ru-RU',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT chk_company_type CHECK (
        company_type IN (
            'SHIPPER','CONSIGNEE','CARRIER','FORWARDER','LSP','WAREHOUSE',
            'TERMINAL','SUPPLIER','GOVERNMENT_AUTHORITY','EDO_OPERATOR',
            'EPD_OPERATOR','PLATFORM_OPERATOR'
        )
    )
);

CREATE INDEX idx_companies_tenant_id ON core.companies(tenant_id);
CREATE INDEX idx_companies_type ON core.companies(company_type);
CREATE INDEX idx_companies_tax_id ON core.companies(tax_id);
CREATE INDEX idx_companies_status ON core.companies(status);

CREATE TABLE core.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    email VARCHAR(320) NOT NULL,
    phone VARCHAR(50),
    password_hash TEXT,
    full_name VARCHAR(255) NOT NULL,
    preferred_locale VARCHAR(10) NOT NULL DEFAULT 'ru-RU',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    mfa_enabled BOOLEAN NOT NULL DEFAULT false,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_users_tenant_email UNIQUE (tenant_id, email),
    CONSTRAINT chk_user_status CHECK (
        status IN ('ACTIVE','INVITED','BLOCKED','DISABLED','DELETED')
    )
);

CREATE INDEX idx_users_tenant_id ON core.users(tenant_id);
CREATE INDEX idx_users_email ON core.users(email);
CREATE INDEX idx_users_status ON core.users(status);

CREATE TABLE core.company_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    company_id UUID NOT NULL REFERENCES core.companies(id),
    user_id UUID NOT NULL REFERENCES core.users(id),
    position VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1,
    CONSTRAINT uq_company_membership UNIQUE (company_id, user_id)
);

CREATE INDEX idx_company_memberships_company_id ON core.company_memberships(company_id);
CREATE INDEX idx_company_memberships_user_id ON core.company_memberships(user_id);
CREATE INDEX idx_company_memberships_tenant_id ON core.company_memberships(tenant_id);

CREATE TABLE core.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES core.tenants(id),
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    scope VARCHAR(50) NOT NULL DEFAULT 'TENANT',
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_roles_global_code ON core.roles(code) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX uq_roles_tenant_code ON core.roles(tenant_id, code) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_roles_tenant_id ON core.roles(tenant_id);
CREATE INDEX idx_roles_code ON core.roles(code);

CREATE TABLE core.permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(150) NOT NULL UNIQUE,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_permissions_resource_action ON core.permissions(resource, action);

CREATE TABLE core.user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES core.tenants(id),
    user_id UUID NOT NULL REFERENCES core.users(id),
    company_id UUID REFERENCES core.companies(id),
    role_id UUID NOT NULL REFERENCES core.roles(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_user_company_role UNIQUE (user_id, company_id, role_id)
);

CREATE INDEX idx_user_roles_user_id ON core.user_roles(user_id);
CREATE INDEX idx_user_roles_company_id ON core.user_roles(company_id);
CREATE INDEX idx_user_roles_role_id ON core.user_roles(role_id);

CREATE TABLE core.role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES core.roles(id),
    permission_id UUID NOT NULL REFERENCES core.permissions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by UUID,
    CONSTRAINT uq_role_permission UNIQUE (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id ON core.role_permissions(role_id);
CREATE INDEX idx_role_permissions_permission_id ON core.role_permissions(permission_id);

CREATE TABLE core.locales (
    code VARCHAR(10) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    native_name VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false
);

INSERT INTO core.locales (code, name, native_name, enabled, is_default)
VALUES
    ('ru-RU', 'Russian', 'Русский', true, true),
    ('en-US', 'English', 'English', true, false),
    ('zh-CN', 'Chinese Simplified', '简体中文', true, false)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE core.translation_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace VARCHAR(100) NOT NULL,
    key TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_translation_key UNIQUE (namespace, key)
);

CREATE INDEX idx_translation_keys_namespace ON core.translation_keys(namespace);

CREATE TABLE core.translation_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id UUID NOT NULL REFERENCES core.translation_keys(id),
    locale_code VARCHAR(10) NOT NULL REFERENCES core.locales(code),
    value TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_translation_value UNIQUE (key_id, locale_code)
);

CREATE INDEX idx_translation_values_locale ON core.translation_values(locale_code);

CREATE TABLE core.audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES core.tenants(id),
    actor_user_id UUID,
    actor_company_id UUID,
    action VARCHAR(150) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID,
    before_state JSONB,
    after_state JSONB,
    ip_address INET,
    user_agent TEXT,
    correlation_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_tenant_id ON core.audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_actor_user_id ON core.audit_logs(actor_user_id);
CREATE INDEX idx_audit_logs_resource ON core.audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created_at ON core.audit_logs(created_at);
