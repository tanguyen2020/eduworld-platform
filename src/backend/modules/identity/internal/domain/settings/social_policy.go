package settings

// Values of the school policy student_club_creation (SOCIAL_SYSTEM §6.1, PlayerBootstrap.policies.social).
const (
	ClubCreationDisabled         = "disabled"
	ClubCreationRequiresApproval = "requires_approval"
	ClubCreationAuto             = "auto"
)

// Grades of the students who may create a club when the school allows it (SOCIAL_SYSTEM §6.1: grades 4–9; grades 1–3
// only join the clubs of the school).
const (
	ClubCreationMinGrade = 4
	ClubCreationMaxGrade = 9
)

// SocialPolicy is PlayerBootstrap.policies.social (delta F013/F015 for the client F111): display only — module chat
// checks createClub (F120-AC-17) and the friend scope on every command.
type SocialPolicy struct {
	StudentClubCreation  string
	ClubCreationEligible bool
	CrossSchoolEnabled   bool
}

// SocialPolicyInput are the facts of the bootstrap the policy is computed from (all owned by module identity).
type SocialPolicyInput struct {
	TenantType   string
	CommonTenant bool
	HasSchool    bool
	GradeLevel   int
	// SchoolClubCreation is the boolean school setting student_club_creation (true = requires_approval, the rule of
	// module chat F120 — Clarifications [Confirmed]).
	SchoolClubCreation bool
	// CrossSchoolSocial is the tenant setting cross_school_social_enabled (ADR-0054).
	CrossSchoolSocial bool
}

// BootstrapSocialPolicy computes the social policy of a student: the school setting maps true → requires_approval and
// false/absent → disabled; the common tenant has no school policy (disabled); a student may create a club only with a
// school, in grades 4–9 and a policy other than disabled; cross-school social only exists in a unit tenant.
func BootstrapSocialPolicy(in SocialPolicyInput) SocialPolicy {
	out := SocialPolicy{StudentClubCreation: ClubCreationDisabled}
	if in.CommonTenant || !in.HasSchool {
		return out
	}
	if in.SchoolClubCreation {
		out.StudentClubCreation = ClubCreationRequiresApproval
	}
	out.ClubCreationEligible = out.StudentClubCreation != ClubCreationDisabled &&
		in.GradeLevel >= ClubCreationMinGrade && in.GradeLevel <= ClubCreationMaxGrade
	out.CrossSchoolEnabled = in.TenantType == TenantTypeUnit && in.CrossSchoolSocial
	return out
}
