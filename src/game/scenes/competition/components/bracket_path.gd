class_name BracketPath
extends VBoxContainer
## The simple "con đường" of a bracket for lớp 1–5 (F109-AC-25; COMPETITION_SYSTEM §10.8): only the matches of the
## student's entry, one step per round ("Vòng 1 · gặp CaHeoVui07 · Em đã thắng"), in words with an icon (not colour
## alone). Nicknames / team names only. Without an entry: the first round, so the student sees how it works.


func _init() -> void:
	name = "BracketPath"
	theme_type_variation = &"FieldBox"


func show_bracket(b: BracketModel, self_id: String) -> void:
	UiBuild.clear(self)
	if b == null or b.nodes.is_empty():
		add_child(UiBuild.label("ui.tournament.bracket.empty", &"MutedLabel"))
		return
	var mine: String = b.entry_of_player(self_id)
	var steps: Array[Dictionary] = b.path_of(mine) if not mine.is_empty() else b.nodes_of_round(1)
	for n: Dictionary in steps:
		var card: PanelContainer = UiBuild.panel(&"QuestCard")
		card.name = "Step%d" % int(n["round"])
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		card.add_child(row)
		var done: bool = not String(n["winner"]).is_empty()
		var won: bool = done and String(n["winner"]) == mine
		row.add_child(UiBuild.icon_rect("emoji_trophy" if won else ("flag" if not done else "emoji_muscle"), 1.2))
		var texts: VBoxContainer = UiBuild.vbox()
		UiBuild.expand(texts)
		row.add_child(texts)
		texts.add_child(UiBuild.data_label(TranslationServer.translate("ui.tournament.bracket.round").format({
			"round": int(n["round"])}), &"HeadingLabel"))
		var other: String = String(n["entry_b"]) if String(n["entry_a"]) == mine else String(n["entry_a"])
		var other_name: String = b.name_of(other)
		var line: String = ""
		if mine.is_empty():
			line = "%s · %s" % [_or_wait(b.name_of(String(n["entry_a"]))), _or_wait(b.name_of(String(n["entry_b"])))]
		else:
			line = TranslationServer.translate("ui.tournament.bracket.versus").format({"name": _or_wait(other_name)})
		texts.add_child(UiBuild.data_label(line, &"MutedLabel"))
		var status: String = String(CompetitionText.NODE_STATUS_KEYS.get(String(n["status"]), "ui.tournament.node.pending"))
		if done and not mine.is_empty():
			status = "ui.tournament.bracket.won" if won else "ui.tournament.bracket.done"
		texts.add_child(ArenaUi.tag(status, &"TileTagCorrect" if won else &"TileTagPending"))
		add_child(card)


static func _or_wait(name: String) -> String:
	return name if not name.is_empty() else TranslationServer.translate("ui.tournament.bracket.tbd")


func step_count() -> int:
	return find_children("Step*", "PanelContainer", false, false).size()
