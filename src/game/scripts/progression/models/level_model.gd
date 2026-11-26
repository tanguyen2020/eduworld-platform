class_name LevelModel
extends RefCounted
## One milestone of the progression curve in force (`ProgressionLevel` of `listProgressionLevels`): level, XP needed
## for the next level, title code and the rewards of reaching it. Shown read-only in "Hành trình của tôi"; the client
## never derives a level from XP with it (FR-GAM-007).

var level: int = 0
var xp_required: int = 0
var title_code: String = ""
## Optional display name of the title when the server sends one (`title` LocalizedName; contract §6 delta).
var title: Dictionary = {}
var rewards: Array[RewardModel] = []


static func from_dict(d: Variant) -> LevelModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	if DataRead.int_of(m, "level") < 1:
		return null
	var l: LevelModel = LevelModel.new()
	l.level = DataRead.int_of(m, "level")
	l.xp_required = maxi(0, DataRead.int_of(m, "xp_required"))
	l.title_code = DataRead.str_of(m, "title_code")
	l.title = DataRead.text_of(m, "title")
	l.rewards = RewardModel.list_from(m.get("rewards"))
	return l


static func list_from(v: Variant) -> Array[LevelModel]:
	var out: Array[LevelModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var l: LevelModel = from_dict(d)
			if l != null:
				out.append(l)
	return out
