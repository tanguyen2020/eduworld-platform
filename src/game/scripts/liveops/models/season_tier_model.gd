class_name SeasonTierModel
extends RefCounted
## One tier of the free season track (OpenAPI `SeasonTrackTier`; F111-AC-16, AC-17, ADR-0049): the points it needs, its
## rewards, `reached` / `claimed` as the server says and whether it is a highlighted milestone tier (10/20/30). There is
## no premium track, no price and no "buy a tier" anywhere (FR-GAM-001).

var tier: int = 1
var points_required: int = 0
var rewards: Array[RewardModel] = []
var reached: bool = false
var claimed: bool = false
var milestone: bool = false


static func from_dict(data: Variant) -> SeasonTierModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var n: int = DataRead.int_of(d, "tier")
	if n < 1:
		return null
	var t: SeasonTierModel = SeasonTierModel.new()
	t.tier = n
	t.points_required = maxi(0, DataRead.int_of(d, "points_required"))
	t.rewards = RewardModel.list_from(d.get("rewards"))
	t.reached = DataRead.bool_of(d, "reached")
	t.claimed = DataRead.bool_of(d, "claimed")
	t.milestone = DataRead.bool_of(d, "milestone")
	return t


func can_claim() -> bool:
	return reached and not claimed
