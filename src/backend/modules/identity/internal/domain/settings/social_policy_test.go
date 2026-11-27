package settings

import "testing"

// Delta F013/F015 for the client F111 (PlayerBootstrap.policies.social, owner chốt 2026-11-28 [Confirmed]).
func TestBootstrapSocialPolicy(t *testing.T) {
	cases := []struct {
		name string
		in   SocialPolicyInput
		want SocialPolicy
	}{
		{"school off", SocialPolicyInput{TenantType: "school", HasSchool: true, GradeLevel: 6},
			SocialPolicy{StudentClubCreation: ClubCreationDisabled}},
		{"school on, grade 6", SocialPolicyInput{TenantType: "school", HasSchool: true, GradeLevel: 6, SchoolClubCreation: true},
			SocialPolicy{StudentClubCreation: ClubCreationRequiresApproval, ClubCreationEligible: true}},
		{"school on, grade 3", SocialPolicyInput{TenantType: "school", HasSchool: true, GradeLevel: 3, SchoolClubCreation: true},
			SocialPolicy{StudentClubCreation: ClubCreationRequiresApproval}},
		{"school on, grade 4", SocialPolicyInput{TenantType: "school", HasSchool: true, GradeLevel: 4, SchoolClubCreation: true},
			SocialPolicy{StudentClubCreation: ClubCreationRequiresApproval, ClubCreationEligible: true}},
		{"unit cross-school", SocialPolicyInput{TenantType: TenantTypeUnit, HasSchool: true, GradeLevel: 9, SchoolClubCreation: true,
			CrossSchoolSocial: true}, SocialPolicy{StudentClubCreation: ClubCreationRequiresApproval, ClubCreationEligible: true,
			CrossSchoolEnabled: true}},
		{"school tenant never cross-school", SocialPolicyInput{TenantType: "school", HasSchool: true, GradeLevel: 5, CrossSchoolSocial: true},
			SocialPolicy{StudentClubCreation: ClubCreationDisabled}},
		{"common tenant", SocialPolicyInput{TenantType: "common", CommonTenant: true, HasSchool: true, GradeLevel: 7,
			SchoolClubCreation: true, CrossSchoolSocial: true}, SocialPolicy{StudentClubCreation: ClubCreationDisabled}},
		{"no school", SocialPolicyInput{TenantType: TenantTypeUnit, GradeLevel: 7, SchoolClubCreation: true},
			SocialPolicy{StudentClubCreation: ClubCreationDisabled}},
	}
	for _, c := range cases {
		if got := BootstrapSocialPolicy(c.in); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}
