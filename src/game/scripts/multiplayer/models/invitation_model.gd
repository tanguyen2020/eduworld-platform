class_name InvitationModel
extends RefCounted
## A party invitation sent to the student (F102 data-model §1): the REST `PartyInvitation` of `listMyInvitations`
## (`inviter` = PlayerCard, ISO `expires_at`) or the payload of the realtime `party.invitation.received` (`inviter` =
## PublicPlayer, epoch-ms `expires_at`, `member_count`). The inviter is shown by nickname and avatar only (AGENTS R16).
## The countdown always follows the server `expires_at` — the client never decides when an invitation ends (R5).

const STATUS_PENDING: String = "pending"
const STATUSES: PackedStringArray = ["pending", "accepted", "rejected", "expired", "cancelled"]
const NAME_MAX_LENGTH: int = 128
const NICKNAME_MAX_LENGTH: int = 32

var invitation_id: String = ""
var party_id: String = ""
var party_name: String = ""
var party_type: String = ""
var inviter_id: String = ""
var inviter_nickname: String = ""
var inviter_avatar: Dictionary = {}
var inviter_level: int = 0
## 0 = unknown (the REST invitation has no count).
var member_count: int = 0
var status: String = STATUS_PENDING
var expires_at_ms: int = 0


## REST `PartyInvitation` or realtime payload; null without a valid id, party or inviter nickname.
static func from_dict(data: Variant) -> InvitationModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "invitation_id")
	var party: String = DataRead.str_of(d, "party_id")
	var inviter: Dictionary = DataRead.dict_of(d, "inviter")
	var nick: String = DataRead.str_of(inviter, "nickname")
	if not Uuid.is_valid(id) or not Uuid.is_valid(party) or nick.is_empty() or nick.length() > NICKNAME_MAX_LENGTH:
		return null
	var m: InvitationModel = InvitationModel.new()
	m.invitation_id = id
	m.party_id = party
	m.party_name = DataRead.str_of(d, "party_name").left(NAME_MAX_LENGTH)
	m.party_type = DataRead.str_of(d, "party_type").left(64)
	m.inviter_id = DataRead.str_of(inviter, "user_id")
	m.inviter_nickname = nick
	m.inviter_avatar = (inviter["avatar_config"] as Dictionary).duplicate(true) if inviter.get("avatar_config") is Dictionary else {}
	m.inviter_level = maxi(0, DataRead.int_of(inviter, "level", 0))
	m.member_count = maxi(0, DataRead.int_of(d, "member_count", 0))
	var s: String = DataRead.str_of(d, "status", STATUS_PENDING)
	m.status = s if s in STATUSES else STATUS_PENDING
	m.expires_at_ms = PartyMemberModel.epoch_ms(d.get("expires_at"))
	return m


static func list_from(data: Variant) -> Array[InvitationModel]:
	var out: Array[InvitationModel] = []
	if not data is Array:
		return out
	for raw: Variant in data as Array:
		var m: InvitationModel = from_dict(raw)
		if m != null:
			out.append(m)
	return out


func is_pending() -> bool:
	return status == STATUS_PENDING


## Whole seconds left until the server `expires_at` at `now_ms` (server clock); 0 once expired or unknown.
func seconds_left(now_ms: int) -> int:
	if expires_at_ms <= 0:
		return 0
	return maxi(0, int(ceil(float(expires_at_ms - now_ms) / 1000.0)))


func is_expired(now_ms: int) -> bool:
	return not is_pending() or (expires_at_ms > 0 and now_ms >= expires_at_ms)


func inviter_avatar_model() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(inviter_avatar) if not inviter_avatar.is_empty() else null
