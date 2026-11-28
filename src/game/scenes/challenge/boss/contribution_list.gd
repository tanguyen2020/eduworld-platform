class_name ContributionList
extends VBoxContainer
## S06 `ContributionList` / `PartyReadyList` (F110-AC-02, AC-06, AC-09; CHALLENGE_BOSS_SYSTEM §3.5, PARTY_SYSTEM §7.1):
## one chip per member — the nickname with an icon AND words, never a number of another member:
## - lobby (`ready_mode`): ✓ "Sẵn sàng" or the clock "Đang chờ";
## - in a phase: ✓ "Đã đạt phần" (`contribution_met` from the server) or the clock "Đang cố gắng"; the student's own chip
##   adds "{n} câu đúng" (own answers only, FR-PTY-013);
## - a member with `connected = false` is faded with the words "Đang kết nối lại" (F110-AC-09).
## No ranking, no order by score, nothing tells who answered wrong (AGENTS R19). The requirement is said in words
## (`ui.boss.need_contribution`). UI only.

var ready_mode: bool = false
var _chips: HFlowContainer = null
var _need: Label = null
var _rows: Dictionary = {}
var _layout_key: String = ""


func _init() -> void:
	name = "ContributionList"
	_chips = HFlowContainer.new()
	_chips.name = "Chips"
	add_child(_chips)
	# The requirement in words follows the chips in the same flow (one line saved on a phone).
	_need = UiBuild.label("ui.boss.need_contribution", &"HudLabel")
	_need.name = "Need"
	_need.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	# Never squeezed beside the chips: it takes its own line when the chips leave less than this.
	_need.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_280)
	UiBuild.expand(_need)
	_chips.add_child(_need)


## `members` in the server order (never re-sorted); `self_id` = the student; `my_correct` = their own right answers.
## The chips are re-used while the same members are shown in the same mode (a server update only changes words/icons).
func show_members(members: Array[BossMemberModel], self_id: String, my_correct: int, lobby: bool, party: bool) -> void:
	var shown: Array[BossMemberModel] = []
	for m: BossMemberModel in members:
		if party or m.student_id == self_id:
			shown.append(m)
	var ids: PackedStringArray = []
	for m: BossMemberModel in shown:
		ids.append(m.student_id)
	var key: String = "%s|%s" % [lobby, ",".join(ids)]
	if key != _layout_key:
		_layout_key = key
		ready_mode = lobby
		name = "PartyReadyList" if lobby else "ContributionList"
		_chips.remove_child(_need)
		UiBuild.clear(_chips)
		_rows.clear()
		for m: BossMemberModel in shown:
			var chip_node: PanelContainer = _make_chip(m.student_id == self_id and not lobby)
			_chips.add_child(chip_node)
			_rows[m.student_id] = chip_node
		_chips.add_child(_need)
	for m: BossMemberModel in shown:
		_fill(_rows[m.student_id] as PanelContainer, m, m.student_id == self_id, my_correct, lobby)
	_need.visible = not lobby


func _make_chip(with_count: bool) -> PanelContainer:
	var c: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	c.name = "Member"
	c.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var row: HBoxContainer = UiBuild.hbox()
	c.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("clock", 0.7)
	icon.name = "StateIcon"
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	for label_name: String in (["Nickname", "State", "MyCorrect"] if with_count else ["Nickname", "State"]):
		var l: Label = UiBuild.data_label("", &"", false)
		l.name = label_name
		l.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(l)
	return c


func _fill(c: PanelContainer, m: BossMemberModel, me: bool, my_correct: int, lobby: bool) -> void:
	var icon_name: String = "wifi_off"
	var words: String = "ui.boss.member.reconnecting"
	if m.connected:
		var done: bool = m.ready if lobby else m.contribution_met
		icon_name = "verdict_correct" if done else "clock"
		if lobby:
			words = "ui.boss.member.ready" if done else "ui.boss.member.waiting"
		else:
			words = "ui.boss.member.met" if done else "ui.boss.member.trying"
	var text_style: StringName = &"" if m.connected else &"PartyAwayLabel"
	c.theme_type_variation = &"PartyMemberCard" if m.connected else &"PartyMemberCardAway"
	(c.find_child("StateIcon", true, false) as TextureRect).texture = UiBuild.icon(icon_name)
	var nick: Label = c.find_child("Nickname", true, false) as Label
	nick.text = tr("ui.boss.member.me") if me else m.nickname
	nick.theme_type_variation = text_style
	var state: Label = c.find_child("State", true, false) as Label
	state.text = tr(words)
	state.theme_type_variation = text_style
	var count: Label = c.find_child("MyCorrect", true, false) as Label
	if count != null:
		count.text = tr("ui.boss.my_correct").format({"n": LocaleFormat.number(my_correct)})
	c.tooltip_text = "%s · %s" % [nick.text, state.text]


func chip(student_id: String) -> PanelContainer:
	return _rows.get(student_id) as PanelContainer


## Words of the chip of `student_id` (tests).
func chip_texts(student_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var c: PanelContainer = chip(student_id)
	if c != null:
		for node: Node in c.find_children("*", "Label", true, false):
			out.append((node as Label).text)
	return out


func need_visible() -> bool:
	return _need.visible
