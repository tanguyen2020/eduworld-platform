class_name ProgressionModel
extends RefCounted
## Level, title and XP of the signed-in student as the server computes them (`ProgressionStatus`, AsyncAPI
## `progression.xp.updated`; PROGRESSION_SYSTEM §4.6). The client holds no level table, XP curve or title list
## (FR-GAM-007, F069-AC-01): every number here is copied from a response or a message — never added up locally
## (data-model §3: a message replaces `xp_total` / `xp_in_level`, its `delta` is only shown as the flying number).

var level: int = 0
## `{code, label: {vi, en}}` of the current level title.
var title_code: String = ""
var title_label: Dictionary = {}
var xp_total: int = 0
var xp_in_level: int = 0
## -1 = maximum level reached (`xp_to_next = null`).
var xp_to_next: int = -1
var daily_xp_remaining: int = -1
var curve_version: int = 0
var max_level: int = 0
var next_level_rewards: Array[RewardModel] = []
## From the latest `progression.xp.updated`: today's XP cap is reached (moon icon, F069-AC-02).
var capped: bool = false


static func from_dict(d: Variant) -> ProgressionModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	if DataRead.int_of(m, "level") < 1:
		return null
	var p: ProgressionModel = ProgressionModel.new()
	p.level = DataRead.int_of(m, "level")
	var title: Dictionary = DataRead.dict_of(m, "title")
	p.title_code = DataRead.str_of(title, "code")
	p.title_label = DataRead.text_of(title, "label")
	p.xp_total = maxi(0, DataRead.int_of(m, "xp_total"))
	p.xp_in_level = maxi(0, DataRead.int_of(m, "xp_in_level"))
	p.xp_to_next = DataRead.nullable_int(m, "xp_to_next")
	p.daily_xp_remaining = DataRead.nullable_int(m, "daily_xp_remaining")
	p.curve_version = DataRead.int_of(m, "curve_version")
	p.max_level = DataRead.int_of(m, "max_level")
	p.next_level_rewards = RewardModel.list_from(m.get("next_level_rewards"))
	p.capped = p.daily_xp_remaining == 0
	return p


## Applies a `progression.xp.updated` payload (values replaced, never summed). Returns true when the level span
## changed (`xp_in_level + xp_to_next` differs: a level-up or a republished curve) so the owner reloads
## `getProgression` for the level and title (F069 edge case "curve được publish lại").
func apply_xp_update(payload: Dictionary) -> bool:
	var old_span: int = span()
	xp_total = maxi(0, DataRead.int_of(payload, "xp_total", xp_total))
	xp_in_level = maxi(0, DataRead.int_of(payload, "xp_in_level", xp_in_level))
	if payload.has("xp_to_next"):
		xp_to_next = DataRead.nullable_int(payload, "xp_to_next")
	if payload.has("daily_xp_remaining"):
		daily_xp_remaining = DataRead.nullable_int(payload, "daily_xp_remaining")
	capped = DataRead.bool_of(payload, "capped", daily_xp_remaining == 0)
	return span() != old_span


func is_max_level() -> bool:
	return xp_to_next < 0


## XP of the whole current level (bar maximum); 0 at the maximum level.
func span() -> int:
	return xp_in_level + xp_to_next if xp_to_next >= 0 else 0


## Bar fill 0…1 (full at the maximum level).
func fraction() -> float:
	if is_max_level():
		return 1.0
	var total: int = span()
	return clampf(float(xp_in_level) / float(total), 0.0, 1.0) if total > 0 else 0.0


func title_text() -> String:
	return L10n.pick(title_label)
