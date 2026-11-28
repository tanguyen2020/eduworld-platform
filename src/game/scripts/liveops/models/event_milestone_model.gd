class_name EventMilestoneModel
extends RefCounted
## A points milestone of a live-ops event (OpenAPI `EventMilestone`; F111-AC-11, AC-14, ADR-0049): the points it needs,
## the rewards the server will grant, `reached` / `claimed` as the server says. `claimed` turns true only after the server
## answered the claim (`RewardClaimResult`); `just_reached` is the static "Mới đạt" badge of an `event.progress.updated`
## (never a blocking popup).

var milestone_id: String = ""
var points_required: int = 1
var rewards: Array[RewardModel] = []
var reached: bool = false
var claimed: bool = false
var just_reached: bool = false


static func from_dict(data: Variant) -> EventMilestoneModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "milestone_id")
	if id.is_empty() or id.length() > 64:
		return null
	var m: EventMilestoneModel = EventMilestoneModel.new()
	m.milestone_id = id
	m.points_required = maxi(1, DataRead.int_of(d, "points_required", 1))
	m.rewards = RewardModel.list_from(d.get("rewards"))
	m.reached = DataRead.bool_of(d, "reached")
	m.claimed = DataRead.bool_of(d, "claimed")
	return m


static func list_from(data: Variant) -> Array[EventMilestoneModel]:
	var out: Array[EventMilestoneModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var m: EventMilestoneModel = from_dict(raw)
			if m != null and out.size() < 20:
				out.append(m)
	return out


func can_claim() -> bool:
	return reached and not claimed
