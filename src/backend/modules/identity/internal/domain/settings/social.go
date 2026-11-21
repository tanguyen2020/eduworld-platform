package settings

import "encoding/json"

// Tenant setting keys projected by the other modules through tenant.updated.settings_subset (F015 T044, A-1 of
// F095): ref.tenants.settings_subset of module chat (and competition) decides the cross-school social scope.
const (
	// KeyCrossSchoolSocial is cross_school_social_enabled (ADR-0054, CONFIG_DEFAULTS default false, only
	// meaningful for a unit tenant; written by the Tenant Admin through updateTenantSettings).
	KeyCrossSchoolSocial = "cross_school_social_enabled"
	// KeyCommonSocial is common_social_enabled (F095 Clarifications, ADR-0078): the System Admin re-enabled
	// friends for a common tenant (written by F120 T059, never through updateTenantSettings); absent = off.
	KeyCommonSocial = "common_social_enabled"
)

// TenantTypeUnit is the only tenant type where cross_school_social_enabled has a meaning.
const TenantTypeUnit = "unit"

// ValidateCrossSchoolSocial checks a cross_school_social_enabled patch value of a tenant of tenantType: a boolean
// or null, and only a unit tenant may turn it on (a school or common tenant has no other school, ADR-0054).
func ValidateCrossSchoolSocial(tenantType string, raw json.RawMessage) error {
	var v *bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return ErrInvalid{KeyCrossSchoolSocial, "boolean or null"}
	}
	if v != nil && *v && tenantType != TenantTypeUnit {
		return ErrInvalid{KeyCrossSchoolSocial, "only a unit tenant"}
	}
	return nil
}

// TenantSubset returns settings_subset of tenant.updated from the stored tenant settings JSON: always both social
// keys, false when absent, null or not a boolean (the safe default, F095 A-1). The consumers replace their
// ref.tenants.settings_subset with it, so it carries every key they read.
func TenantSubset(value []byte) map[string]any {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(value, &m)
	flag := func(k string) bool {
		var b bool
		if raw, ok := m[k]; ok {
			_ = json.Unmarshal(raw, &b)
		}
		return b
	}
	return map[string]any{KeyCrossSchoolSocial: flag(KeyCrossSchoolSocial), KeyCommonSocial: flag(KeyCommonSocial)}
}
