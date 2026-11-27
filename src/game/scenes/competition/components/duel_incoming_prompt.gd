class_name DuelIncomingPrompt
extends PanelContainer
## Lời thách đấu đến — the friendly prompt over the world when a friend challenges the student (F109-AC-27; realtime
## `competition.duel.received`, server delta 2026-11-27; STUDENT_GAME_UX_SOCIAL §8 "Thách đấu bạn"):
## - the challenger's public card only (portrait + nickname + level — AGENTS R16), "{nickname} muốn thách đấu em!",
##   the subject (and topic) of the duel and "Luyện tập, không tính hạng";
## - "Nhận lời" (`acceptDuel` → the match opens) / "Để lần sau" (`declineDuel`);
## - it lives until the server `expires_at` (60 s): a calm bar of the time left; lớp 4–9 also read "Còn 45 giây",
##   lớp 1–3 see the bar only (no countdown numbers, AC-09 rule). No blinking, no sound, no pressure (R19);
## - an answer the server refuses (`410 DUEL_EXPIRED`, …) says it in one kind sentence and offers "Đóng".
## UI only: MatchmakingService does the work (AGENTS R1).

const PORTRAIT_SCALE: float = 1.1

var matchmaking: MatchmakingService = null
var duel: DuelModel = null
var _total_ms: int = 1
var _thumb: AvatarThumb = null
var _title: Label = null
var _detail: Label = null
var _level: Label = null
var _bar: ProgressBar = null
var _seconds: Label = null
var _status: Label = null
var _accept: Button = null
var _decline: Button = null
var _close: Button = null


func _init() -> void:
	name = "DuelIncomingPrompt"
	theme_type_variation = &"FormCard"
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	_build()


## Shows `incoming` (a newer duel replaces the one on screen).
func present(mm: MatchmakingService, incoming: DuelModel) -> void:
	matchmaking = mm
	duel = incoming
	_total_ms = maxi(1, duel.expires_ms - mm.now_ms())
	var look: AvatarConfigModel = AvatarConfigModel.from_dict(duel.other_avatar) if not duel.other_avatar.is_empty() else null
	_thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	_status.visible = false
	_accept.visible = true
	_decline.visible = true
	_close.visible = false
	_accept.disabled = false
	_decline.disabled = false
	_render()
	UiBuild.size_buttons(self)
	show()


func dismiss() -> void:
	hide()
	duel = null


## Advances the time left (server clock; ArenaHud calls it every frame); at `expires_at` the prompt closes by itself (the duel ends on the server).
func advance(_delta: float) -> void:
	if not visible or duel == null or matchmaking == null or _close.visible:
		return
	if duel.seconds_left(matchmaking.now_ms()) <= 0:
		matchmaking.dismiss_incoming_duel()
		dismiss()
		return
	_render_time()


func title_text() -> String:
	return _title.text


func detail_text() -> String:
	return _detail.text


func time_text() -> String:
	return _seconds.text if _seconds.visible else ""


func status_text() -> String:
	return _status.text if _status.visible else ""


func accept_button() -> Button:
	return _accept


func decline_button() -> Button:
	return _decline


func close_button() -> Button:
	return _close


func time_bar() -> ProgressBar:
	return _bar


func _render() -> void:
	if duel == null:
		return
	_title.text = tr("ui.arena.duel.incoming_prompt").format({"nickname": duel.other_nickname})
	var subject: String = matchmaking.subject_name(duel.subject_id) if matchmaking != null else ""
	var topic: String = matchmaking.topic_name(duel.subject_id, duel.topic_id) \
			if matchmaking != null and not duel.topic_id.is_empty() else ""
	_detail.text = tr("ui.arena.duel.incoming_topic").format({"subject": subject, "topic": topic}) if not topic.is_empty() \
			else tr("ui.arena.duel.incoming_subject").format({"subject": subject})
	_level.text = tr("ui.arena.duel.level").format({"level": duel.other_level})
	_level.visible = duel.other_level > 0
	UiBuild.fit_words(_title)
	UiBuild.fit_words(_detail)
	_render_time()


func _render_time() -> void:
	var left_ms: int = maxi(0, duel.expires_ms - matchmaking.now_ms()) if duel != null and matchmaking != null else 0
	_bar.value = clampf(float(left_ms) / float(_total_ms), 0.0, 1.0)
	# Lớp 1–3: the bar only, never a number counting down.
	_seconds.visible = UiBuild.band() != "g1_3"
	_seconds.text = tr("ui.arena.duel.seconds_left").format({"seconds": ceili(float(left_ms) / 1000.0)})


func _on_accept_pressed() -> void:
	if matchmaking == null or duel == null:
		return
	var item: DuelModel = duel
	_accept.disabled = true
	_decline.disabled = true
	var result: ApiResult = await matchmaking.accept_duel(item)
	if duel != item:
		return
	if result.ok:
		dismiss()
		return
	_say(CompetitionText.error_key(result.code))


func _on_decline_pressed() -> void:
	if matchmaking == null or duel == null:
		return
	var item: DuelModel = duel
	dismiss()
	await matchmaking.decline_duel(item)


func _say(key: String) -> void:
	_status.text = tr(key)
	_status.visible = true
	_accept.visible = false
	_decline.visible = false
	_close.visible = true
	UiBuild.size_buttons(self)


func _on_locale_changed(_locale: String) -> void:
	if visible and not _close.visible:
		_render()


func _ready() -> void:
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func _build() -> void:
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "Box"
	add_child(box)
	var top: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(top)
	_thumb = AvatarThumb.new()
	_thumb.standalone = true
	_thumb.name = "Portrait"
	var side: float = UiBuild.target() * PORTRAIT_SCALE
	_thumb.custom_minimum_size = Vector2(side, side)
	_thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	top.add_child(_thumb)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	top.add_child(texts)
	_title = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_title.name = "Title"
	texts.add_child(_title)
	_detail = UiBuild.keep_words(UiBuild.data_label("", &"InstructionLabel"))
	_detail.name = "Detail"
	texts.add_child(_detail)
	var tags: HFlowContainer = HFlowContainer.new()
	texts.add_child(tags)
	_level = ArenaUi.tag("", &"SectionTag")
	_level.name = "Level"
	_level.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	tags.add_child(_level)
	tags.add_child(ArenaUi.tag("ui.arena.mode.unranked", &"TileTagPending"))
	var time_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	time_row.name = "TimeRow"
	box.add_child(time_row)
	var clock: TextureRect = UiBuild.icon_rect("clock", 0.75)
	clock.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	time_row.add_child(clock)
	_bar = UiBuild.progress_bar(1.0)
	_bar.name = "TimeBar"
	_bar.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(_bar)
	time_row.add_child(_bar)
	_seconds = UiBuild.data_label("", &"MutedLabel", false)
	_seconds.name = "TimeLeft"
	_seconds.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	time_row.add_child(_seconds)
	_status = UiBuild.data_label("", &"InstructionLabel")
	_status.name = "Status"
	_status.visible = false
	box.add_child(_status)
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.name = "Buttons"
	buttons.alignment = FlowContainer.ALIGNMENT_END
	box.add_child(buttons)
	_decline = UiBuild.button("ui.arena.duel.decline", "cancel", &"SecondaryButton")
	_decline.name = "Decline"
	_decline.pressed.connect(_on_decline_pressed)
	buttons.add_child(_decline)
	_accept = UiBuild.button("ui.arena.duel.accept", "play_light")
	_accept.name = "Accept"
	_accept.pressed.connect(_on_accept_pressed)
	buttons.add_child(_accept)
	_close = UiBuild.button("ui.common.close", "cancel", &"SecondaryButton")
	_close.name = "Close"
	_close.visible = false
	_close.pressed.connect(dismiss)
	buttons.add_child(_close)
