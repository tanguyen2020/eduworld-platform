class_name LevelUpModel
extends RefCounted
## A level-up the server announced (`progression.level.reached`: `from_level`, `to_level`, title, rewards already
## granted — STUDENT_GAME_UX_CORE S07, AC-UX-0702). `merge()` folds several pending messages into the one modal
## "Lên {count} cấp!" at the highest level with every reward (F069-AC-03, AC-UX-0704). Display data only.

var from_level: int = 0
var to_level: int = 0
var title_code: String = ""
var title: Dictionary = {}
var rewards: Array[RewardModel] = []
## Levels gained by this (merged) announcement: `to_level - from_level`, at least 1.
var levels_gained: int = 1


static func from_dict(d: Variant) -> LevelUpModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var to: int = DataRead.int_of(m, "to_level")
	if to < 1:
		return null
	var l: LevelUpModel = LevelUpModel.new()
	l.to_level = to
	l.from_level = clampi(DataRead.int_of(m, "from_level", to - 1), 1, to)
	l.title_code = DataRead.str_of(m, "title_code")
	l.title = DataRead.text_of(m, "title")
	l.rewards = RewardModel.list_from(m.get("rewards"))
	l.levels_gained = maxi(1, l.to_level - l.from_level)
	return l


## One announcement for every pending one: highest level, the newest title that has a name, all rewards (capped at
## the contract list size). Null for an empty batch.
static func merge(batch: Array[LevelUpModel]) -> LevelUpModel:
	if batch.is_empty():
		return null
	var out: LevelUpModel = LevelUpModel.new()
	out.from_level = batch[0].from_level
	out.to_level = batch[0].to_level
	for l: LevelUpModel in batch:
		out.from_level = mini(out.from_level, l.from_level)
		if l.to_level >= out.to_level:
			out.to_level = l.to_level
			if not l.title.is_empty():
				out.title = l.title
				out.title_code = l.title_code
		for r: RewardModel in l.rewards:
			out.add_reward(r)
	out.levels_gained = maxi(1, out.to_level - out.from_level)
	return out


func is_multi() -> bool:
	return levels_gained > 1


func title_text() -> String:
	return L10n.pick(title)


## First reward that is a wearable item (target of "Trang bị ngay", F069-AC-04) or "".
func first_item_id() -> String:
	for r: RewardModel in rewards:
		if r.kind == RewardModel.KIND_ITEM and not r.item_id.is_empty():
			return r.item_id
	return ""


## Adds `r` to the merged rewards: XP and Xu of the same currency are summed into one chip ("100 Xu", not "50 Xu"
## twice); items, badges and titles stay one chip each (capped at twice the contract list size).
func add_reward(r: RewardModel) -> void:
	if r.kind in [RewardModel.KIND_XP, RewardModel.KIND_CURRENCY]:
		for existing: RewardModel in rewards:
			if existing.kind == r.kind and existing.currency_code == r.currency_code:
				existing.amount += r.amount
				return
		var copy: RewardModel = RewardModel.new()
		copy.kind = r.kind
		copy.amount = r.amount
		copy.currency_code = r.currency_code
		copy.label = r.label
		rewards.append(copy)
		return
	if rewards.size() < RewardModel.MAX_ITEMS * 2:
		rewards.append(r)
