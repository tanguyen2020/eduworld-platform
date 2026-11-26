class_name PartyMemberModel
extends RefCounted
## One member of the student's party as the server may show it (F102 data-model §1; AGENTS R16, ADR-0095): the
## `PartyMember` of the REST `Party` (`student_id`, ISO `joined_at`, `presence`) or the `PartyMemberState` of the
## realtime party messages (`player` = PublicPlayer, epoch-ms `joined_at`, `status`). Only the nickname, the avatar
## config, the level and the lobby state are kept — unknown keys (a real name, a school, a grade) are never copied, so
## nothing else can reach the UI. A member without a valid id or nickname is dropped (GAME_NETWORKING §9).

const ROLE_LEADER: String = "leader"
const ROLE_MEMBER: String = "member"
## Presence values of REST `PartyMember.presence` and AsyncAPI `PartyMemberState.status` (one enum on screen).
const ONLINE: String = "online"
const AWAY: String = "away"
const IN_ACTIVITY: String = "in_activity"
const DISCONNECTED: String = "disconnected"
const OFFLINE: String = "offline"
const PRESENCES: PackedStringArray = [ONLINE, AWAY, IN_ACTIVITY, DISCONNECTED, OFFLINE]
const NICKNAME_MAX_LENGTH: int = 32

var student_id: String = ""
var nickname: String = ""
var avatar_config: Dictionary = {}
## 0 = unknown (the REST `PartyMember` has no level; the realtime `PublicPlayer` has one).
var level: int = 0
var role: String = ROLE_MEMBER
var ready: bool = false
var presence: String = ONLINE
var joined_at_ms: int = 0


## REST `PartyMember`; null without a valid `student_id` / `nickname`.
static func from_rest(data: Variant) -> PartyMemberModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var m: PartyMemberModel = _base(d.get("student_id"), d.get("nickname"))
	if m == null:
		return null
	m.avatar_config = (d["avatar_config"] as Dictionary).duplicate(true) if d.get("avatar_config") is Dictionary else {}
	m.level = maxi(0, WsEnvelope.as_int(d.get("level"), 0))
	m.role = _role(d.get("role"))
	m.ready = DataRead.bool_of(d, "ready")
	m.presence = _presence(d.get("presence"))
	m.joined_at_ms = epoch_ms(d.get("joined_at"))
	return m


## AsyncAPI `PartyMemberState` (`player` is a PublicPlayer); null when the player is not valid.
static func from_ws(data: Variant) -> PartyMemberModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var player: Dictionary = DataRead.dict_of(d, "player")
	var m: PartyMemberModel = _base(player.get("user_id"), player.get("nickname"))
	if m == null:
		return null
	m.avatar_config = (player["avatar_config"] as Dictionary).duplicate(true) if player.get("avatar_config") is Dictionary else {}
	m.level = maxi(0, WsEnvelope.as_int(player.get("level"), 0))
	m.role = _role(d.get("role"))
	m.ready = DataRead.bool_of(d, "ready")
	m.presence = _presence(d.get("status"))
	m.joined_at_ms = epoch_ms(d.get("joined_at"))
	return m


## A friend of `listFriends` (`PlayerCard`: user_id, nickname, level, avatar, presence) for the invite picker.
static func from_card(data: Variant) -> PartyMemberModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var m: PartyMemberModel = _base(d.get("user_id"), d.get("nickname"))
	if m == null:
		return null
	m.avatar_config = (d["avatar_config"] as Dictionary).duplicate(true) if d.get("avatar_config") is Dictionary else {}
	m.level = maxi(0, WsEnvelope.as_int(d.get("level"), 0))
	var p: String = DataRead.str_of(d, "presence", OFFLINE)
	m.presence = p if p in PRESENCES else (IN_ACTIVITY if p == "in_match" else OFFLINE)
	return m


func is_leader() -> bool:
	return role == ROLE_LEADER


## The member lost the connection (grace of the server): the card fades and says so in words.
func is_disconnected() -> bool:
	return presence == DISCONNECTED or presence == OFFLINE


## Avatar config for the 2D thumbnail (AvatarThumb); null when the server sent none.
func avatar() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(avatar_config) if not avatar_config.is_empty() else null


func copy() -> PartyMemberModel:
	var m: PartyMemberModel = PartyMemberModel.new()
	m.student_id = student_id
	m.nickname = nickname
	m.avatar_config = avatar_config.duplicate(true)
	m.level = level
	m.role = role
	m.ready = ready
	m.presence = presence
	m.joined_at_ms = joined_at_ms
	return m


## Epoch ms of a realtime `EpochMs` (number) or a REST RFC 3339 date-time; 0 when unreadable.
static func epoch_ms(value: Variant) -> int:
	if value is int or value is float:
		return maxi(0, int(value))
	if value is String and not (value as String).is_empty():
		var text: String = (value as String).strip_edges().trim_suffix("Z").split(".")[0].split("+")[0]
		var seconds: int = Time.get_unix_time_from_datetime_string(text)
		return maxi(0, seconds * 1000) if seconds > 0 else 0
	return 0


static func _base(id: Variant, nick: Variant) -> PartyMemberModel:
	if not id is String or not Uuid.is_valid(String(id)):
		return null
	if not nick is String or (nick as String).is_empty() or (nick as String).length() > NICKNAME_MAX_LENGTH:
		return null
	var m: PartyMemberModel = PartyMemberModel.new()
	m.student_id = String(id)
	m.nickname = String(nick)
	return m


static func _role(value: Variant) -> String:
	return ROLE_LEADER if value is String and String(value) == ROLE_LEADER else ROLE_MEMBER


static func _presence(value: Variant) -> String:
	return String(value) if value is String and String(value) in PRESENCES else ONLINE
