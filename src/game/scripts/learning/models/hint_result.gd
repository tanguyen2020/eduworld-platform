class_name HintResultModel
extends RefCounted
## Next hint level of an item (`HintResult`, ACTIVITY_ENGINE §8.3): `{level, total_levels, hint}`. The server keeps
## `hints_revealed`/`hints_used`; the client never counts them (F063-AC-16).

const LEVEL_MAX: int = 5
## `HintResult.hint.text` maxLength (specs/openapi).
const TEXT_MAX_LENGTH: int = 2000

var position: int = 0
var level: int = 0
var total_levels: int = 0
var text: String = ""
var math_assets: Array[MathAssetRefModel] = []


## Null when `level`, `total_levels` or `hint.text` is missing.
static func from_dict(data: Dictionary) -> HintResultModel:
	var hint: Variant = data.get("hint")
	var lvl: Variant = data.get("level")
	var total: Variant = data.get("total_levels")
	if not hint is Dictionary or not (lvl is int or lvl is float) or not (total is int or total is float) \
			or not (hint as Dictionary).get("text") is String:
		return null
	var h: HintResultModel = HintResultModel.new()
	var pos: Variant = data.get("position")
	h.position = int(pos) if pos is int or pos is float else 0
	h.total_levels = clampi(int(total), 1, LEVEL_MAX)
	h.level = clampi(int(lvl), 1, h.total_levels)
	h.text = ((hint as Dictionary)["text"] as String).left(TEXT_MAX_LENGTH)
	h.math_assets = MathAssetRefModel.list_from((hint as Dictionary).get("math_assets"))
	return h


## The last level was revealed: the hint button greys out.
func is_last() -> bool:
	return level >= total_levels
