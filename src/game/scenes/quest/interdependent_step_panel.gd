class_name InterdependentStepPanel
extends Control
## The `interdependent` step of a party quest (F110 US3; QUEST_SYSTEM §3.2, FR-QST-009): `ui_requested("coop_step",
## {quest_id, step_id})` from the quest journal. The student sees:
## - their own role(s) — the name and icon of the role and its clue card, which only they see until they press
##   "Chia sẻ dữ kiện" (`shareQuestStepClue`, F110-AC-14…16);
## - the other members' roles by nickname + role name only (never their card before it is shared, AC-QST-014);
## - the shared board (SharedClueBoard) — the cards appear for every member when the server says so (≤ 1 s);
## - "Trả lời câu cuối": the student's OWN learning work (`startQuestActivity` → ActivityHost), never for another member
##   (AC-17); "Chat nhóm" only when the step's `share_mode` is `clue_card_and_chat` (party channel, F119), "Chọn vai" for
##   the leader while the server waits for the leader's choice (`leader_assigns`, AC-14);
## - "Em giữ thêm vai {role}" when a member past the grace left a role to this student; the shared cards stay (AC-18).
## The client only shows the cards the server returned (AC-15) — nothing is combined, guessed or stored. UI only.

signal closed()

const PARTY_SCREEN: StringName = &"party"

var services: ServiceLocator = null
var coop: CoopQuestService = null
var quest_id: String = ""
var step_id: String = ""
var _ui: Dictionary = {}
var _notice: Dictionary = {}
var _step_title: Label = null
var _mine: VBoxContainer = null
var _team: VBoxContainer = null
var _board: SharedClueBoard = null
var _final: Button = null
var _chat: Button = null
var _assign: Button = null
var _dialog: RoleAssignDialog = null
var _cards: Dictionary = {}
var _notice_left: float = 0.0


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()
	EventBus.locale_changed.connect(func(_l: String) -> void:
		if visible:
			render())


func _exit_tree() -> void:
	_unbind()


func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	_bind(services.coop_quest if services != null else null)
	quest_id = String(args.get("quest_id", ""))
	step_id = String(args.get("step_id", ""))
	ArenaUi.show_notice(_notice, "")
	show()
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)
	(_ui["close"] as Button).grab_focus.call_deferred(true)
	render()
	if coop != null and not quest_id.is_empty() and not step_id.is_empty():
		await coop.load_clues(quest_id, step_id)


func close() -> void:
	_dialog.dismiss()
	hide()
	_unbind()
	closed.emit()


func render() -> void:
	if coop == null:
		return
	var s: CoopQuestStore = coop.store
	_step_title.text = _title_of_step()
	var mine: PackedStringArray = s.my_roles(quest_id, step_id)
	UiBuild.clear(_mine)
	_cards.clear()
	for role: String in mine:
		var c: ClueCardModel = s.clue(quest_id, step_id, role)
		if c == null:
			_mine.add_child(UiBuild.label("ui.quest.coop.loading_card", &"MutedLabel"))
			continue
		var card: ClueCard = ClueCard.new()
		card.show_card(c, true)
		card.share_requested.connect(_on_share)
		_mine.add_child(card)
		_cards[role] = card
	if mine.is_empty():
		_mine.add_child(UiBuild.keep_words(UiBuild.label("ui.quest.coop.no_role_yet", &"MutedLabel")))
	UiBuild.clear(_team)
	for r: RoleAssignmentModel in s.roles(quest_id, step_id):
		if r.is_me:
			continue
		var name_key: String = L10n.pick(r.title) if not r.title.is_empty() else tr("ui.quest.coop.role_secret")
		var line: String = tr("ui.quest.coop.member_role").format({"nickname": r.nickname, "role": name_key})
		var row: HBoxContainer = UiBuild.hbox()
		var icon: TextureRect = UiBuild.icon_rect("group", 0.8)
		if not r.icon_key.is_empty():
			icon.texture = ClueCard.icon_for(r.icon_key)
		icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(icon)
		var l: Label = UiBuild.keep_words(UiBuild.data_label(line, &"InstructionLabel"))
		l.name = "MemberRole"
		UiBuild.expand(l)
		row.add_child(l)
		_team.add_child(row)
	if _team.get_child_count() == 0:
		_team.add_child(UiBuild.keep_words(UiBuild.label("ui.quest.coop.alone", &"MutedLabel")))
	_board.show_cards(s.clues(quest_id, step_id))
	_chat.visible = coop.share_mode(quest_id, step_id) == CoopQuestService.SHARE_CLUE_CARD_AND_CHAT
	_assign.visible = coop.can_assign(quest_id, step_id)
	UiBuild.size_buttons(self)


# ---------- test hooks ----------

func my_card(role: String) -> ClueCard:
	return _cards.get(role) as ClueCard


func my_card_count() -> int:
	return _cards.size()


func board() -> SharedClueBoard:
	return _board


func team_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for n: Node in _team.find_children("*", "Label", true, false):
		out.append((n as Label).text)
	return out


func final_button() -> Button:
	return _final


func chat_button() -> Button:
	return _chat


func assign_button() -> Button:
	return _assign


func assign_dialog() -> RoleAssignDialog:
	return _dialog


func notice_text() -> String:
	return (_notice["label"] as Label).text if (_notice["panel"] as PanelContainer).visible else ""


func _process(delta: float) -> void:
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			ArenaUi.show_notice(_notice, "")


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


# ---------- binding ----------

func _bind(next: CoopQuestService) -> void:
	_unbind()
	coop = next
	if coop == null:
		return
	for pair: Array in _pairs():
		(pair[0] as Signal).connect(pair[1] as Callable)


func _unbind() -> void:
	if coop == null:
		return
	for pair: Array in _pairs():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	coop = null


func _pairs() -> Array:
	return [[coop.roles_changed, _on_step_changed], [coop.clues_changed, _on_step_changed],
		[coop.extra_role_taken, _on_extra_role], [coop.step_closed, _on_step_closed],
		[coop.notice_requested, _on_notice]]


func _on_step_changed(q: String, s: String) -> void:
	if q == quest_id and s == step_id and visible:
		render()


func _on_extra_role(q: String, s: String, role: String) -> void:
	if q != quest_id or s != step_id:
		return
	var c: ClueCardModel = coop.store.clue(q, s, role)
	var name_text: String = c.display_title() if c != null else tr("ui.quest.coop.role_unnamed")
	_show_notice("ui.quest.clue.extra_role", {"role": name_text})


func _on_step_closed(q: String, s: String) -> void:
	if q == quest_id and s == step_id and visible:
		close()


func _on_notice(key: String, args: Dictionary) -> void:
	if visible:
		_show_notice(key, args)


func _show_notice(key: String, args: Dictionary) -> void:
	ArenaUi.show_notice(_notice, key, args, "info")
	_notice_left = 4.0


func _on_share(role: String) -> void:
	if coop != null:
		await coop.share(quest_id, step_id, role)
		if visible:
			render()


func _on_final() -> void:
	if coop == null:
		return
	_final.disabled = true
	var result: ApiResult = await coop.start_my_activity(quest_id, step_id)
	if is_instance_valid(_final):
		_final.disabled = false
	if result.ok and visible:
		close()


func _on_chat() -> void:
	close()
	EventBus.ui_requested.emit(PARTY_SCREEN, {"chat": true})


func _on_assign() -> void:
	if coop != null:
		_dialog.open_roles(coop.store.roles(quest_id, step_id), coop.role_candidates())


func _on_assign_requested(assignments: Array[Dictionary]) -> void:
	if coop != null:
		await coop.assign_roles(quest_id, step_id, assignments)


func _title_of_step() -> String:
	var q: QuestModel = services.quest.quest(quest_id) if services != null and services.quest != null else null
	if q != null:
		for s: Dictionary in q.steps:
			if String(s.get("step_id", "")) == step_id:
				return L10n.pick(s.get("title", {}) as Dictionary)
		return q.display_name()
	return ""


# ---------- build ----------

func _build() -> void:
	_ui = UiBuild.window(self, "ui.quest.coop.title", "emoji_handshake")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	_notice = ArenaUi.notice()
	body.add_child(_notice["panel"] as PanelContainer)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	body.add_child(scroll)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(box)
	scroll.add_child(box)
	_step_title = UiBuild.keep_words(UiBuild.data_label("", &"HeadingLabel"))
	_step_title.name = "StepTitle"
	box.add_child(_step_title)
	box.add_child(UiBuild.keep_words(UiBuild.label("ui.quest.coop.how", &"MutedLabel")))
	var cols: HFlowContainer = HFlowContainer.new()
	cols.name = "Columns"
	box.add_child(cols)
	var left: VBoxContainer = UiBuild.vbox(&"FormBox")
	left.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_280)
	UiBuild.expand(left)
	cols.add_child(left)
	left.add_child(FriendsUi.group_title("ui.quest.coop.my_role", "star"))
	_mine = UiBuild.vbox(&"FormBox")
	_mine.name = "MyRoles"
	left.add_child(_mine)
	left.add_child(FriendsUi.group_title("ui.quest.coop.team", "group"))
	_team = UiBuild.vbox()
	_team.name = "Team"
	left.add_child(_team)
	_board = SharedClueBoard.new()
	_board.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_280)
	UiBuild.expand(_board)
	cols.add_child(_board)
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	body.add_child(actions)
	_final = UiBuild.button("ui.quest.clue.answer_final", "play_light")
	_final.name = "AnswerFinal"
	_final.pressed.connect(_on_final)
	actions.add_child(_final)
	_chat = UiBuild.button("ui.quest.coop.chat", "chat", &"SecondaryButton")
	_chat.name = "Chat"
	_chat.visible = false
	_chat.pressed.connect(_on_chat)
	actions.add_child(_chat)
	_assign = UiBuild.button("ui.quest.coop.assign_roles", "crown", &"SecondaryButton")
	_assign.name = "AssignRoles"
	_assign.visible = false
	_assign.pressed.connect(_on_assign)
	actions.add_child(_assign)
	_dialog = RoleAssignDialog.new()
	_dialog.name = "RoleAssignDialog"
	_dialog.assign_requested.connect(_on_assign_requested)
	add_child(_dialog)
	UiBuild.size_buttons(self)
