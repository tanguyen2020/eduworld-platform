class_name AchievementModel
extends RefCounted
## One achievement and the student's progress (`AchievementProgress`, INVENTORY_ECONOMY §7). A hidden achievement not
## yet unlocked keeps NOTHING of its title, description or progress — even when a response carries them by mistake
## (F069-AC-21): the screen can only show "?" and the vague hint. Rewards are granted by the server on unlock; the
## client shows "Đã nhận" and never offers a claim button (F069-AC-22, FR-RWD-003).

const CATEGORY_HIDDEN: String = "hidden"
const TIER_NONE: String = "none"
## Translation keys of the contract categories and tiers, listed literally for tools/extract_pot.gd.
const CATEGORY_KEYS: PackedStringArray = [
	"ui.achievement.category.first_time", "ui.achievement.category.progress", "ui.achievement.category.learning",
	"ui.achievement.category.exploration", "ui.achievement.category.social", "ui.achievement.category.team",
	"ui.achievement.category.competition", "ui.achievement.category.seasonal", "ui.achievement.category.event",
	"ui.achievement.category.hidden",
]
const TIER_KEYS: PackedStringArray = [
	"ui.achievement.tier.single", "ui.achievement.tier.bronze", "ui.achievement.tier.silver",
	"ui.achievement.tier.gold", "ui.achievement.tier.platinum",
]

var achievement_id: String = ""
var code: String = ""
var category: String = ""
var unlocked: bool = false
var unlocked_at: String = ""
var tier: String = TIER_NONE
## True for a hidden achievement not unlocked yet (`hidden` of the contract delta, or category `hidden`).
var hidden: bool = false
var title: Dictionary = {}
var description: Dictionary = {}
var progress_current: float = 0.0
var progress_target: float = 0.0
var rewards: Array[RewardModel] = []


static func from_dict(d: Variant) -> AchievementModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var a: AchievementModel = AchievementModel.new()
	a.achievement_id = DataRead.str_of(m, "achievement_id")
	if a.achievement_id.is_empty():
		return null
	a.code = DataRead.str_of(m, "code")
	a.category = DataRead.str_of(m, "category")
	a.unlocked = DataRead.bool_of(m, "unlocked")
	a.unlocked_at = DataRead.str_of(m, "unlocked_at")
	a.tier = DataRead.str_of(m, "tier", TIER_NONE)
	a.hidden = not a.unlocked and (DataRead.bool_of(m, "hidden") or a.category == CATEGORY_HIDDEN)
	if a.hidden:
		return a
	a.title = DataRead.text_of(m, "title")
	a.description = DataRead.text_of(m, "description")
	a.progress_current = maxf(0.0, DataRead.num_of(m, "progress_current"))
	a.progress_target = maxf(0.0, DataRead.num_of(m, "progress_target"))
	a.rewards = RewardModel.list_from(m.get("rewards"))
	return a


static func list_from(v: Variant) -> Array[AchievementModel]:
	var out: Array[AchievementModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var a: AchievementModel = from_dict(d)
			if a != null:
				out.append(a)
	return out


func display_title() -> String:
	return "?" if hidden else L10n.pick(title)


func fraction() -> float:
	if unlocked:
		return 1.0
	return clampf(progress_current / progress_target, 0.0, 1.0) if progress_target > 0.0 else 0.0


## "7/10" (locale digits); empty when there is no target or the achievement is hidden.
func progress_text() -> String:
	if hidden or progress_target <= 0.0:
		return ""
	return "%s/%s" % [LocaleFormat.number(floorf(minf(progress_current, progress_target))),
		LocaleFormat.number(progress_target)]


func tier_key() -> String:
	return "ui.achievement.tier." + tier if tier != TIER_NONE and not tier.is_empty() else ""
