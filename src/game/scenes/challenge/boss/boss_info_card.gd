class_name BossInfoCard
extends Control
## S06 (a) — the boss card of a gate / NPC (F110-AC-01; STUDENT_GAME_UX_CORE S06): `ui_requested("boss_info",
## {area_id | boss_id})` lists the bosses the server gives this student (`listBosses` — grade, school, availability are
## the server's) with the name, the number of phases and how it can be played. A boss with `solo` has "Chơi một mình"
## (`createBossEncounter` `mode = solo`, the key kept while retried) then opens the BossArena; every boss with `party`
## has "Rủ bạn" (the Party screen, F102); a boss without `solo` has ONLY "Rủ bạn". A refusal shows a friendly sentence
## (`CHALLENGE_NOT_ELIGIBLE`, `CHALLENGE_ATTEMPT_LIMIT`). UI only: BossClient does the work (AGENTS R1).

signal closed()

const ARENA_SCREEN: StringName = &"boss_arena"
const PARTY_SCREEN: StringName = &"party"

var services: ServiceLocator = null
var client: BossClient = null
var area_id: String = ""
var boss_id: String = ""
var _ui: Dictionary = {}
var _list: VBoxContainer = null
var _state: Label = null
var _notice: Dictionary = {}
var _cards: Dictionary = {}


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_ui = UiBuild.window(self, "ui.boss.info_title", "area_challenge_mountain")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	_notice = ArenaUi.notice()
	body.add_child(_notice["panel"] as PanelContainer)
	_state = UiBuild.keep_words(UiBuild.label("", &"MutedLabel"))
	_state.name = "State"
	body.add_child(_state)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	_list = UiBuild.vbox(&"FormBox")
	_list.name = "Bosses"
	UiBuild.expand(_list)
	scroll.add_child(_list)
	EventBus.locale_changed.connect(func(_l: String) -> void:
		if visible:
			render())


func _exit_tree() -> void:
	_unbind()


func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	_unbind()
	client = services.boss if services != null else null
	if client != null:
		client.notice_requested.connect(_on_notice)
	area_id = String(args.get("area_id", AppState.current_zone))
	boss_id = String(args.get("boss_id", ""))
	ArenaUi.show_notice(_notice, "")
	show()
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)
	(_ui["close"] as Button).grab_focus.call_deferred(true)
	await refresh()


func close() -> void:
	hide()
	_unbind()
	closed.emit()


## Coroutine: `listBosses` of the area (or the one boss asked for).
func refresh() -> void:
	if client == null:
		return
	_state.text = "ui.boss.loading"
	_state.visible = true
	var result: ApiResult = await client.list("" if not boss_id.is_empty() else area_id)
	if not visible:
		return
	_state.visible = not result.ok
	_state.text = "" if result.ok else "ui.boss.load_failed"
	render()


func render() -> void:
	UiBuild.clear(_list)
	_cards.clear()
	if client == null:
		return
	var shown: int = 0
	for b: BossSummary in client.bosses:
		if not boss_id.is_empty() and b.boss_id != boss_id:
			continue
		_list.add_child(_card(b))
		shown += 1
	if shown == 0 and not _state.visible:
		_list.add_child(FriendsUi.empty_line("ui.boss.none"))
	UiBuild.size_buttons(self)


func solo_button(id: String) -> Button:
	var card: Control = _cards.get(id) as Control
	return card.find_child("Solo", true, false) as Button if card != null else null


func party_button(id: String) -> Button:
	var card: Control = _cards.get(id) as Control
	return card.find_child("Party", true, false) as Button if card != null else null


func notice_text() -> String:
	return (_notice["label"] as Label).text if (_notice["panel"] as PanelContainer).visible else ""


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


func _card(b: BossSummary) -> Control:
	var card: PanelContainer = UiBuild.panel(&"FormCard")
	card.name = "Boss"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	var fig: BossFigure = BossFigure.new()
	fig.custom_minimum_size = Vector2.ONE * float(UiTokens.COMPONENT_COACH_SIZE)
	fig.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(fig)
	var texts: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(texts)
	row.add_child(texts)
	var title: Label = UiBuild.keep_words(UiBuild.data_label(b.display_title(), &"HeadingLabel"))
	title.name = "BossTitle"
	texts.add_child(title)
	var facts: PackedStringArray = [tr("ui.boss.phase_count").format({"n": LocaleFormat.number(b.phase_count)})]
	if b.allows_party():
		facts.append(tr("ui.boss.party_size").format({"min": LocaleFormat.number(maxi(2, b.party_size_min)),
			"max": LocaleFormat.number(b.party_size_max)}))
	facts.append(tr("ui.boss.modes.solo_party" if b.allows_solo() and b.allows_party()
			else ("ui.boss.modes.solo" if b.allows_solo() else "ui.boss.modes.party")))
	var info: Label = UiBuild.keep_words(UiBuild.data_label(" · ".join(facts), &"MutedLabel"))
	info.name = "Facts"
	texts.add_child(info)
	if b.defeated_before:
		texts.add_child(FriendsUi.status("ui.boss.defeated_before", "medal"))
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	texts.add_child(actions)
	if b.allows_solo():
		var solo: Button = UiBuild.button("ui.boss.solo", "play_light")
		solo.name = "Solo"
		solo.pressed.connect(_on_solo.bind(b, solo))
		actions.add_child(solo)
	if b.allows_party() or not b.allows_solo():
		var party: Button = UiBuild.button("ui.boss.invite_friends", "group", &"SecondaryButton" if b.allows_solo() else &"")
		party.name = "Party"
		party.pressed.connect(_on_party)
		actions.add_child(party)
	_cards[b.boss_id] = card
	return card


func _on_solo(b: BossSummary, button: Button) -> void:
	button.disabled = true
	var result: ApiResult = await client.start_solo(b)
	if is_instance_valid(button):
		button.disabled = false
	if not result.ok or not visible:
		return
	var encounter: String = client.store.encounter_id()
	var title: String = b.display_title()
	close()
	EventBus.ui_requested.emit(ARENA_SCREEN, {"encounter_id": encounter, "mode": BossSummary.MODE_SOLO, "title": title})


func _on_party() -> void:
	close()
	EventBus.ui_requested.emit(PARTY_SCREEN, {})


func _on_notice(key: String, args: Dictionary) -> void:
	if visible:
		ArenaUi.show_notice(_notice, key, args, "info")


func _unbind() -> void:
	if client != null and client.notice_requested.is_connected(_on_notice):
		client.notice_requested.disconnect(_on_notice)
