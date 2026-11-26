extends VBoxContainer
## Result screen after submit (STUDENT_GAME_UX_LEARNING §10, LEARNING_EXPERIENCE §7, F063-AC-21…24). Two separate
## regions: "Kết quả học tập" (from `getActivityAttemptReview`: right answers / total — `g1_3` as a count with a star,
## never a 10-point scale —, the server score for older bands, objectives to review) and "Phần thưởng trong game"
## (only rewards the server granted; hidden when there are none). Then Mây's recommendations with a practice button
## (`startAdHocAttempt` with `recommendation_id`, through the host). No comparison with other students, no teacher
## comment (ADR-0072, AGENTS R16). When results are released later it only says so (`results_later`). F096-AC-14: a
## CelebrationBanner on top (Mây, "Hoàn thành!", one star icon per right answer for `g1_3`, a single effect ≤ 1.5 s, none
## with reduced motion); rewards are listed with icons, never emoji (F096-AC-15).

signal practice_requested(recommendation: AttemptReviewModel.Recommendation)
signal back_requested()

const BAND_COUNT_ONLY: String = "g1_3"
const REWARD_ICONS: Dictionary = {
	"xp": preload("res://resources/ui/icons/star.svg"), "badge": preload("res://resources/ui/icons/medal.svg"),
}
const REWARD_ICON_ITEM: Texture2D = preload("res://resources/ui/icons/party.svg")

var review: AttemptReviewModel = null
var rewards: Array[Dictionary] = []
var results_later: bool = false

@onready var _score: Label = %Score
@onready var _points: Label = %Points
@onready var _weak: Label = %Weak
@onready var _later: Label = %Later
@onready var _rewards_box: PanelContainer = %Rewards
@onready var _reward_list: VBoxContainer = %RewardList
@onready var _recs_box: VBoxContainer = %Recommendations
@onready var _rec_list: VBoxContainer = %RecommendationList
@onready var _back: Button = %Back
@onready var _celebration: CelebrationBanner = %Celebration


func _ready() -> void:
	_back.pressed.connect(func() -> void: back_requested.emit())


func show_review(reviewed: AttemptReviewModel, grade_band: String, granted: Array = []) -> void:
	review = reviewed
	results_later = false
	_later.visible = false
	var total: int = reviewed.items.size()
	var correct: int = reviewed.correct_count()
	_score.text = tr("ui.result.correct_count").format({"correct": correct, "total": total})
	_score.visible = total > 0
	_points.visible = grade_band != BAND_COUNT_ONLY and reviewed.has_score
	_points.text = tr("ui.result.score").format({"score": _number(reviewed.score), "max": _number(reviewed.max_score)})
	_weak.visible = not reviewed.weak_objectives.is_empty()
	_weak.text = tr("ui.result.weak_count").format({"n": reviewed.weak_objectives.size()})
	_show_rewards(granted)
	_show_recommendations(reviewed.recommendations)
	_celebration.celebrate(correct if grade_band == BAND_COUNT_ONLY else 0, Settings.reduce_motion)


## `result_release` ≠ immediate and not released yet: "Kết quả sẽ có khi nhà trường công bố" (F063-AC-23).
func show_results_later(granted: Array = []) -> void:
	review = null
	results_later = true
	_score.visible = false
	_points.visible = false
	_weak.visible = false
	_later.visible = true
	_show_rewards(granted)
	_show_recommendations([])
	_celebration.celebrate(0, Settings.reduce_motion)


func celebration() -> CelebrationBanner:
	return _celebration


func score_text() -> String:
	return _score.text if _score.visible else ""


func points_visible() -> bool:
	return _points.visible


func rewards_visible() -> bool:
	return _rewards_box.visible


func recommendation_buttons() -> Array[Button]:
	var out: Array[Button] = []
	for child: Node in _rec_list.get_children():
		if child is Button:
			out.append(child as Button)
	return out


func _show_rewards(granted: Array) -> void:
	rewards.clear()
	for child: Node in _reward_list.get_children():
		child.queue_free()
	for entry: Variant in granted:
		if entry is Dictionary:
			rewards.append(entry as Dictionary)
			var row: HBoxContainer = HBoxContainer.new()
			row.theme_type_variation = &"ToolbarBox"
			var icon: TextureRect = TextureRect.new()
			icon.texture = REWARD_ICONS.get(String((entry as Dictionary).get("kind", "")), REWARD_ICON_ITEM) as Texture2D
			icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
			icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
			icon.custom_minimum_size = Vector2.ONE * UiTokens.COMPONENT_ICON_SIZE
			row.add_child(icon)
			var label: Label = Label.new()
			label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			label.text = _reward_text(entry as Dictionary)
			row.add_child(label)
			_reward_list.add_child(row)
	_rewards_box.visible = not rewards.is_empty()


func _show_recommendations(recs: Array) -> void:
	for child: Node in _rec_list.get_children():
		child.queue_free()
	var shown: int = 0
	for rec: Variant in recs:
		var r: AttemptReviewModel.Recommendation = rec as AttemptReviewModel.Recommendation
		if r == null or r.title.is_empty():
			continue
		if r.is_practice():
			var b: Button = Button.new()
			b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			b.text = r.title
			b.pressed.connect(func() -> void: practice_requested.emit(r))
			_rec_list.add_child(b)
		else:
			var label: Label = Label.new()
			label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			label.text = r.title
			_rec_list.add_child(label)
		shown += 1
	_recs_box.visible = shown > 0


func _reward_text(reward: Dictionary) -> String:
	var amount: int = int(reward.get("amount", 0)) if reward.get("amount") is int or reward.get("amount") is float else 0
	match String(reward.get("kind", "")):
		"xp":
			return tr("ui.result.reward_xp").format({"amount": amount})
		"badge":
			return tr("ui.result.reward_badge")
	return tr("ui.result.reward_item").format({"amount": maxi(amount, 1)})


static func _number(value: float) -> String:
	return str(int(value)) if is_equal_approx(value, roundf(value)) else "%.2f" % value
