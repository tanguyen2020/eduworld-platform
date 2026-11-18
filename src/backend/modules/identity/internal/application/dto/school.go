package dto

import (
	"encoding/json"

	"github.com/google/uuid"
)

// NullableString is a JSON merge-patch field: absent (Set=false), null (Set, Value nil) or a value.
type NullableString struct {
	Set   bool
	Value *string
}

// UnmarshalJSON records that the field was present.
func (n *NullableString) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	n.Value = &s
	return nil
}

// LocalizedName is TenantLocalizedName (vi required, en optional).
type LocalizedName struct {
	Vi string `json:"vi"`
	En string `json:"en,omitempty"`
}

// CreateSchool is AdminSchoolCreate (createAdminTenantSchool).
type CreateSchool struct {
	TenantID     uuid.UUID     `json:"-"`
	Code         string        `json:"code"`
	Name         LocalizedName `json:"name"`
	ProvinceCode string        `json:"province_code"`
	MoetCode     *string       `json:"moet_code,omitempty"`
	SchoolLevel  string        `json:"school_level"`
	GradeMin     int           `json:"grade_min"`
	GradeMax     int           `json:"grade_max"`
	Address      *string       `json:"address,omitempty"`
	Reason       string        `json:"reason,omitempty"`
}

// UpdateSchool is SchoolUpdateRequest (tenant) or AdminSchoolUpdate (System Admin, with Status and
// TenantID of the path). IfMatch is the version of the If-Match header.
type UpdateSchool struct {
	TenantID     uuid.UUID      `json:"-"`
	SchoolID     uuid.UUID      `json:"-"`
	IfMatch      int64          `json:"-"`
	Name         *LocalizedName `json:"name,omitempty"`
	ProvinceCode *string        `json:"province_code,omitempty"`
	MoetCode     NullableString `json:"moet_code"`
	SchoolLevel  *string        `json:"school_level,omitempty"`
	GradeMin     *int           `json:"grade_min,omitempty"`
	GradeMax     *int           `json:"grade_max,omitempty"`
	Address      NullableString `json:"address"`
	Status       *string        `json:"status,omitempty"`
	Reason       string         `json:"reason,omitempty"`
}

// ProvinceRef is the province of a school (code + bilingual name).
type ProvinceRef struct {
	Code string            `json:"code"`
	Name map[string]string `json:"name"`
}

// School is the School schema of the contract.
type School struct {
	SchoolID    uuid.UUID     `json:"school_id"`
	TenantID    uuid.UUID     `json:"tenant_id"`
	Code        string        `json:"code"`
	Name        LocalizedName `json:"name"`
	CountryCode *string       `json:"country_code"`
	Province    *ProvinceRef  `json:"province,omitempty"`
	MoetCode    *string       `json:"moet_code"`
	SchoolLevel string        `json:"school_level"`
	GradeMin    int           `json:"grade_min"`
	GradeMax    int           `json:"grade_max"`
	Address     *string       `json:"address"`
	Status      string        `json:"status"`
	IsVirtual   bool          `json:"is_virtual"`
	Version     int64         `json:"version"`
}
