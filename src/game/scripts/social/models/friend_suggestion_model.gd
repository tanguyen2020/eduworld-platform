class_name FriendSuggestionModel
extends RefCounted
## One "Vừa chơi cùng" suggestion (OpenAPI `FriendSuggestion` of `listFriendSuggestions`, F095 delta; F111 data-model §1,
## AGENTS R16, ADR-0095): the public card of the player (FriendModel: nickname, avatar, level, level title — never a real
## name, a school or a grade), the source (only `recently_played`; there is no "Bạn cùng lớp" source any more) and the
## context of the last game together, shown as an icon AND words (party / match / club / co-op). Unknown keys are never
## copied; an unknown context shows no context chip.

const SOURCE_RECENTLY_PLAYED: String = "recently_played"
## context_kind → [icon, words].
const CONTEXT_VIEW: Dictionary = {
	"party": ["party", "ui.friends.suggest.context.party"],
	"match": ["arena", "ui.friends.suggest.context.match"],
	"club": ["flag", "ui.friends.suggest.context.club"],
	"coop": ["emoji_handshake", "ui.friends.suggest.context.coop"],
}

var player: FriendModel = null
var source: String = SOURCE_RECENTLY_PLAYED
var context_kind: String = ""
var last_played_at: String = ""


static func from_dict(data: Variant) -> FriendSuggestionModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if DataRead.str_of(d, "source") != SOURCE_RECENTLY_PLAYED:
		return null
	var card: FriendModel = FriendModel.from_card(d.get("player"))
	if card == null:
		return null
	var s: FriendSuggestionModel = FriendSuggestionModel.new()
	s.player = card
	var context: String = DataRead.str_of(d, "context_kind")
	s.context_kind = context if CONTEXT_VIEW.has(context) else ""
	s.last_played_at = DataRead.str_of(d, "last_played_at").left(32)
	return s


static func list_from(data: Variant) -> Array[FriendSuggestionModel]:
	var out: Array[FriendSuggestionModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var s: FriendSuggestionModel = from_dict(raw)
			if s != null:
				out.append(s)
	return out


func user_id() -> String:
	return player.user_id


func context_icon() -> String:
	return String((CONTEXT_VIEW[context_kind] as Array)[0]) if CONTEXT_VIEW.has(context_kind) else ""


func context_key() -> String:
	return String((CONTEXT_VIEW[context_kind] as Array)[1]) if CONTEXT_VIEW.has(context_kind) else ""
