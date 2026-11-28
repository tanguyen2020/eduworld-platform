class_name SeasonHistoryItemModel
extends RefCounted
## A past season of "Mùa trước" (OpenAPI `SeasonTrackHistoryItem` of `listSeasonTrackHistory`; F111-AC-16): its name,
## the final tier, the points and the tiers the server granted by itself at the end of the season (unclaimed rewards).

var season_id: String = ""
var season_name: Dictionary = {}
var final_tier: int = 0
var points: int = 0
var auto_granted_tiers: PackedInt32Array = []
var ended_at: String = ""


static func from_dict(data: Variant) -> SeasonHistoryItemModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "season_id")
	if not Uuid.is_valid(id):
		return null
	var h: SeasonHistoryItemModel = SeasonHistoryItemModel.new()
	h.season_id = id
	h.season_name = DataRead.text_of(d, "season_name")
	h.final_tier = maxi(0, DataRead.int_of(d, "final_tier"))
	h.points = maxi(0, DataRead.int_of(d, "points"))
	for v: Variant in DataRead.arr_of(d, "auto_granted_tiers"):
		if (v is int or v is float) and int(v) >= 1 and h.auto_granted_tiers.size() < 100:
			h.auto_granted_tiers.append(int(v))
	h.ended_at = DataRead.str_of(d, "ended_at").left(32)
	return h


static func list_from(data: Variant) -> Array[SeasonHistoryItemModel]:
	var out: Array[SeasonHistoryItemModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var h: SeasonHistoryItemModel = from_dict(raw)
			if h != null:
				out.append(h)
	return out


func display_name(locale: String = "") -> String:
	return L10n.pick(season_name, locale)
