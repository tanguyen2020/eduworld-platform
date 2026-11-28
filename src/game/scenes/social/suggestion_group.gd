class_name SuggestionGroup
extends VBoxContainer
## "Vừa chơi cùng" of the "Tìm bạn" tab (F111-AC-06…08; STUDENT_GAME_UX_SOCIAL §6, ADR-0095): the players this student
## played with recently (`listFriendSuggestions`), each with the 2D portrait, nickname, "Cấp n" and the context of the last
## game together as an icon AND words (party / match / club / co-op) — never a real name, a school or a grade (AGENTS
## R16) — and "Kết bạn" ("Đang chờ" once a request was sent). Lớp 1–3 (`big`): a row of big portrait tiles, tapping the
## portrait sends the request. An empty list shows Mây with the friend-code hint; a failing list hides the whole group
## (the other groups stay). UI only: the friends screen calls SocialService (AGENTS R1).

signal add_requested(user_id: String)

## Portrait side of a lớp 1–3 tile in touch-target units.
const BIG_PORTRAIT: float = 1.5

var _list: Container = null
var _empty: Control = null
var _hint: Label = null
var _cards: Dictionary = {}
var _adds: Dictionary = {}
var _portraits: Dictionary = {}


func _init() -> void:
	name = "SuggestionGroup"
	theme_type_variation = &"FormBox"
	add_child(FriendsUi.group_title("ui.friends.suggest.title", "history"))
	_hint = UiBuild.keep_words(UiBuild.label("ui.friends.suggest.tap_hint", &"MutedLabel"))
	_hint.name = "TapHint"
	add_child(_hint)
	_empty = FriendsUi.empty_line("ui.friends.suggest.empty", "Empty")
	add_child(_empty)


## `pending`: ids with a request this student sent; `failed`: hide the group (F111-AC-08); `big`: lớp 1–3 tiles.
func set_suggestions(list: Array[FriendSuggestionModel], pending: Dictionary, failed: bool, big: bool) -> void:
	visible = not failed
	if _list != null:
		remove_child(_list)
		_list.queue_free()
	_cards.clear()
	_adds.clear()
	_portraits.clear()
	if big:
		var flow: HFlowContainer = HFlowContainer.new()
		_list = flow
	else:
		_list = UiBuild.vbox(&"FormBox")
	_list.name = "List"
	add_child(_list)
	move_child(_list, _empty.get_index())
	for s: FriendSuggestionModel in list:
		var card: Control = _tile(s, pending.has(s.user_id())) if big else _row(s, pending.has(s.user_id()))
		_cards[s.user_id()] = card
		_list.add_child(card)
	_hint.visible = big and not list.is_empty()
	_empty.visible = list.is_empty()
	UiBuild.size_buttons(self)
	for b: Button in _portraits.values():
		var side: float = UiBuild.target() * BIG_PORTRAIT
		b.custom_minimum_size = Vector2(side, side)


func count() -> int:
	return _cards.size()


func card(user_id: String) -> Control:
	return _cards.get(user_id) as Control


func add_button(user_id: String) -> Button:
	return _adds.get(user_id) as Button


func portrait_button(user_id: String) -> Button:
	return _portraits.get(user_id) as Button


func empty_visible() -> bool:
	return _empty.is_visible_in_tree()


func empty_text() -> String:
	var labels: Array[Node] = _empty.find_children("*", "Label", true, false)
	return tr((labels[0] as Label).text) if not labels.is_empty() else ""


## Every text of one card (tests: nickname, level, context words — nothing else).
func card_texts(user_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var c: Control = card(user_id)
	if c == null:
		return out
	for node: Node in c.find_children("*", "Label", true, false):
		if (node as Label).visible:
			out.append(tr((node as Label).text))
	return out


func _row(s: FriendSuggestionModel, pending: bool) -> Control:
	var ui: Dictionary = FriendsUi.person_card(s.player.nickname, s.player.avatar_config, "Suggestion")
	var texts: VBoxContainer = (ui["card"] as Control).find_child("Nickname", true, false).get_parent() as VBoxContainer
	_add_details(texts, s)
	var actions: HFlowContainer = ui["actions"] as HFlowContainer
	if pending:
		actions.add_child(FriendsUi.status("ui.friends.request_pending", "clock"))
	else:
		var add: Button = UiBuild.button("ui.friends.add", "invite_light")
		add.name = "Add"
		add.pressed.connect(_on_add.bind(s.user_id(), add))
		actions.add_child(add)
		_adds[s.user_id()] = add
	return ui["card"] as Control


func _tile(s: FriendSuggestionModel, pending: bool) -> Control:
	var tile: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	tile.name = "Suggestion"
	var box: VBoxContainer = UiBuild.vbox()
	box.alignment = BoxContainer.ALIGNMENT_CENTER
	tile.add_child(box)
	var portrait: Button = Button.new()
	portrait.name = "Portrait"
	portrait.theme_type_variation = &"SecondaryButton"
	portrait.tooltip_text = s.player.nickname
	portrait.accessibility_name = s.player.nickname
	portrait.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	portrait.disabled = pending
	var thumb: AvatarThumb = AvatarThumb.new()
	thumb.standalone = true
	thumb.mouse_filter = Control.MOUSE_FILTER_IGNORE
	thumb.set_anchors_preset(Control.PRESET_FULL_RECT)
	var look: AvatarConfigModel = s.player.avatar()
	thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
	portrait.add_child(thumb)
	portrait.pressed.connect(_on_add.bind(s.user_id(), portrait))
	box.add_child(portrait)
	_portraits[s.user_id()] = portrait
	var nick: Label = UiBuild.data_label(s.player.nickname, &"HeadingLabel", false)
	nick.name = "Nickname"
	nick.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(nick)
	_add_details(box, s)
	if pending:
		box.add_child(FriendsUi.status("ui.friends.request_pending", "clock"))
	return tile


func _add_details(box: Container, s: FriendSuggestionModel) -> void:
	if s.player.level > 0:
		var level: Label = UiBuild.data_label(tr("ui.friends.level").format({"level": LocaleFormat.number(s.player.level)}),
				&"MutedLabel", false)
		level.name = "Level"
		box.add_child(level)
	if not s.context_kind.is_empty():
		var ctx: HBoxContainer = FriendsUi.status(s.context_key(), s.context_icon())
		ctx.name = "Context"
		box.add_child(ctx)


func _on_add(user_id: String, b: Button) -> void:
	b.disabled = true
	add_requested.emit(user_id)
