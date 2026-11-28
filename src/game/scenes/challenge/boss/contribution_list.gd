class_name ContributionList
extends VBoxContainer
## S06 `ContributionList` / `PartyReadyList` (F110-AC-02, AC-06, AC-09; CHALLENGE_BOSS_SYSTEM §3.5, PARTY_SYSTEM §7.1):
## one chip per member — the state icon, the nickname in bold and the state in words, never a number of another member:
## - lobby (`ready_mode`): ✓ "Sẵn sàng" or the clock "Đang chờ";
## - in a phase: ✓ "Đã đạt phần" (`contribution_met` from the server) or the clock "Đang cố gắng"; the student's own chip
##   adds "{n} câu đúng" in its own pill (own answers only, FR-PTY-013);
## - a member with `connected = false` is faded with the words "Đang kết nối lại" (F110-AC-09).
## F110 compact polish (F125, STUDENT_GAME_UX_CORE S06 §6b): on the compact phone canvas a chip is one short line — the
## ✓ / clock icon and the nickname (+ the own count pill); the state words stay in the tooltip / accessibility name,
## except "Đang kết nối lại", which is always written. The requirement in words (`ui.boss.need_contribution`, party
## only — never in solo) follows the chips during a phase; on the compact canvas it is said in the party lobby instead,
## where the board is empty, so the question keeps its room during the phase.
## No ranking, no order by score, nothing tells who answered wrong (AGENTS R19). UI only.

var ready_mode: bool = false
var _chips: HFlowContainer = null
var _need: Label = null
var _rows: Dictionary = {}
var _layout_key: String = ""
var _party: bool = false


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
	_party = party
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
	_need.visible = need_shown(lobby, party, AppState.compact)
	_keep_chips_on_one_line()


## Compact: the list asks for the width of its chips on one line (up to `SIZE_LAYOUT_660`), so the message strip
## beside it in the HUD row wraps instead of pushing the chips onto a second line.
func _keep_chips_on_one_line() -> void:
	var width: float = 0.0
	if AppState.compact and is_inside_tree():
		var shown: int = 0
		for c: Variant in _rows.values():
			width += (c as Control).get_combined_minimum_size().x
			shown += 1
		width += float(maxi(shown - 1, 0) * _chips.get_theme_constant(&"h_separation"))
	custom_minimum_size.x = minf(width, float(UiTokens.SIZE_LAYOUT_660))


## Where the requirement in words is shown: party only; during a phase on a big screen, in the lobby on a phone.
static func need_shown(lobby: bool, party: bool, compact: bool) -> bool:
	return party and lobby == compact


func _make_chip(with_count: bool) -> PanelContainer:
	var c: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	c.name = "Member"
	c.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	c.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("clock", 0.7)
	icon.name = "StateIcon"
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	# The nickname in bold, the state in plain words and the own count in its own pill: three parts the eye tells apart.
	var parts: Array = [["Nickname", &"ChatNameLabel"], ["State", &""]]
	if with_count:
		parts.append(["MyCorrect", &"SectionTag"])
	for part: Array in parts:
		var l: Label = UiBuild.data_label("", part[1] as StringName, false)
		l.name = String(part[0])
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
	var faded: bool = not m.connected
	c.theme_type_variation = &"PartyMemberCardAway" if faded else &"PartyMemberCard"
	(c.find_child("StateIcon", true, false) as TextureRect).texture = UiBuild.icon(icon_name)
	var nick: Label = c.find_child("Nickname", true, false) as Label
	nick.text = tr("ui.boss.member.me") if me else m.nickname
	nick.theme_type_variation = &"PartyAwayLabel" if faded else &"ChatNameLabel"
	var state: Label = c.find_child("State", true, false) as Label
	state.text = tr(words)
	state.theme_type_variation = &"PartyAwayLabel" if faded else &""
	# Compact: the icon says ✓ / clock; the words of a reconnecting member are always written (F110-AC-09).
	state.visible = not AppState.compact or faded
	var count: Label = c.find_child("MyCorrect", true, false) as Label
	if count != null:
		count.text = tr("ui.boss.my_correct").format({"n": LocaleFormat.number(my_correct)})
	c.tooltip_text = "%s · %s" % [nick.text, state.text]
	c.accessibility_name = c.tooltip_text


func chip(student_id: String) -> PanelContainer:
	return _rows.get(student_id) as PanelContainer


## Words of the chip of `student_id` (tests; hidden words included — see `shown_texts`).
func chip_texts(student_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var c: PanelContainer = chip(student_id)
	if c != null:
		for node: Node in c.find_children("*", "Label", true, false):
			out.append((node as Label).text)
	return out


## Words of the chip of `student_id` that are on screen (tests of the compact chip).
func shown_texts(student_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var c: PanelContainer = chip(student_id)
	if c != null:
		for node: Node in c.find_children("*", "Label", true, false):
			if (node as Label).visible:
				out.append((node as Label).text)
	return out


func need_visible() -> bool:
	return _need.visible
