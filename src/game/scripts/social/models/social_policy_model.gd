class_name SocialPolicyModel
extends RefCounted
## The effective social policy of the student's school, cut down for the client (`PlayerBootstrap.policies.social`, F013 /
## F015 delta; F111 data-model §1, SOCIAL_SYSTEM §6.1, §7). Display only: it hides or shows "Tạo club" — `createClub` is
## still checked by the server (`403 PERMISSION_DENIED`, F120-AC-17). Absent or unknown values read as the safest
## default: club creation `disabled`, not eligible, no cross-school social.

const DISABLED: String = "disabled"
const REQUIRES_APPROVAL: String = "requires_approval"
const AUTO: String = "auto"
const CREATION_VALUES: PackedStringArray = [DISABLED, REQUIRES_APPROVAL, AUTO]

var student_club_creation: String = DISABLED
var club_creation_eligible: bool = false
var cross_school_enabled: bool = false


## `policies` of the bootstrap (AppState.school_policies).
static func from_policies(policies: Dictionary) -> SocialPolicyModel:
	var p: SocialPolicyModel = SocialPolicyModel.new()
	var social: Dictionary = DataRead.dict_of(policies, "social")
	var creation: String = DataRead.str_of(social, "student_club_creation", DISABLED)
	p.student_club_creation = creation if creation in CREATION_VALUES else DISABLED
	p.club_creation_eligible = DataRead.bool_of(social, "club_creation_eligible")
	p.cross_school_enabled = DataRead.bool_of(social, "cross_school_enabled")
	return p


## F111-AC-22: "Tạo club" shows only when the school does not disable it AND the student is eligible.
func can_show_create() -> bool:
	return student_club_creation != DISABLED and club_creation_eligible


## The school asked the student's own club to be approved first ("Club sẽ chờ nhà trường duyệt").
func needs_approval() -> bool:
	return student_club_creation == REQUIRES_APPROVAL
