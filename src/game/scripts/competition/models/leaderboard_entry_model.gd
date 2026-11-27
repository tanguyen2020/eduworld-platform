class_name LeaderboardEntryModel
extends RefCounted
## One row of a board (`LeaderboardEntry`; F109-AC-20, FR-LDB-003): the rank and value the server computed, the
## player's nickname + avatar (PlayerCard) and `is_me`. Nothing else of `player` is copied — no real name, school,
## grade or class can reach a row (AGENTS R16, ADR-0095).

const VALUE_NONE: float = -1.0

var rank: int = 0
var player: PublicPlayerModel = null
var value: float = VALUE_NONE
var tier_code: String = ""
var is_me: bool = false


static func from_dict(data: Variant) -> LeaderboardEntryModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var e: LeaderboardEntryModel = LeaderboardEntryModel.new()
	e.rank = DataRead.int_of(d, "rank")
	e.player = PublicPlayerModel.from_dict(d.get("player"))
	if e.rank < 1 or e.player == null:
		return null
	e.value = DataRead.num_of(d, "value", VALUE_NONE) if (d.get("value") is int or d.get("value") is float) else VALUE_NONE
	e.tier_code = DataRead.str_of(d, "tier_code").left(64)
	e.is_me = DataRead.bool_of(d, "is_me")
	return e


static func list_from(raw: Variant) -> Array[LeaderboardEntryModel]:
	var out: Array[LeaderboardEntryModel] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var e: LeaderboardEntryModel = from_dict(item)
			if e != null:
				out.append(e)
	return out
