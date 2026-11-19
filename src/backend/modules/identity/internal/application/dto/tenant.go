// Package dto holds the input/output types of module identity's use cases (JSON names of the contract
// specs/openapi/modules/identity.v1.yaml); string lengths are validated again by the OpenAPI validator.
package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateTenant is TenantCreate (createTenant).
type CreateTenant struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	TenantType    string `json:"tenant_type"`
	Alias         string `json:"keycloak_org_alias"`
	DefaultLocale string `json:"default_locale,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// UpdateTenant is TenantUpdate (updateTenant, JSON merge patch) with the If-Match version.
type UpdateTenant struct {
	ID      uuid.UUID `json:"-"`
	IfMatch int64     `json:"-"`
	Name    *string   `json:"name,omitempty"`
	Status  *string   `json:"status,omitempty"`
	Reason  string    `json:"reason"`
}

// Tenant is TenantDetail (TenantSummary + alias and timestamps). Version is the If-Match value of updateTenant
// (OQ F057-3): returned in the body and as the ETag.
type Tenant struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	TenantType  string    `json:"tenant_type"`
	Status      string    `json:"status"`
	SchoolCount *int      `json:"school_count,omitempty"`
	Alias       string    `json:"keycloak_org_alias"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int64     `json:"version,omitempty"` // also the ETag of create/update (If-Match of updateTenant)
}
