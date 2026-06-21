package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	Code        string
	Name        string
	Description *string
	Scope       string
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int
}

type UserRoleAssignment struct {
	RoleID    uuid.UUID
	Code      string
	Name      string
	CompanyID *uuid.UUID
}

type AssignRoleInput struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	CompanyID *uuid.UUID
	RoleID    uuid.UUID
}
