class_name MatchHeader
extends PanelContainer
## Top of the battle screen (F109-AC-05, AC-09, AC-10; STUDENT_GAME_UX_SOCIAL §8 `MatchHeader`): the two sides with a
## portrait, the nickname and the server score (team battle: "Đội của em" / "Đội bạn" with the team score and the
## members' nicknames — the member answering this question for each team (server `primary_user_ids`) has a star and
## stronger text, and the student reads "Lượt của em" / "Em hỗ trợ" in words), the
## question progress "Câu 2/3" and the time bar from `deadline_at − server now`. Lớp 1–3 see the bar only (no seconds
## — AC-09); lớp 4–9 also see "12 giây". A player away has a grey card with the wifi sign; the words are in the reconnect banner (not colour only).
## UI only: values are BattleClient's (server).

var _left: VBoxContainer = null
var _right: VBoxContainer = null
var _progress: Label = null
var _bar: ProgressBar = null
var _seconds: Label = null
var _role: Label = null
var _self_id: String = ""
## What the side cards were built for (mode, seats, teams, connections): the same layout only updates the scores.
var _signature: String = ""
## user_id / "team:<n>" → score Label of the cards on screen.
var _scores: Dictionary = {}


func _init() -> void:
	name = "MatchHeader"
	# The light window frame of the arena (dark HUD panels are for the world HUD).
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(row)
	_left = UiBuild.vbox()
	_left.name = "LeftSide"
	UiBuild.expand(_left)
	row.add_child(_left)
	var center: VBoxContainer = UiBuild.vbox()
	center.name = "Center"
	center.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	center.size_flags_stretch_ratio = 1.2
	row.add_child(center)
	_progress = UiBuild.data_label("", &"HeadingLabel")
	_progress.name = "Progress"
	_progress.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	center.add_child(_progress)
	var time_row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	center.add_child(time_row)
	var clock: TextureRect = UiBuild.icon_rect("clock")
	clock.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	time_row.add_child(clock)
	_bar = UiBuild.progress_bar(1.0)
	_bar.name = "TimeBar"
	UiBuild.expand(_bar)
	time_row.add_child(_bar)
	_seconds = UiBuild.data_label("", &"InstructionLabel", false)
	_seconds.name = "Seconds"
	_seconds.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	time_row.add_child(_seconds)
	_role = ArenaUi.tag("", &"TileTagPending")
	_role.name = "Role"
	_role.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_role.visible = false
	center.add_child(_role)
	_right = UiBuild.vbox()
	_right.name = "RightSide"
	UiBuild.expand(_right)
	row.add_child(_right)


## Draws `view` for the student `self_id`.
func render(view: BattleView, self_id: String) -> void:
	_self_id = self_id
	if view == null:
		UiBuild.clear(_left)
		UiBuild.clear(_right)
		_signature = ""
		return
	var sig: String = _signature_of(view, self_id)
	if sig == _signature:
		_update_scores(view)
		_render_progress(view, self_id)
		return
	_signature = sig
	_scores.clear()
	UiBuild.clear(_left)
	UiBuild.clear(_right)
	var teams: Array[int] = view.teams()
	var mine: int = 0
	var me: BattlePlayer = view.player(self_id)
	if me != null:
		mine = me.team
	var other: int = -1
	for t: int in teams:
		if t != mine:
			other = t
			break
	if view.is_team_battle():
		_left.add_child(_team_side(view, mine, true))
		if other >= 0:
			_right.add_child(_team_side(view, other, false))
	else:
		if me != null:
			_left.add_child(_player_side(me, true))
		for p: BattlePlayer in view.participants:
			if p.user_id != self_id:
				_right.add_child(_player_side(p, false))
	_render_progress(view, self_id)


func _render_progress(view: BattleView, self_id: String) -> void:
	_progress.text = TranslationServer.translate("ui.arena.battle.progress").format({
		"n": mini(view.question_index + 1, maxi(view.question_count, 1)), "total": maxi(view.question_count, 1)})
	# Wraps between words only (a long translation), never inside a word.
	_progress.custom_minimum_size.x = ArenaUi.longest_word_width(_progress)
	var primary: bool = self_id in view.primary_user_ids
	_role.visible = view.is_team_battle() and not view.primary_user_ids.is_empty()
	_role.text = "ui.arena.battle.role_primary" if primary else "ui.arena.battle.role_support"
	_role.theme_type_variation = &"TileTagCorrect" if primary else &"TileTagPending"


func _signature_of(view: BattleView, self_id: String) -> String:
	var parts: PackedStringArray = [view.mode, self_id, TranslationServer.get_locale(), ",".join(view.primary_user_ids)]
	for p: BattlePlayer in view.participants:
		parts.append("%s|%d|%s|%s" % [p.user_id, p.team, p.connected, p.score_visible()])
	return ",".join(parts)


func _update_scores(view: BattleView) -> void:
	for key: Variant in _scores.keys():
		var k: String = String(key)
		var value: int = view.team_score(int(k.trim_prefix("team:"))) if k.begins_with("team:") else view.player(k).score
		(_scores[key] as Label).text = _score_text(value)


## Updates the time bar (server clock): bar for every band, seconds in words only for lớp 4–9 (AC-09).
func render_time(view: BattleView, now_ms: int) -> void:
	var on: bool = view != null and view.state == BattleView.STATE_QUESTION and view.deadline_at > 0
	_bar.visible = on
	_seconds.visible = on and ArenaUi.band() != "g1_3"
	if not on:
		return
	var left_ms: int = view.ms_left(now_ms)
	_bar.value = clampf(float(left_ms) / float(view.total_ms()), 0.0, 1.0)
	_seconds.text = TranslationServer.translate("ui.arena.battle.seconds").format({"seconds": ceili(float(left_ms) / 1000.0)})


func seconds_visible() -> bool:
	return _seconds.visible


func seconds_text() -> String:
	return _seconds.text if _seconds.visible else ""


func progress_text() -> String:
	return _progress.text


func role_text() -> String:
	return tr(_role.text) if _role.visible else ""


func side_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in _left.find_children("*", "Label", true, false) + _right.find_children("*", "Label", true, false):
		var l: Label = node as Label
		if l.visible:
			out.append(tr(l.text) if l.auto_translate_mode != Node.AUTO_TRANSLATE_MODE_DISABLED else l.text)
	return out


func _player_side(p: BattlePlayer, mine: bool) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard" if p.connected else &"PartyMemberCardAway")
	card.name = "Me" if mine else "Opponent"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	if not mine:
		row.alignment = BoxContainer.ALIGNMENT_END
	var thumb: AvatarThumb = ArenaUi.portrait(p.avatar_config, 0.7)
	var texts: VBoxContainer = UiBuild.vbox()
	texts.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(texts)
	var nick: String = p.nickname if not p.bot else TranslationServer.translate("ui.arena.battle.robot")
	var name_label: Label = UiBuild.data_label(nick, &"HeadingLabel", false)
	name_label.name = "Nickname"
	# A nickname stays on one line (≤ 32 characters, never cut): its row grows instead (F109-AC-30 edge case).
	name_label.tooltip_text = nick
	name_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT if mine else HORIZONTAL_ALIGNMENT_RIGHT
	texts.add_child(name_label)
	var score: Label = UiBuild.data_label(_score_text(p.score), &"InstructionLabel")
	score.name = "Score"
	score.horizontal_alignment = name_label.horizontal_alignment
	_scores[p.user_id] = score
	texts.add_child(score)
	if mine:
		row.add_child(thumb)
		row.add_child(texts)
	else:
		row.add_child(texts)
		row.add_child(thumb)
	if not p.connected:
		# Grey card + the wifi sign; the words are in the reconnect banner of the screen (the header stays one line).
		var away: TextureRect = UiBuild.icon_rect("wifi_off", 0.8)
		away.name = "Away"
		away.tooltip_text = "ui.arena.battle.reconnecting"
		away.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(away)
		row.move_child(away, 0 if not mine else row.get_child_count() - 1)
	return card


func _team_side(view: BattleView, team: int, mine: bool) -> Control:
	var card: PanelContainer = UiBuild.panel(&"PartyMemberCard")
	card.name = "MyTeam" if mine else "OtherTeam"
	var box: VBoxContainer = UiBuild.vbox()
	card.add_child(box)
	var head: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(head)
	head.add_child(UiBuild.icon_rect("group"))
	var title: Label = UiBuild.label("ui.arena.battle.my_team" if mine else "ui.arena.battle.other_team", &"HeadingLabel")
	UiBuild.expand(title)
	head.add_child(title)
	# The score on its own line: the header stays narrow enough for the chat strip beside it.
	var score: Label = UiBuild.data_label(_score_text(view.team_score(team)), &"InstructionLabel")
	score.name = "TeamScore"
	box.add_child(score)
	_scores["team:%d" % team] = score
	# Each member on its own chip-like item, never cut (keep whole words); the primary of this question has a star.
	var members: HFlowContainer = HFlowContainer.new()
	members.name = "Members"
	box.add_child(members)
	for p: BattlePlayer in view.members(team):
		members.add_child(_member_item(p, p.user_id in view.primary_user_ids))
	# A player away is said in words by the reconnect banner of the screen (the header stays short on a phone).
	return card


## One team member: the nickname, with a star + stronger text when the server named them primary for this question.
func _member_item(p: BattlePlayer, primary: bool) -> HBoxContainer:
	var item: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	item.name = "Primary" if primary else "Member"
	if primary:
		var star: TextureRect = UiBuild.icon_rect("star", 0.8)
		star.name = "PrimaryStar"
		star.tooltip_text = "ui.arena.battle.primary_marker"
		star.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		item.add_child(star)
	var nick: String = p.nickname if not p.bot else TranslationServer.translate("ui.arena.battle.robot")
	var l: Label = UiBuild.data_label(nick, &"InstructionLabel" if primary else &"MutedLabel", false)
	l.name = "Nickname"
	l.tooltip_text = nick
	item.add_child(l)
	return item


## Nicknames the server marked primary for this question (tests / accessibility).
func primary_names() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("Primary*", "HBoxContainer", true, false):
		out.append(((node as Node).get_node("Nickname") as Label).text)
	return out


func _score_text(score: int) -> String:
	if score == BattlePlayer.SCORE_HIDDEN:
		return TranslationServer.translate("ui.arena.battle.score_hidden")
	return TranslationServer.translate("ui.arena.battle.score").format({"score": ArenaUi.num(float(score))})
