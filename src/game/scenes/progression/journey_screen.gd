extends Control
## "Hành trình của tôi" (F069-AC-05; PROGRESSION_SYSTEM §4.6, STUDENT_GAME_UX_CORE S07 exit): the current level and
## title, the latest XP lines of `listXpHistory` (20 per page, "Xem thêm" follows the cursor) in friendly words
## (`ui.journey.source.<source_code>`, else the server label) and the level milestones of `listProgressionLevels`. No
## technical id is shown, and game XP/levels are never presented as school marks (FR-GAM-006: a note says so).
## Loading / empty / error (+ "Thử lại") states. UI only: ProgressionService does the work (AGENTS R1).

signal closed()

var entries: Array[XpEntryModel] = []
var next_cursor: String = ""
var service: ProgressionService = null
var _ui: Dictionary = {}
var _summary: Label = null
var _bar: ProgressBar = null
var _rows: VBoxContainer = null
var _levels: VBoxContainer = null
var _state: Label = null
var _retry: Button = null
var _more: Button = null


func _ready() -> void:
	visible = false
	_ui = UiBuild.window(self, "ui.journey.title", "flag")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	body.add_child(card)
	var card_box: VBoxContainer = UiBuild.vbox()
	card.add_child(card_box)
	_summary = UiBuild.data_label("", &"HeadingLabel")
	_summary.name = "Summary"
	card_box.add_child(_summary)
	_bar = UiBuild.progress_bar()
	_bar.name = "Bar"
	card_box.add_child(_bar)
	card_box.add_child(UiBuild.label("ui.journey.not_a_grade", &"MutedLabel"))
	var state_row: HBoxContainer = UiBuild.hbox()
	body.add_child(state_row)
	_state = UiBuild.label("", &"MutedLabel")
	_state.name = "StateLabel"
	UiBuild.expand(_state)
	state_row.add_child(_state)
	_retry = UiBuild.button("ui.common.retry", "rotate_right", &"SecondaryButton")
	_retry.name = "Retry"
	_retry.pressed.connect(refresh)
	state_row.add_child(_retry)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	var list: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(list)
	scroll.add_child(list)
	list.add_child(UiBuild.label("ui.journey.recent", &"HeadingLabel"))
	_rows = UiBuild.vbox()
	_rows.name = "Rows"
	list.add_child(_rows)
	_more = UiBuild.button("ui.journey.more", "arrow_down", &"SecondaryButton")
	_more.name = "More"
	_more.pressed.connect(load_more)
	list.add_child(_more)
	list.add_child(UiBuild.label("ui.journey.milestones", &"HeadingLabel"))
	_levels = UiBuild.vbox()
	_levels.name = "Levels"
	list.add_child(_levels)
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## WorldRoot: `ui_requested("journey", {})`.
func open(_args: Dictionary = {}) -> void:
	if service == null:
		service = AppState.get_services().progression
	show()
	UiBuild.size_buttons(self)
	refresh()


func close() -> void:
	hide()
	closed.emit()


## Coroutine: first page of XP lines + the level milestones (+ the current progression when not loaded yet).
func refresh() -> void:
	entries.clear()
	next_cursor = ""
	_set_state("ui.journey.loading", false)
	_render()
	if service.progression() == null:
		await service.refresh()
	var page: Dictionary = await service.load_history()
	if not bool(page["ok"]):
		_set_state("ui.journey.load_failed", true)
		_render()
		return
	entries.assign(page["entries"] as Array)
	next_cursor = String(page["next_cursor"])
	await service.load_levels()
	_set_state("ui.journey.empty" if entries.is_empty() else "", false)
	_render()


## Coroutine: the next page of the cursor.
func load_more() -> void:
	if next_cursor.is_empty():
		return
	var page: Dictionary = await service.load_history(next_cursor)
	if not bool(page["ok"]):
		_set_state("ui.journey.load_failed", true)
		return
	entries.append_array(page["entries"] as Array)
	next_cursor = String(page["next_cursor"])
	_render()


## Texts of the XP lines on screen (tests: no technical id shown).
func row_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for label: Node in _rows.find_children("*", "Label", true, false):
		out.append((label as Label).text)
	return out


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()


func _set_state(key: String, error: bool) -> void:
	_state.text = key
	_state.visible = not key.is_empty()
	_retry.visible = error


func _render() -> void:
	var p: ProgressionModel = service.progression() if service != null else null
	if p != null:
		var level_text: String = tr("ui.progression.level").format({"level": LocaleFormat.number(p.level)})
		_summary.text = level_text if p.title_text().is_empty() else "%s · %s" % [level_text, p.title_text()]
		_bar.value = p.fraction()
	_summary.visible = p != null
	_bar.visible = p != null
	UiBuild.clear(_rows)
	for e: XpEntryModel in entries:
		_rows.add_child(_row(e))
	_more.visible = not next_cursor.is_empty()
	UiBuild.clear(_levels)
	if service != null:
		for l: LevelModel in service.levels:
			_levels.add_child(_level_row(l, p))


func _row(e: XpEntryModel) -> Control:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	var h: HBoxContainer = UiBuild.hbox()
	card.add_child(h)
	h.add_child(UiBuild.icon_rect("star"))
	var label: Label = UiBuild.data_label(e.label())
	UiBuild.expand(label)
	h.add_child(label)
	var amount: Label = UiBuild.data_label(tr("ui.progression.xp_gained").format({"xp": LocaleFormat.number(e.amount)}),
			&"HeadingLabel", false)
	h.add_child(amount)
	if e.capped:
		var moon: TextureRect = UiBuild.icon_rect("moon", 0.75)
		moon.tooltip_text = "ui.hub.daily_cap_reached"
		h.add_child(moon)
	var when: int = ExamCountdown.iso_to_ms(e.created_at)
	if when > 0:
		h.add_child(UiBuild.data_label(LocaleFormat.date(LocaleFormat.from_unix(when / 1000)), &"MutedLabel", false))
	return card


func _level_row(l: LevelModel, p: ProgressionModel) -> Control:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	var h: HBoxContainer = UiBuild.hbox()
	card.add_child(h)
	var reached: bool = p != null and l.level <= p.level
	h.add_child(UiBuild.icon_rect("verdict_correct" if reached else "lock"))
	var name_text: String = tr("ui.progression.level").format({"level": LocaleFormat.number(l.level)})
	var title: String = L10n.pick(l.title)
	if p != null and l.level == p.level and title.is_empty():
		title = p.title_text()
	var label: Label = UiBuild.data_label(name_text if title.is_empty() else "%s · %s" % [name_text, title], &"", false)
	label.custom_minimum_size.x = UiBuild.target() * 3.0
	h.add_child(label)
	var rewards: RewardRow = RewardRow.new()
	UiBuild.expand(rewards)
	h.add_child(rewards)
	rewards.set_rewards(l.rewards)
	return card


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()
