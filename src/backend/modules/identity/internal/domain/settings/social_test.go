package settings

import (
	"encoding/json"
	"reflect"
	"testing"
)

// F015 T044: settings_subset of tenant.updated always carries both social keys, false unless stored true.
func TestTenantSubset(t *testing.T) {
	for in, want := range map[string]map[string]any{
		``:   {KeyCrossSchoolSocial: false, KeyCommonSocial: false},
		`{}`: {KeyCrossSchoolSocial: false, KeyCommonSocial: false},
		`{"cross_school_social_enabled": true, "default_locale": "vi-VN"}`:       {KeyCrossSchoolSocial: true, KeyCommonSocial: false},
		`{"cross_school_social_enabled": null, "common_social_enabled": true}`:   {KeyCrossSchoolSocial: false, KeyCommonSocial: true},
		`{"cross_school_social_enabled": "yes", "common_social_enabled": 1}`:     {KeyCrossSchoolSocial: false, KeyCommonSocial: false},
		`{"cross_school_social_enabled": false, "common_social_enabled": false}`: {KeyCrossSchoolSocial: false, KeyCommonSocial: false},
	} {
		if got := TenantSubset([]byte(in)); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: %v, want %v", in, got, want)
		}
	}
}

func TestValidateCrossSchoolSocial(t *testing.T) {
	ok := []struct{ typ, raw string }{{"unit", "true"}, {"unit", "false"}, {"unit", "null"}, {"school", "false"}, {"common", "null"}}
	for _, c := range ok {
		if err := ValidateCrossSchoolSocial(c.typ, json.RawMessage(c.raw)); err != nil {
			t.Fatalf("%v: %v", c, err)
		}
	}
	bad := []struct{ typ, raw string }{{"school", "true"}, {"common", "true"}, {"unit", `"true"`}, {"unit", "1"}}
	for _, c := range bad {
		if err := ValidateCrossSchoolSocial(c.typ, json.RawMessage(c.raw)); err == nil {
			t.Fatalf("%v accepted", c)
		}
	}
}
