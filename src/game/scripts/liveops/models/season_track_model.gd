class_name SeasonTrackModel
extends RefCounted
## The free season track of the current season (OpenAPI `SeasonTrack` of `getSeasonTrack`, AsyncAPI
## `season.track.updated`; F111-AC-16, AC-17, EVENT_SEASON_SYSTEM §7): season name, points, tier / max tier, the points
## left today (neutral words) and the tiers. Points and tier are the server's: an update replaces them; a tier the server
## says reached is marked reached (never computed from the points by the client).

const MAX_TIERS: int = 100

var season_id: String = ""
var season_name: Dictionary = {}
var starts_at: String = ""
var ends_at: String = ""
var points: int = 0
var tier: int = 0
var max_tier: int = 1
var points_per_tier: int = 1
var daily_points_remaining: int = -1
var tiers: Array[SeasonTierModel] = []


static func from_dict(data: Variant) -> SeasonTrackModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "season_id")
	if not Uuid.is_valid(id):
		return null
	var t: SeasonTrackModel = SeasonTrackModel.new()
	t.season_id = id
	t.season_name = DataRead.text_of(d, "season_name")
	t.starts_at = DataRead.str_of(d, "starts_at").left(32)
	t.ends_at = DataRead.str_of(d, "ends_at").left(32)
	t.points = maxi(0, DataRead.int_of(d, "points"))
	t.tier = maxi(0, DataRead.int_of(d, "tier"))
	t.max_tier = maxi(1, DataRead.int_of(d, "max_tier", 1))
	t.points_per_tier = maxi(1, DataRead.int_of(d, "points_per_tier", 1))
	if d.has("daily_points_remaining"):
		t.daily_points_remaining = maxi(0, DataRead.int_of(d, "daily_points_remaining"))
	for raw: Variant in DataRead.arr_of(d, "tiers"):
		var item: SeasonTierModel = SeasonTierModel.from_dict(raw)
		if item != null and t.tiers.size() < MAX_TIERS:
			t.tiers.append(item)
	t.tiers.sort_custom(func(a: SeasonTierModel, b: SeasonTierModel) -> bool: return a.tier < b.tier)
	return t


## `season.track.updated`: points, tier and the daily points left of the server; tiers up to `tier` read reached.
func apply_update(payload: Dictionary) -> void:
	points = maxi(0, DataRead.int_of(payload, "points", points))
	tier = maxi(0, DataRead.int_of(payload, "tier", tier))
	if payload.has("daily_points_remaining"):
		daily_points_remaining = maxi(0, DataRead.int_of(payload, "daily_points_remaining"))
	for t: SeasonTierModel in tiers:
		if t.tier <= tier:
			t.reached = true


func tier_model(n: int) -> SeasonTierModel:
	for t: SeasonTierModel in tiers:
		if t.tier == n:
			return t
	return null


func display_name(locale: String = "") -> String:
	return L10n.pick(season_name, locale)


func claimable_count() -> int:
	var n: int = 0
	for t: SeasonTierModel in tiers:
		if t.can_claim():
			n += 1
	return n
