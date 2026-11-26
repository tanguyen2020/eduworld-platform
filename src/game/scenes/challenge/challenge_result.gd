extends Control
## S06 (c) — end of a solo challenge (F069-AC-19; STUDENT_GAME_UX_CORE S06 §8, CHALLENGE_BOSS_SYSTEM §3.5; WorldRoot
## screen `challenge_result`): Mây and kind words for every outcome — `succeeded` "Hoàn thành thử thách!", `failed`
## "Em đã hoàn thành {n} hoạt động" + a review hint (never a red "Thất bại"), `abandoned` "Em đã dừng thử thách" —
## and the rewards the server granted (none to claim). Closing it ends the run for the client, so a level-up held
## during the run shows now (F069-AC-03). UI only (AGENTS R1).

signal closed()

var client: ChallengeClient = null
var attempt: ChallengeAttemptModel = null
var _ui: Dictionary = {}
var _mascot: CloudyMascot = null
var _heading: Label = null
var _line: Label = null
var _rewards_label: Label = null
var _rewards: RewardRow = null


func _ready() -> void:
	visible = false
	_ui = UiBuild.window(self, "ui.challenge.result_title", "area_challenge_mountain")
	# A card that hugs its content, centred (no big empty window after a short result).
	var win: PanelContainer = _ui["window"] as PanelContainer
	for side: Side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
		win.set_anchor(side, 0.5)
		win.set_offset(side, 0.0)
	win.grow_horizontal = Control.GROW_DIRECTION_BOTH
	win.grow_vertical = Control.GROW_DIRECTION_BOTH
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	body.add_child(coach["row"] as Control)
	_mascot = coach["mascot"] as CloudyMascot
	var bubble: VBoxContainer = coach["bubble_box"] as VBoxContainer
	_heading = UiBuild.data_label("", &"StemLabel")
	_heading.name = "Heading"
	bubble.add_child(_heading)
	_line = UiBuild.data_label("", &"SpeechBubbleLabel")
	_line.name = "Line"
	bubble.add_child(_line)
	_rewards_label = UiBuild.label("ui.challenge.rewards", &"HeadingLabel")
	body.add_child(_rewards_label)
	_rewards = RewardRow.new()
	_rewards.name = "Rewards"
	body.add_child(_rewards)
	var actions: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	actions.alignment = BoxContainer.ALIGNMENT_END
	body.add_child(actions)
	var board: Button = UiBuild.button("ui.challenge.back_to_board", "area_challenge_mountain", &"SecondaryButton")
	board.name = "BackToBoard"
	board.pressed.connect(_on_back_to_board)
	actions.add_child(board)
	var done: Button = UiBuild.button("ui.levelup.ok", "check")
	done.name = "Done"
	done.pressed.connect(close)
	actions.add_child(done)
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## WorldRoot: `ui_requested("challenge_result", {challenge_attempt_id})`.
func open(_args: Dictionary = {}) -> void:
	client = AppState.get_services().challenge
	attempt = client.attempt()
	(_ui["window"] as Control).custom_minimum_size.x = clampf(get_viewport_rect().size.x * 0.6,
			float(UiTokens.SIZE_LAYOUT_280) * 1.5, float(UiTokens.SIZE_LAYOUT_660) * 1.2)
	show()
	UiBuild.size_buttons(self)
	_render()


func close() -> void:
	hide()
	if client != null:
		client.dismiss_result()
	closed.emit()


func texts() -> PackedStringArray:
	return PackedStringArray([_heading.text, _line.text] + Array(_rewards.chip_texts()))


func _render() -> void:
	if attempt == null:
		return
	var n: String = LocaleFormat.number(attempt.activities_completed)
	match attempt.status:
		ChallengeAttemptModel.STATUS_SUCCEEDED:
			_mascot.mood = CloudyMascot.Mood.HAPPY
			_heading.text = tr("ui.challenge.result.succeeded")
			_line.text = tr("ui.challenge.result.succeeded_line").format({"n": n})
		ChallengeAttemptModel.STATUS_ABANDONED:
			_mascot.mood = CloudyMascot.Mood.ENCOURAGE
			_heading.text = tr("ui.challenge.result.abandoned")
			_line.text = tr("ui.challenge.result.abandoned_line").format({"n": n})
		_:
			_mascot.mood = CloudyMascot.Mood.ENCOURAGE
			_heading.text = tr("ui.challenge.result.completed_n").format({"n": n})
			_line.text = tr("ui.challenge.result.review_hint")
	_rewards.set_rewards(attempt.rewards, UiBuild.band() == "g1_3")
	_rewards_label.visible = not attempt.rewards.is_empty()


func _on_back_to_board() -> void:
	close()
	EventBus.ui_requested.emit(&"challenge_board", {})


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()
