extends Control
## Hộp thông báo — the notification inbox (F123 US2/US4; SOCIAL_SYSTEM §8, CHAT_SYSTEM §10, STUDENT_GAME_UX_CORE S03
## §12 "mọi thông báo có thể mở lại trong chuông"). A centred window like S15 Cài đặt (F114 tier 1 frame): blue title bar
## "THÔNG BÁO" with the red Close button, then the student's notifications newest first. Each row is a card with the
## icon of its kind, the text in the current language (message code + params, else the server title/body — F123-AC-17),
## the day and time, a "Mới" pill while it was unread when the window opened (words, not colour only) and, when this
## client has the screen of its action, a "Xem" button (F123-AC-18). Opening the window loads the first page and marks
## the rows read through NotificationService (WS `notification.item.mark_read`); "Xem thêm" follows the cursor;
## "Đã xem hết" (when more unread rows wait on later pages) marks everything read. Empty: Mây says where notifications
## come from. Loading / error (+ "Thử lại") states. Only nicknames and content names appear (AGENTS R16). UI only: the
## service does the work (AGENTS R1).

signal closed()

## Icon of each notification kind (design/icons/game), drawn on the light row card.
const KIND_ICONS: Dictionary = {
	"friend_request": "heart", "party_invitation": "party", "assignment_new": "assignments", "assignment_due": "clock",
	"assignment_results_released": "tick", "level_reached": "star", "achievement_unlocked": "medal",
	"reward_granted": "gift", "quest_available": "journal", "tournament_update": "flag", "moderation_notice": "info",
}
const DEFAULT_ICON: String = "bell"
const ROW_ICON_SCALE: float = 1.2

var service: NotificationService = null
var open_tween: Tween = null
## Ids that were unread when they were shown in this opening (their "Mới" pill stays until the window closes).
var fresh: Dictionary = {}
var _window: PanelContainer = null
var _title: Label = null
var _close: Button = null
var _state: Label = null
var _retry: Button = null
var _read_all: Button = null
var _scroll: ScrollContainer = null
var _rows: VBoxContainer = null
var _empty: Control = null
var _more: Button = null


func _ready() -> void:
	visible = false
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)
	_unbind()


## WorldRoot: `ui_requested("notification_inbox", {})`.
func open(_args: Dictionary = {}) -> void:
	if service == null and AppState.services != null:
		service = AppState.get_services().notifications
	_bind(service)
	fresh.clear()
	show()
	UiBuild.size_buttons(self)
	open_tween = UiMotion.pop_in(_window, Settings.reduce_motion)
	_close.grab_focus.call_deferred(true)
	refresh()


func close() -> void:
	hide()
	fresh.clear()
	_unbind()
	closed.emit()


## Coroutine: the first page (shows what the service already holds while it loads), then marks the rows read.
func refresh() -> void:
	if service == null:
		return
	_set_state("ui.notification.inbox.loading", false)
	_render()
	var result: ApiResult = await service.refresh_inbox()
	if not visible:
		return
	if not result.ok:
		_set_state("ui.notification.inbox.load_failed", true)
		_render()
		return
	_set_state("", false)
	await _mark_shown()


## Coroutine: the next cursor page ("Xem thêm"), its rows marked read too.
func load_more() -> void:
	if service == null:
		return
	var result: ApiResult = await service.load_more()
	if not result.ok and not service.page_cursor.is_empty():
		_set_state("ui.notification.inbox.load_failed", true)
		return
	await _mark_shown()


## Number of notification rows on screen (tests).
func row_count() -> int:
	return _rows.get_child_count()


## Texts of the rows on screen (tests: only nicknames / content names, no technical id).
func row_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for row: Node in _rows.get_children():
		out.append((row.find_child("Text", true, false) as Label).text)
	return out


## The "Xem" button of row `index`, or null when its screen is not in this client.
func open_button(index: int) -> Button:
	if index < 0 or index >= _rows.get_child_count():
		return null
	return _rows.get_child(index).find_child("Open", true, false) as Button


## The "Mới" pill of row `index` is shown.
func is_marked_new(index: int) -> bool:
	if index < 0 or index >= _rows.get_child_count():
		return false
	var pill: Control = _rows.get_child(index).find_child("New", true, false) as Control
	return pill != null and pill.visible


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()


## Follows the list of `next` while the window is open.
func _bind(next: NotificationService) -> void:
	_unbind()
	service = next
	if service != null:
		service.items_changed.connect(_on_items_changed)


func _unbind() -> void:
	if service != null and service.items_changed.is_connected(_on_items_changed):
		service.items_changed.disconnect(_on_items_changed)


## Every unread row now on screen is remembered as "Mới" and marked read (F123-AC-18).
func _mark_shown() -> void:
	var targets: Array[NotificationModel] = []
	for m: NotificationModel in service.items:
		if not m.read and not m.is_local():
			fresh[m.notification_id] = true
			targets.append(m)
	_render()
	if not targets.is_empty():
		await service.mark_seen(targets)


func _on_items_changed() -> void:
	if not visible:
		return
	# A notification arriving while the window is open is seen at once.
	var arrived: bool = false
	for m: NotificationModel in service.items:
		if not m.read and not m.is_local() and not fresh.has(m.notification_id) and not service.is_loading():
			arrived = true
	if arrived:
		_mark_shown()
	else:
		_render()


func _set_state(key: String, error: bool) -> void:
	_state.text = key
	_state.visible = not key.is_empty()
	_retry.visible = error


func _render() -> void:
	if _rows == null:
		return
	UiBuild.clear(_rows)
	var list: Array[NotificationModel] = service.items if service != null else [] as Array[NotificationModel]
	for i: int in list.size():
		_rows.add_child(_row(list[i], i))
	var loading: bool = service != null and service.is_loading()
	_empty.visible = list.is_empty() and not loading and not _retry.visible
	_more.visible = service != null and not service.page_cursor.is_empty()
	_read_all.visible = service != null and service.unread_count > 0 and service.inbox_loaded
	UiBuild.size_buttons(self)


func _row(item: NotificationModel, index: int) -> Control:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	card.name = "Row%d" % index
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect(String(KIND_ICONS.get(item.kind, DEFAULT_ICON)), ROW_ICON_SCALE)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var texts: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(texts)
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(texts)
	var line: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	texts.add_child(line)
	var pill: Label = UiBuild.label("ui.notification.inbox.new", &"CountBadge", false)
	pill.name = "New"
	pill.uppercase = true
	pill.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	pill.visible = fresh.has(item.notification_id) or (not item.read and not item.is_local())
	line.add_child(pill)
	var text: Label = UiBuild.data_label(item.display_text())
	text.name = "Text"
	UiBuild.expand(text)
	line.add_child(text)
	var when: String = _when(item.created_at_ms)
	if not when.is_empty():
		var time: Label = UiBuild.data_label(when, &"MutedLabel")
		time.name = "When"
		texts.add_child(time)
	if service.can_open(item):
		var open_button: Button = UiBuild.button("ui.notification.inbox.open", "next_light")
		open_button.name = "Open"
		open_button.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		open_button.pressed.connect(_on_open_pressed.bind(item))
		row.add_child(open_button)
	return card


## "20/11/2026 lúc 08:00" in the device time zone and the current language.
static func _when(epoch_ms: int) -> String:
	if epoch_ms <= 0:
		return ""
	var local: Dictionary = LocaleFormat.from_unix(int(epoch_ms / 1000))
	return String(TranslationServer.translate("ui.notification.inbox.when")).format({
		"date": LocaleFormat.date(local), "time": LocaleFormat.time(local)})


func _on_open_pressed(item: NotificationModel) -> void:
	service.open(item)


func _on_read_all_pressed() -> void:
	await service.mark_all_read()
	_render()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()


# ---------- layout (F114 window: scrim, frame, title bar, red Close) ----------

func _build() -> void:
	add_child(UiBuild.scrim())
	_window = UiBuild.panel()
	_window.name = "Window"
	_window.set_anchors_preset(Control.PRESET_FULL_RECT)
	_window.anchor_left = 0.07
	_window.anchor_top = 0.04
	_window.anchor_right = 0.93
	_window.anchor_bottom = 0.96
	_window.offset_left = 0.0
	_window.offset_top = 0.0
	_window.offset_right = 0.0
	_window.offset_bottom = 0.0
	add_child(_window)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "Box"
	_window.add_child(box)
	var bar: PanelContainer = UiBuild.panel(&"WindowTitleBar")
	bar.name = "TitleBar"
	box.add_child(bar)
	var bar_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	bar.add_child(bar_row)
	_title = UiBuild.label("ui.notification.inbox.title", &"WindowTitleLabel", false)
	_title.name = "Title"
	_title.uppercase = true
	_title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_title)
	bar_row.add_child(_title)
	_close = UiBuild.button("ui.common.close", "cross", &"DangerButton")
	_close.name = "Close"
	_close.pressed.connect(close)
	bar_row.add_child(_close)
	var tools: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	tools.name = "Tools"
	box.add_child(tools)
	_state = UiBuild.label("", &"MutedLabel")
	_state.name = "StateLabel"
	UiBuild.expand(_state)
	tools.add_child(_state)
	_retry = UiBuild.button("ui.common.retry", "rotate_right", &"SecondaryButton")
	_retry.name = "Retry"
	_retry.visible = false
	_retry.pressed.connect(refresh)
	tools.add_child(_retry)
	_read_all = UiBuild.button("ui.notification.inbox.read_all", "check", &"SecondaryButton")
	_read_all.name = "ReadAll"
	_read_all.visible = false
	_read_all.pressed.connect(_on_read_all_pressed)
	tools.add_child(_read_all)
	_scroll = ScrollContainer.new()
	_scroll.name = "Scroll"
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(_scroll, true, true)
	box.add_child(_scroll)
	var list: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(list)
	_scroll.add_child(list)
	_rows = UiBuild.vbox(&"FormBox")
	_rows.name = "Rows"
	list.add_child(_rows)
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	_empty = coach["row"] as Control
	_empty.name = "Empty"
	_empty.visible = false
	(coach["bubble_box"] as VBoxContainer).add_child(UiBuild.label("ui.notification.inbox.empty", &"SpeechBubbleLabel"))
	# The bubble keeps the height of its words (not of Mây).
	(_empty.get_node("Bubble") as Control).size_flags_vertical = Control.SIZE_SHRINK_CENTER
	list.add_child(_empty)
	_more = UiBuild.button("ui.notification.inbox.more", "arrow_down", &"SecondaryButton")
	_more.name = "More"
	_more.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_more.visible = false
	_more.pressed.connect(load_more)
	list.add_child(_more)
