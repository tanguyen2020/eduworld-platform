class_name ClubModel
extends RefCounted
## A club as the game shows it (OpenAPI `Club` of `listClubs` / `getClub` / `createClub`; F111 data-model §1, SOCIAL_SYSTEM
## §6.5): name, description, icon, type, member count of the server, status and the student's own role (which only decides
## what is SHOWN — every action is checked by the server, F120). `school_id` / `owner_id` are never kept (nothing on screen
## says which school a club or a member belongs to, AGENTS R16). A legacy `study_group` (before ADR-0095) or an unknown
## type is dropped. The icon is one id of the client icon library (`ICONS`, presentation only — R2 does not apply to UI
## assets); an unknown id shows the generic flag.

const TYPES: PackedStringArray = ["school_club", "subject_club", "competition_team", "interest_group"]
## Types a student may create (SOCIAL_SYSTEM §6.1, F111-AC-22).
const STUDENT_TYPES: PackedStringArray = ["interest_group", "competition_team"]
const PENDING: String = "pending_approval"
const ACTIVE: String = "active"
const LOCKED: String = "locked"
const ARCHIVED: String = "archived"
const STATUSES: PackedStringArray = [PENDING, ACTIVE, LOCKED, ARCHIVED]
const ROLE_OWNER: String = "owner"
const ROLE_OFFICER: String = "officer"
const ROLE_MEMBER: String = "member"
const ROLES: PackedStringArray = [ROLE_OWNER, ROLE_OFFICER, ROLE_MEMBER]
const NAME_MIN: int = 2
const NAME_MAX: int = 64
const DESCRIPTION_MAX: int = 500
## Client icon library of the club form (`icon_id` → design/icons/game icon).
const ICONS: Dictionary = {
	"club_icon_star": "star", "club_icon_book": "emoji_book", "club_icon_rocket": "emoji_rocket",
	"club_icon_compass": "map", "club_icon_music": "music", "club_icon_trophy": "arena", "club_icon_idea": "emoji_light_bulb",
	"club_icon_heart": "heart",
}
const DEFAULT_ICON: String = "flag"

var club_id: String = ""
var club_type: String = ""
var name: String = ""
var description: String = ""
var icon_id: String = ""
var status: String = ACTIVE
var member_count: int = 0
var max_members: int = 0
## "" = not a member (a club of "Club đề xuất").
var my_role: String = ""
var chat_channel_id: String = ""
var version: int = 0


static func from_dict(data: Variant) -> ClubModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "club_id")
	var kind: String = DataRead.str_of(d, "club_type")
	var club_name: String = DataRead.str_of(d, "name")
	if not Uuid.is_valid(id) or not kind in TYPES or club_name.is_empty() or club_name.length() > NAME_MAX:
		return null
	var c: ClubModel = ClubModel.new()
	c.club_id = id
	c.club_type = kind
	c.name = club_name
	c.description = DataRead.str_of(d, "description").left(DESCRIPTION_MAX)
	c.icon_id = DataRead.str_of(d, "icon_id").left(64)
	var st: String = DataRead.str_of(d, "status", ACTIVE)
	c.status = st if st in STATUSES else LOCKED
	c.member_count = maxi(0, DataRead.int_of(d, "member_count"))
	c.max_members = maxi(0, DataRead.int_of(d, "max_members"))
	var role: String = DataRead.str_of(d, "my_role")
	c.my_role = role if role in ROLES else ""
	var channel: String = DataRead.str_of(d, "chat_channel_id")
	c.chat_channel_id = channel if Uuid.is_valid(channel) else ""
	c.version = maxi(0, DataRead.int_of(d, "version"))
	return c


static func list_from(data: Variant) -> Array[ClubModel]:
	var out: Array[ClubModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var c: ClubModel = from_dict(raw)
			if c != null:
				out.append(c)
	return out


func is_member() -> bool:
	return not my_role.is_empty()


## Owner / officer: the "Yêu cầu" tab and "Mời bạn" are shown (the server still decides, F111-AC-25).
func is_officer() -> bool:
	return my_role == ROLE_OWNER or my_role == ROLE_OFFICER


func is_owner() -> bool:
	return my_role == ROLE_OWNER


func is_active() -> bool:
	return status == ACTIVE


func is_pending() -> bool:
	return status == PENDING


## `locked` / `archived`: view only — no join, invite or chat writing (F111-AC-24).
func is_read_only() -> bool:
	return status == LOCKED or status == ARCHIVED


func is_full() -> bool:
	return max_members > 0 and member_count >= max_members


func type_key() -> String:
	return "ui.club.type.%s" % club_type


func status_key() -> String:
	return "ui.club.status.%s" % status


func icon_name() -> String:
	return icon_for(icon_id)


static func icon_for(id: String) -> String:
	return String(ICONS.get(id, DEFAULT_ICON))
