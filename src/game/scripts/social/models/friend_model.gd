class_name FriendModel
extends RefCounted
## Another player as the friends screen may show them (OpenAPI `PlayerCard` of `listFriends` / a friend request;
## realtime `presence.friend.updated`; F110 data-model §1, AGENTS R16, ADR-0095): id, nickname, avatar config, level,
## the level title (`PlayerCard.title`, hidden when absent) and the presence of the shared enum (MULTIPLAYER_GAMEPLAY §5).
## There is deliberately no field for a real name, a school, a grade or any learning result — unknown keys are never
## copied. A friend without presence data is `offline` (F110-AC-20).

const ONLINE: String = "online"
const IN_ACTIVITY: String = "in_activity"
const IN_MATCH: String = "in_match"
const AWAY: String = "away"
const OFFLINE: String = "offline"
const PRESENCES: PackedStringArray = [ONLINE, IN_ACTIVITY, IN_MATCH, AWAY, OFFLINE]
## Presence → [icon, words] (never colour alone, F110-AC-28).
const PRESENCE_VIEW: Dictionary = {
	ONLINE: ["presence_online", "ui.friends.presence.online"],
	IN_ACTIVITY: ["play", "ui.friends.presence.in_activity"],
	IN_MATCH: ["arena", "ui.friends.presence.in_match"],
	AWAY: ["moon", "ui.friends.presence.away"],
	OFFLINE: ["wifi_off", "ui.friends.presence.offline"],
}
const NICKNAME_MAX_LENGTH: int = 32

var user_id: String = ""
var nickname: String = ""
var avatar_config: Dictionary = {}
var level: int = 0
var presence: String = OFFLINE
## `LocalizedName` of the level title; empty = hidden.
var title: Dictionary = {}
var activity: String = ""
var area_id: String = ""


static func from_card(data: Variant) -> FriendModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "user_id")
	var nick: String = DataRead.str_of(d, "nickname")
	if not Uuid.is_valid(id) or nick.is_empty() or nick.length() > NICKNAME_MAX_LENGTH:
		return null
	var f: FriendModel = FriendModel.new()
	f.user_id = id
	f.nickname = nick
	f.avatar_config = DataRead.dict_of(d, "avatar_config").duplicate(true)
	f.level = maxi(0, DataRead.int_of(d, "level"))
	f.set_presence(DataRead.str_of(d, "presence", OFFLINE))
	f.title = DataRead.text_of(d, "title")
	return f


static func list_from(data: Variant) -> Array[FriendModel]:
	var out: Array[FriendModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var f: FriendModel = from_card(raw)
			if f != null:
				out.append(f)
	return out


## Any value outside the shared enum is `offline`.
func set_presence(value: String) -> void:
	presence = value if value in PRESENCES else OFFLINE


func is_online() -> bool:
	return presence != OFFLINE


func presence_icon() -> String:
	return String((PRESENCE_VIEW[presence] as Array)[0])


func presence_key() -> String:
	return String((PRESENCE_VIEW[presence] as Array)[1])


func avatar() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(avatar_config) if not avatar_config.is_empty() else null


func display_title(locale: String = "") -> String:
	return L10n.pick(title, locale) if not title.is_empty() else ""
