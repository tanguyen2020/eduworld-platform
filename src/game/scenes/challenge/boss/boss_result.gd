class_name BossResult
extends Control
## S06 (c) — the end of a boss encounter (F110-AC-08; STUDENT_GAME_UX_CORE S06 §8, CHALLENGE_BOSS_SYSTEM §3.5): a window
## over the arena with Mây and, from the server's `boss.encounter.ended`:
## - `defeated`: "Hạ {boss}!" and THIS student's rewards (`rewards`, only sent to a member who reached the minimum
##   contribution) — a defeated boss without rewards for the student says so kindly;
## - `time_up` / `abandoned`: "Em đã tới phase {n}/{total}" + a review hint — never the word "Thất bại" (NFR-SAFE-003).
## The red ✕ Đóng and the main button go back: to the zone (solo) or the party lobby (party, after
## `party.session.ended`). UI only.

signal done()

var outcome: String = ""
var _ui: Dictionary = {}
var _coach_slot: VBoxContainer = null
var _rewards: RewardRow = null
var _hint: Label = null
var _back: Button = null


func _ready() -> void:
	name = "BossResult"
	visible = false
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_ui = UiBuild.window(self, "ui.boss.result_title", "medal")
	var win: PanelContainer = _ui["window"] as PanelContainer
	win.anchor_left = 0.12
	win.anchor_right = 0.88
	win.anchor_top = 0.1
	win.anchor_bottom = 0.9
	(_ui["close"] as Button).pressed.connect(_finish)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(box)
	scroll.add_child(box)
	_coach_slot = UiBuild.vbox()
	_coach_slot.name = "Outcome"
	box.add_child(_coach_slot)
	_rewards = RewardRow.new()
	_rewards.name = "Rewards"
	box.add_child(_rewards)
	_hint = UiBuild.keep_words(UiBuild.label("", &"MutedLabel"))
	_hint.name = "Hint"
	box.add_child(_hint)
	var buttons: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	buttons.name = "Buttons"
	buttons.alignment = BoxContainer.ALIGNMENT_END
	body.add_child(buttons)
	_back = UiBuild.button("ui.boss.back_zone", "next_light")
	_back.name = "Back"
	_back.pressed.connect(_finish)
	buttons.add_child(_back)


## `result` = BossStore.result; `boss_title` in the student's language; `party` picks "Về nhóm".
func show_result(result: Dictionary, boss_title: String, party: bool) -> void:
	outcome = String(result.get("outcome", ""))
	UiBuild.clear(_coach_slot)
	var defeated: bool = outcome == BossEncounterModel.OUTCOME_DEFEATED
	var rewards: Array[RewardModel] = RewardModel.list_from(result.get("rewards", []))
	var key: String = "ui.boss.defeated" if defeated else "ui.boss.reached_phase"
	if defeated and rewards.is_empty():
		key = "ui.boss.defeated_no_reward"
	var args: Dictionary = {"boss": boss_title, "n": LocaleFormat.number(int(result.get("phases_cleared", 0))),
		"total": LocaleFormat.number(maxi(1, int(result.get("phase_count", 1))))}
	_coach_slot.add_child(ArenaUi.coach_line(key, args, CloudyMascot.Mood.HAPPY if defeated else CloudyMascot.Mood.ENCOURAGE))
	_rewards.set_rewards(rewards, UiBuild.band() == "g1_3")
	_rewards.visible = not rewards.is_empty()
	_hint.text = "" if defeated else "ui.boss.review_hint"
	_hint.visible = not defeated
	_back.text = "ui.boss.back_lobby" if party else "ui.boss.back_zone"
	_back.icon = UiBuild.icon("party" if party else "next_light")
	UiBuild.size_buttons(self)
	show()
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)


## The encounter is gone for this student (`ROOM_NOT_MEMBER` / `404`): one general sentence and "Về Quảng trường".
func show_message(key: String) -> void:
	outcome = ""
	UiBuild.clear(_coach_slot)
	_coach_slot.add_child(ArenaUi.coach_line(key, {}, CloudyMascot.Mood.ENCOURAGE))
	_rewards.visible = false
	_hint.visible = false
	_back.text = "ui.boss.back_hub"
	_back.icon = UiBuild.icon("next_light")
	UiBuild.size_buttons(self)
	show()


func headline() -> String:
	var l: Label = _coach_slot.find_child("CoachLine", true, false) as Label
	return l.text if l != null else ""


func reward_texts() -> PackedStringArray:
	return _rewards.chip_texts() if _rewards.visible else PackedStringArray()


func back_button() -> Button:
	return _back


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		_finish()


func _finish() -> void:
	hide()
	done.emit()
