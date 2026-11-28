class_name ClubManage
extends Control
## "Quản lý club" of the club owner (F112 US5, US6; F112-AC-18…AC-24, AC-26, AC-27; SOCIAL_SYSTEM §6.2–§6.3): opened by
## "Quản lý club" on the page of an active club the student owns (F111 ClubPage). A window with the shared title bar
## (red "✕ Đóng") and four section chips:
## - "Sửa thông tin": name (2–64, typing past 64 is blocked), description (≤ 500 with a counter, typing past it blocked),
##   an icon of the F111 library; "Lưu" sends only what changed (ClubManageService);
## - "Phó nhóm": every member (portrait, nickname, role in words + icon — nothing else of a member, AGENTS R16) with
##   "Chọn làm phó nhóm" (member) / "Thôi làm phó nhóm" (officer), each asked once ("Hủy" neutral); the owner's row has
##   no action;
## - "Mục tiêu tuần": the metric by picture + words (the server's accepted ones), the target with −/+ (1…100 000), an
##   optional title (≤ 128); no per-member share, no ranking, no reward to buy (AGENTS R19);
## - "Sự kiện club": the kind by picture + words (never "giải nội bộ"), title (1–128), description (≤ 1000), the day
##   (the next 7 days) and the hour + minute in ICT shown in the student's locale, an optional length; "Tạo sự kiện" is
##   grey for a time already past by the server clock; no countdown (AGENTS R19).
## Problems are a sentence under the form (moderation: a general sentence, nothing echoed — R15; `422` beside the field);
## a success is a 2-second toast. The action row stays under the scroll box on a phone (F125 compact). UI only:
## ClubManageService does the work (AGENTS R1).

signal closed()

const SECTION_EDIT: String = "edit"
const SECTION_OFFICERS: String = "officers"
const SECTION_GOAL: String = "goal"
const SECTION_EVENT: String = "event"
## [id, label key, icon].
const SECTIONS: Array = [
	[SECTION_EDIT, "ui.club.manage.section.edit", "edit"],
	[SECTION_OFFICERS, "ui.club.manage.section.officers", "crown"],
	[SECTION_GOAL, "ui.club.manage.section.goal", "flag"],
	[SECTION_EVENT, "ui.club.manage.section.event", "calendar"],
]
const TOAST_SEC: float = 2.0
## Days offered for a club event (today and the next six).
const EVENT_DAYS: int = 7
const MINUTE_STEP: int = 15
## Optional length of an event in minutes (0 = none).
const LENGTHS: PackedInt32Array = [0, 30, 60, 120]
const ICT_OFFSET_SEC: int = 7 * 3600
const DEFAULT_TARGET: int = 10
const DEFAULT_HOUR: int = 9

var service: ClubManageService = null
## Server clock (UTC seconds) — the event form never offers a time already past.
var now_source: Callable = func() -> int: return int(Time.get_unix_time_from_system())
var club_id: String = ""
var section: String = SECTION_EDIT
var metric: String = ""
var target: int = DEFAULT_TARGET
var event_kind: String = "practice_session"
var day_offset: int = 1
var hour: int = DEFAULT_HOUR
var minute: int = 0
var length_min: int = 0
var icon_id: String = ""
var _ui: Dictionary = {}
var _chips: Dictionary = {}
var _chip_icons: Dictionary = {}
var _pages: Dictionary = {}
var _error: PanelContainer = null
var _error_label: Label = null
var _toast: PanelContainer = null
var _toast_label: Label = null
var _toast_left: float = 0.0
var _footer: HBoxContainer = null
var _submit: Button = null
var _name: LineEdit = null
var _description: TextEdit = null
var _desc_counter: Label = null
var _field_labels: Dictionary = {}
var _icons: Dictionary = {}
var _officers: VBoxContainer = null
var _role_buttons: Dictionary = {}
var _confirm: PanelContainer = null
var _confirm_label: Label = null
var _confirm_user: String = ""
var _confirm_officer: bool = false
var _metrics: HFlowContainer = null
var _metric_buttons: Dictionary = {}
var _target_label: Label = null
var _goal_title: LineEdit = null
var _kinds: Dictionary = {}
var _event_title: LineEdit = null
var _event_description: TextEdit = null
var _event_counter: Label = null
var _days: HFlowContainer = null
var _day_buttons: Array[Button] = []
var _time_label: Label = null
var _length_buttons: Dictionary = {}
var _when_label: Label = null
var _busy: bool = false


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()


func bind(manage: ClubManageService) -> void:
	if service != null:
		for pair: Array in [[service.changed, _render], [service.saved, _on_saved], [service.busy_changed, _on_busy],
				[service.tools_revoked, _on_revoked]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	service = manage
	if service != null:
		service.changed.connect(_render)
		service.saved.connect(_on_saved)
		service.busy_changed.connect(_on_busy)
		service.tools_revoked.connect(_on_revoked)


## Coroutine: opens the tools of `id` (the page already checked it is the owner's active club).
func open_for(id: String) -> void:
	club_id = id
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	UiBuild.fit_window(_ui["window"] as Control)
	section = SECTION_EDIT
	_toast.visible = false
	_confirm.visible = false
	show()
	if service != null:
		await service.load_club(id)
	_fill_edit()
	select_section(SECTION_EDIT)


func close() -> void:
	hide()
	closed.emit()


func select_section(id: String) -> void:
	section = id
	for k: String in _chips.keys():
		(_chips[k] as Button).set_pressed_no_signal(k == section)
	UiBuild.mark_chips(_chips.values(), _chip_icons)
	for k: String in _pages.keys():
		(_pages[k] as Control).visible = k == section
	_confirm.visible = false
	_render()


# ---------- test hooks ----------

func section_button(id: String) -> Button:
	return _chips.get(id) as Button


func submit_button() -> Button:
	return _submit


func error_text() -> String:
	return _error_label.text if _error.visible else ""


func toast_text() -> String:
	return _toast_label.text if _toast.visible else ""


func field_error(field: String) -> String:
	var l: Label = _field_labels.get(field) as Label
	return l.text if l != null and l.visible else ""


func name_field() -> LineEdit:
	return _name


func description_field() -> TextEdit:
	return _description


func role_button(user_id: String) -> Button:
	return _role_buttons.get(user_id) as Button


func confirm_visible() -> bool:
	return _confirm.visible


## "Đồng ý" of the one-step confirmation (tests and the button).
func confirm_role() -> void:
	_confirm.visible = false
	if service != null and not _confirm_user.is_empty():
		await service.set_role(_confirm_user, _confirm_officer)


func metric_ids() -> PackedStringArray:
	var out: PackedStringArray = []
	for k: String in _metric_buttons.keys():
		if (_metric_buttons[k] as Button).visible:
			out.append(k)
	return out


func kind_ids() -> PackedStringArray:
	return PackedStringArray(_kinds.keys())


func target_text() -> String:
	return _target_label.text


func set_target(value: int) -> void:
	target = ClubFormOptions.clamp_target(value)
	_render()


func set_goal_title(text: String) -> void:
	_goal_title.text = text


func set_event(form: Dictionary) -> void:
	_event_title.text = String(form.get("title", _event_title.text))
	_event_description.text = String(form.get("description", _event_description.text))
	event_kind = String(form.get("event_kind", event_kind))
	day_offset = int(form.get("day_offset", day_offset))
	hour = int(form.get("hour", hour))
	minute = int(form.get("minute", minute))
	length_min = int(form.get("length", length_min))
	_render()


func when_text() -> String:
	return _when_label.text


## The event form as ClubFormOptions reads it (UTC seconds).
func event_form() -> Dictionary:
	var starts: int = _day_start(day_offset) + hour * 3600 + minute * 60
	return {"title": _event_title.text, "description": _event_description.text, "event_kind": event_kind,
		"starts_unix": starts, "ends_unix": starts + length_min * 60 if length_min > 0 else 0}


func all_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).is_visible_in_tree():
			out.append(tr((node as Label).text))
	return out


# ---------- actions ----------

func _on_submit() -> void:
	if service == null or _busy:
		return
	match section:
		SECTION_EDIT:
			await service.save_club({"name": _name.text, "description": _description.text, "icon_id": icon_id})
			if service.form_error == ClubManageService.KEY_CONFLICT:
				_fill_edit()
		SECTION_GOAL:
			await service.create_goal({"metric": metric, "target": target, "title": _goal_title.text})
		SECTION_EVENT:
			await service.create_event(event_form(), now_source.call() as int)


func _ask_role(user_id: String, officer: bool) -> void:
	var m: ClubMemberModel = service.member(user_id) if service != null else null
	if m == null:
		return
	_confirm_user = user_id
	_confirm_officer = officer
	_confirm_label.text = tr("ui.club.manage.officer.ask_make" if officer else "ui.club.manage.officer.ask_remove").format(
			{"nickname": m.nickname})
	_confirm.visible = true


func _on_saved(what: String) -> void:
	_toast_label.text = tr("ui.club.manage.saved.%s" % what)
	_toast.visible = true
	_toast_left = TOAST_SEC
	if what == "goal":
		_goal_title.text = ""
	elif what == "event":
		_event_title.text = ""
		_event_description.text = ""


func _on_busy(busy: bool) -> void:
	_busy = busy
	_render_footer()


func _on_revoked(_id: String) -> void:
	close()


func _process(delta: float) -> void:
	if _toast_left > 0.0:
		_toast_left -= delta
		if _toast_left <= 0.0:
			_toast.visible = false


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		if _confirm.visible:
			_confirm.visible = false
		else:
			close()


# ---------- rendering ----------

func _fill_edit() -> void:
	var c: ClubModel = service.club if service != null else null
	if c == null:
		return
	_name.text = c.name
	_description.text = c.description
	icon_id = c.icon_id


func _render() -> void:
	if service == null:
		return
	_error_label.text = tr(service.form_error) if not service.form_error.is_empty() else ""
	_error.visible = not service.form_error.is_empty()
	for field: String in _field_labels.keys():
		var l: Label = _field_labels[field] as Label
		l.text = tr(String(service.field_errors[field])) if service.field_errors.has(field) else ""
		l.visible = service.field_errors.has(field)
	for id: String in _icons.keys():
		(_icons[id] as Button).set_pressed_no_signal(id == icon_id)
	UiBuild.mark_chips(_icons.values())
	_desc_counter.text = "%s/%s" % [LocaleFormat.number(_description.text.length()), LocaleFormat.number(ClubModel.DESCRIPTION_MAX)]
	_render_officers()
	var offered: PackedStringArray = ClubFormOptions.metrics(service.accepted_metrics)
	if not metric in offered:
		metric = offered[0] if not offered.is_empty() else ""
	for id: String in _metric_buttons.keys():
		var b: Button = _metric_buttons[id] as Button
		b.visible = id in offered
		b.set_pressed_no_signal(id == metric)
	UiBuild.mark_chips(_metric_buttons.values())
	_target_label.text = LocaleFormat.number(target)
	for id: String in _kinds.keys():
		(_kinds[id] as Button).set_pressed_no_signal(id == event_kind)
	UiBuild.mark_chips(_kinds.values())
	for i: int in _day_buttons.size():
		_day_buttons[i].text = _day_text(i)
		_day_buttons[i].set_pressed_no_signal(i == day_offset)
	UiBuild.mark_chips(_day_buttons)
	_time_label.text = LocaleFormat.time({"hour": hour, "minute": minute})
	for len_min: int in _length_buttons.keys():
		(_length_buttons[len_min] as Button).set_pressed_no_signal(len_min == length_min)
	UiBuild.mark_chips(_length_buttons.values())
	var starts: int = _day_start(day_offset) + hour * 3600 + minute * 60
	_when_label.text = tr("ui.club.manage.event.when").format({"date": LiveOpsTime.ict_date(starts),
		"time": LiveOpsTime.ict_time(starts)})
	_event_counter.text = "%s/%s" % [LocaleFormat.number(_event_description.text.length()),
		LocaleFormat.number(ClubFormOptions.EVENT_DESCRIPTION_MAX)]
	_render_footer()


func _render_footer() -> void:
	_footer.visible = section != SECTION_OFFICERS
	match section:
		SECTION_EDIT:
			_submit.text = "ui.club.manage.save"
			var changes: Dictionary = ClubFormOptions.club_changes(service.club if service != null else null,
					{"name": _name.text, "description": _description.text, "icon_id": icon_id})
			_submit.disabled = _busy or changes.is_empty() or not ClubFormOptions.check_club(changes).is_empty()
		SECTION_GOAL:
			_submit.text = "ui.club.manage.goal.create"
			_submit.disabled = _busy or ClubFormOptions.goal_body({"metric": metric, "target": target,
				"title": _goal_title.text}, service.accepted_metrics if service != null else PackedStringArray()).is_empty()
		SECTION_EVENT:
			_submit.text = "ui.club.manage.event.create"
			_submit.disabled = _busy or not ClubFormOptions.check_event(event_form(), now_source.call() as int).is_empty()


func _render_officers() -> void:
	UiBuild.clear(_officers)
	_role_buttons.clear()
	if service == null:
		return
	for m: ClubMemberModel in service.members:
		var ui: Dictionary = FriendsUi.person_card(m.nickname, {}, "Member")
		var actions: HFlowContainer = ui["actions"] as HFlowContainer
		actions.add_child(FriendsUi.status(m.role_key(), m.role_icon()))
		if m.role == ClubModel.ROLE_MEMBER or m.role == ClubModel.ROLE_OFFICER:
			var make: bool = m.role == ClubModel.ROLE_MEMBER
			var b: Button = UiBuild.button("ui.club.manage.officer.make" if make else "ui.club.manage.officer.remove",
					"crown" if make else "cancel", &"" if make else &"SecondaryButton")
			b.name = "Role"
			b.disabled = _busy
			var id: String = m.user_id
			b.pressed.connect(func() -> void: _ask_role(id, make))
			actions.add_child(b)
			_role_buttons[m.user_id] = b
		_officers.add_child(ui["card"] as Control)
	UiBuild.size_buttons(_officers)


## UTC seconds of midnight ICT `offset` days after today (by the server clock).
func _day_start(offset: int) -> int:
	var now: int = now_source.call() as int
	var ict: int = now + ICT_OFFSET_SEC
	return ict - posmod(ict, 86400) + offset * 86400 - ICT_OFFSET_SEC


func _day_text(offset: int) -> String:
	if offset == 0:
		return tr("ui.club.manage.event.today")
	if offset == 1:
		return tr("ui.club.manage.event.tomorrow")
	return LiveOpsTime.ict_date(_day_start(offset) + 12 * 3600)


# ---------- build ----------

func _build() -> void:
	_ui = UiBuild.window(self, "ui.club.manage.title", "settings")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var chips: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	chips.name = "Sections"
	var group: ButtonGroup = ButtonGroup.new()
	for spec: Array in SECTIONS:
		var c: Button = UiBuild.chip(String(spec[1]), group)
		c.name = "Section_" + String(spec[0])
		c.icon = UiBuild.icon(String(spec[2]))
		_chip_icons[c] = c.icon
		c.pressed.connect(select_section.bind(String(spec[0])))
		chips.add_child(c)
		_chips[spec[0]] = c
	body.add_child(UiBuild.chip_scroller(chips))
	_toast = UiBuild.panel(&"InfoBannerPanel")
	_toast.name = "Toast"
	_toast.visible = false
	_toast_label = UiBuild.keep_words(UiBuild.data_label("", &"InfoBannerLabel"))
	_toast.add_child(_toast_label)
	body.add_child(_toast)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	var pages: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(pages)
	scroll.add_child(pages)
	_pages[SECTION_EDIT] = _build_edit(pages)
	_pages[SECTION_OFFICERS] = _build_officers(pages)
	_pages[SECTION_GOAL] = _build_goal(pages)
	_pages[SECTION_EVENT] = _build_event(pages)
	_error = UiBuild.panel(&"InlineErrorPanel")
	_error.name = "Error"
	_error.visible = false
	_error_label = UiBuild.keep_words(UiBuild.data_label("", &"InlineErrorLabel"))
	_error.add_child(_error_label)
	body.add_child(_error)
	# The action row sits under the scroll box: "Lưu" / "Tạo…" never scroll away on a phone (F125 compact).
	_footer = UiBuild.hbox(&"ToolbarBox")
	_footer.name = "Footer"
	_footer.alignment = BoxContainer.ALIGNMENT_END
	body.add_child(_footer)
	_submit = UiBuild.button("ui.club.manage.save", "check")
	_submit.name = "Submit"
	_submit.pressed.connect(_on_submit)
	_footer.add_child(_submit)
	UiBuild.size_buttons(self)


func _heading(parent: Control, key: String) -> void:
	parent.add_child(UiBuild.label(key, &"HeadingLabel"))


func _field_error(parent: Control, field: String) -> void:
	var l: Label = UiBuild.keep_words(UiBuild.data_label("", &"InlineErrorLabel"))
	l.name = "FieldError_" + field
	l.visible = false
	parent.add_child(l)
	_field_labels[field] = l


func _build_edit(pages: Control) -> Control:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "EditPage"
	pages.add_child(box)
	_heading(box, "ui.club.form.name")
	_name = LineEdit.new()
	_name.name = "Name"
	_name.max_length = ClubModel.NAME_MAX
	_name.custom_minimum_size.y = UiBuild.target()
	_name.text_changed.connect(func(_t: String) -> void: _render_footer())
	UiBuild.expand(_name)
	box.add_child(_name)
	_field_error(box, "name")
	var head: HBoxContainer = UiBuild.hbox()
	box.add_child(head)
	var dl: Label = UiBuild.label("ui.club.form.description", &"HeadingLabel")
	UiBuild.expand(dl)
	head.add_child(dl)
	_desc_counter = UiBuild.data_label("", &"MutedLabel", false)
	_desc_counter.name = "Counter"
	head.add_child(_desc_counter)
	_description = TextEdit.new()
	_description.name = "Description"
	_description.wrap_mode = TextEdit.LINE_WRAPPING_BOUNDARY
	_description.custom_minimum_size.y = UiBuild.target() * 1.6
	_description.text_changed.connect(func() -> void:
		_cap(_description, ClubModel.DESCRIPTION_MAX)
		_desc_counter.text = "%s/%s" % [LocaleFormat.number(_description.text.length()),
			LocaleFormat.number(ClubModel.DESCRIPTION_MAX)]
		_render_footer())
	UiBuild.expand(_description)
	box.add_child(_description)
	_field_error(box, "description")
	_heading(box, "ui.club.form.icon")
	var icons: HFlowContainer = HFlowContainer.new()
	icons.name = "Icons"
	box.add_child(icons)
	var ig: ButtonGroup = ButtonGroup.new()
	for id: String in ClubModel.ICONS.keys():
		var ib: Button = UiBuild.button("", ClubModel.icon_for(id), &"ChoiceChip")
		ib.name = "Icon_" + id
		ib.toggle_mode = true
		ib.button_group = ig
		ib.tooltip_text = "ui.club.form.icon"
		ib.accessibility_name = TranslationServer.translate("ui.club.form.icon")
		ib.pressed.connect(func() -> void:
			icon_id = id
			_render())
		icons.add_child(ib)
		_icons[id] = ib
	_field_error(box, "icon_id")
	return box


func _build_officers(pages: Control) -> Control:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "OfficersPage"
	pages.add_child(box)
	box.add_child(UiBuild.keep_words(UiBuild.label("ui.club.manage.officer.hint", &"MutedLabel")))
	_confirm = UiBuild.panel(&"WarningBannerPanel")
	_confirm.name = "Confirm"
	_confirm.visible = false
	var cbox: VBoxContainer = UiBuild.vbox(&"FieldBox")
	_confirm.add_child(cbox)
	_confirm_label = UiBuild.keep_words(UiBuild.data_label("", &"WarningBannerLabel"))
	cbox.add_child(_confirm_label)
	var crow: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	crow.alignment = BoxContainer.ALIGNMENT_END
	cbox.add_child(crow)
	var cancel: Button = UiBuild.button("ui.common.cancel", "cross", &"SecondaryButton")
	cancel.name = "Cancel"
	cancel.pressed.connect(func() -> void: _confirm.visible = false)
	crow.add_child(cancel)
	var ok: Button = UiBuild.button("ui.club.manage.officer.confirm", "check")
	ok.name = "Confirm"
	ok.pressed.connect(confirm_role)
	crow.add_child(ok)
	box.add_child(_confirm)
	_officers = UiBuild.vbox(&"FormBox")
	_officers.name = "Members"
	box.add_child(_officers)
	return box


func _build_goal(pages: Control) -> Control:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "GoalPage"
	pages.add_child(box)
	box.add_child(UiBuild.keep_words(UiBuild.label("ui.club.manage.goal.hint", &"MutedLabel")))
	_heading(box, "ui.club.manage.goal.metric")
	_metrics = HFlowContainer.new()
	_metrics.name = "Metrics"
	box.add_child(_metrics)
	var mg: ButtonGroup = ButtonGroup.new()
	for spec: Array in ClubFormOptions.METRICS:
		var b: Button = UiBuild.chip(String(spec[1]), mg)
		b.name = "Metric_" + String(spec[0])
		b.icon = UiBuild.icon(String(spec[2]))
		var id: String = String(spec[0])
		b.pressed.connect(func() -> void:
			metric = id
			_render())
		_metrics.add_child(b)
		_metric_buttons[id] = b
	_field_error(box, "metric")
	_heading(box, "ui.club.manage.goal.target")
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.name = "Target"
	box.add_child(row)
	var minus: Button = UiBuild.button("", "minus", &"SecondaryButton")
	minus.name = "Minus"
	minus.tooltip_text = "ui.club.manage.goal.less"
	minus.accessibility_name = TranslationServer.translate("ui.club.manage.goal.less")
	minus.pressed.connect(func() -> void: set_target(target - (ClubFormOptions.TARGET_STEP if target > ClubFormOptions.TARGET_STEP else 1)))
	row.add_child(minus)
	_target_label = UiBuild.data_label("", &"HeadingLabel", false)
	_target_label.name = "TargetValue"
	_target_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_target_label)
	var plus: Button = UiBuild.button("", "plus", &"SecondaryButton")
	plus.name = "Plus"
	plus.tooltip_text = "ui.club.manage.goal.more"
	plus.accessibility_name = TranslationServer.translate("ui.club.manage.goal.more")
	plus.pressed.connect(func() -> void: set_target(target + ClubFormOptions.TARGET_STEP))
	row.add_child(plus)
	_field_error(box, "target")
	_heading(box, "ui.club.manage.goal.title")
	_goal_title = LineEdit.new()
	_goal_title.name = "GoalTitle"
	_goal_title.max_length = ClubFormOptions.GOAL_TITLE_MAX
	_goal_title.placeholder_text = "ui.club.manage.goal.title_hint"
	_goal_title.custom_minimum_size.y = UiBuild.target()
	_goal_title.text_changed.connect(func(_t: String) -> void: _render_footer())
	UiBuild.expand(_goal_title)
	box.add_child(_goal_title)
	_field_error(box, "title")
	return box


func _build_event(pages: Control) -> Control:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "EventPage"
	pages.add_child(box)
	_heading(box, "ui.club.manage.event.kind")
	var kinds: HFlowContainer = HFlowContainer.new()
	kinds.name = "Kinds"
	box.add_child(kinds)
	var kg: ButtonGroup = ButtonGroup.new()
	for spec: Array in ClubFormOptions.EVENT_KINDS:
		var b: Button = UiBuild.chip(String(spec[1]), kg)
		b.name = "Kind_" + String(spec[0])
		b.icon = UiBuild.icon(String(spec[2]))
		var id: String = String(spec[0])
		b.pressed.connect(func() -> void:
			event_kind = id
			_render())
		kinds.add_child(b)
		_kinds[id] = b
	_heading(box, "ui.club.manage.event.title")
	_event_title = LineEdit.new()
	_event_title.name = "EventTitle"
	_event_title.max_length = ClubFormOptions.EVENT_TITLE_MAX
	_event_title.custom_minimum_size.y = UiBuild.target()
	_event_title.text_changed.connect(func(_t: String) -> void: _render_footer())
	UiBuild.expand(_event_title)
	box.add_child(_event_title)
	_field_error(box, "event_title")
	var head: HBoxContainer = UiBuild.hbox()
	box.add_child(head)
	var dl: Label = UiBuild.label("ui.club.manage.event.description", &"HeadingLabel")
	UiBuild.expand(dl)
	head.add_child(dl)
	_event_counter = UiBuild.data_label("", &"MutedLabel", false)
	_event_counter.name = "EventCounter"
	head.add_child(_event_counter)
	_event_description = TextEdit.new()
	_event_description.name = "EventDescription"
	_event_description.wrap_mode = TextEdit.LINE_WRAPPING_BOUNDARY
	_event_description.custom_minimum_size.y = UiBuild.target() * 1.6
	_event_description.text_changed.connect(func() -> void:
		_cap(_event_description, ClubFormOptions.EVENT_DESCRIPTION_MAX)
		_render())
	UiBuild.expand(_event_description)
	box.add_child(_event_description)
	_heading(box, "ui.club.manage.event.day")
	_days = HFlowContainer.new()
	_days.name = "Days"
	box.add_child(_days)
	var dg: ButtonGroup = ButtonGroup.new()
	for i: int in EVENT_DAYS:
		var b: Button = UiBuild.chip("", dg)
		b.name = "Day_%d" % i
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		var offset: int = i
		b.pressed.connect(func() -> void:
			day_offset = offset
			_render())
		_days.add_child(b)
		_day_buttons.append(b)
	_heading(box, "ui.club.manage.event.time")
	var trow: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	trow.name = "Time"
	box.add_child(trow)
	var earlier: Button = UiBuild.button("", "minus", &"SecondaryButton")
	earlier.name = "Earlier"
	earlier.tooltip_text = "ui.club.manage.event.earlier"
	earlier.accessibility_name = TranslationServer.translate("ui.club.manage.event.earlier")
	earlier.pressed.connect(func() -> void: _shift_time(-MINUTE_STEP))
	trow.add_child(earlier)
	_time_label = UiBuild.data_label("", &"HeadingLabel", false)
	_time_label.name = "TimeValue"
	_time_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	trow.add_child(_time_label)
	var later: Button = UiBuild.button("", "plus", &"SecondaryButton")
	later.name = "Later"
	later.tooltip_text = "ui.club.manage.event.later"
	later.accessibility_name = TranslationServer.translate("ui.club.manage.event.later")
	later.pressed.connect(func() -> void: _shift_time(MINUTE_STEP))
	trow.add_child(later)
	_heading(box, "ui.club.manage.event.length")
	var lengths: HFlowContainer = HFlowContainer.new()
	lengths.name = "Lengths"
	box.add_child(lengths)
	var lg: ButtonGroup = ButtonGroup.new()
	for len_min: int in LENGTHS:
		var b: Button = UiBuild.chip("ui.club.manage.event.length_none" if len_min == 0 else "", lg)
		b.name = "Length_%d" % len_min
		if len_min > 0:
			b.text = tr("ui.club.manage.event.length_min").format({"minutes": LocaleFormat.number(len_min)})
			b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		var value: int = len_min
		b.pressed.connect(func() -> void:
			length_min = value
			_render())
		lengths.add_child(b)
		_length_buttons[len_min] = b
	_when_label = UiBuild.keep_words(UiBuild.data_label("", &"InstructionLabel"))
	_when_label.name = "When"
	box.add_child(_when_label)
	return box


func _shift_time(delta_min: int) -> void:
	var total: int = posmod(hour * 60 + minute + delta_min, 24 * 60)
	hour = total / 60
	minute = total % 60
	_render()


## Typing past `limit` characters is blocked (the extra characters are dropped, the caret stays at the end).
static func _cap(edit: TextEdit, limit: int) -> void:
	if edit.text.length() <= limit:
		return
	edit.text = edit.text.left(limit)
	edit.set_caret_line(edit.get_line_count() - 1)
	edit.set_caret_column(edit.get_line(edit.get_line_count() - 1).length())
