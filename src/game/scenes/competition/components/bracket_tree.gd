class_name BracketTree
extends ScrollContainer
## The bracket tree for lớp 6–9 (F109-AC-25; COMPETITION_SYSTEM §10.8): one column per round, one match card per node
## (the two nicknames / team names, the winner marked with a tick **and** the word "Thắng", the status in words). It
## scrolls sideways when the rounds do not fit. Data only from `getTournamentBracket`.

var _columns: HBoxContainer = null


func _init() -> void:
	name = "BracketTree"
	vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	UiBuild.expand(self)
	_columns = UiBuild.hbox(&"ToolbarBox")
	_columns.name = "Rounds"
	add_child(_columns)


func show_bracket(b: BracketModel, self_id: String) -> void:
	UiBuild.clear(_columns)
	if b == null or b.nodes.is_empty():
		_columns.add_child(UiBuild.label("ui.tournament.bracket.empty", &"MutedLabel"))
		return
	var mine: String = b.entry_of_player(self_id)
	var t: float = UiBuild.target()
	custom_minimum_size.y = t * 3.4
	for round_no: int in range(1, b.rounds() + 1):
		var col: VBoxContainer = UiBuild.vbox(&"FieldBox")
		col.name = "Round%d" % round_no
		col.alignment = BoxContainer.ALIGNMENT_CENTER
		col.custom_minimum_size.x = t * 4.2
		var round_tag: Label = UiBuild.data_label(TranslationServer.translate("ui.tournament.bracket.round").format({"round": round_no}),
				&"SectionTag", false)
		round_tag.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
		col.add_child(round_tag)
		for n: Dictionary in b.nodes_of_round(round_no):
			col.add_child(_node_card(b, n, mine))
		_columns.add_child(col)


func _node_card(b: BracketModel, n: Dictionary, mine: String) -> PanelContainer:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "Node"
	card.set_meta(&"bracket_node", true)
	var box: VBoxContainer = UiBuild.vbox()
	card.add_child(box)
	for side: String in ["entry_a", "entry_b"]:
		var id: String = String(n[side])
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		box.add_child(row)
		var won: bool = not id.is_empty() and id == String(n["winner"])
		var name: String = b.name_of(id)
		var label: Label = UiBuild.data_label(name if not name.is_empty() else TranslationServer.translate(
				"ui.tournament.bracket.tbd"), &"HeadingLabel" if id == mine else &"MutedLabel", false)
		# A nickname stays on one line (≤ 32 characters, never cut): its row grows instead (F109-AC-30 edge case).
		label.tooltip_text = label.text
		UiBuild.expand(label)
		row.add_child(label)
		if won:
			row.add_child(UiBuild.icon_rect("tick", 0.8))
			row.add_child(ArenaUi.tag("ui.tournament.bracket.winner", &"TileTagCorrect"))
	box.add_child(UiBuild.label(String(CompetitionText.NODE_STATUS_KEYS.get(String(n["status"]),
			"ui.tournament.node.pending")), &"MutedLabel", false))
	return card


func node_count() -> int:
	var count: int = 0
	for node: Node in _columns.find_children("*", "PanelContainer", true, false):
		if node.has_meta(&"bracket_node"):
			count += 1
	return count
