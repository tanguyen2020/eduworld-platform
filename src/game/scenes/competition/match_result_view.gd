class_name MatchResultView
extends VBoxContainer
## The result screen of a match (F109 US3; STUDENT_GAME_UX_SOCIAL §8 `ResultScreen`, COMPETITION_SYSTEM §3–§4, §12):
## - completed: Mây and the outcome in encouraging words ("Em thắng rồi!", "Hòa — cả hai đều giỏi!", "Chưa thắng lần
##   này — em đã làm rất tốt!"), the student's score and right answers, the Competition Points the server gave and the
##   tier reloaded from `getMyRatings` (never a rating number — AC-15); team battle: the student's team outcome; Timed
##   Quiz: lớp 1–3 the grade goal (GradeGoalBar, no rank), lớp 4–9 "Em xếp thứ 3" + the top nicknames the server sent;
## - the server `encouragement` (§12, sent to this student only after a losing streak): a second line in Mây's bubble
##   "Em đã rất cố gắng! Mình cùng ôn lại chủ đề {topic} môn {subject}…" (`ui.competition.encouragement.<code>`, the
##   names from MatchmakingService; no count of losses, no rank — AC-16);
## - the questions to review (from `getMatch.review`, after the match: ✓ / ✗ in words, the explanation);
## - cancelled / aborted: "Trận đã hủy, không tính kết quả" (AC-14);
## - "Báo cáo người chơi" / "Chặn" per opponent (F119 ChatPlayerSafety + the report sheet; never for the robot);
## - [Chơi lại] [Về Đấu trường].
## UI only.

signal play_again_requested()
signal back_requested()

var self_id: String = ""
## Names of the encouragement's subject / topic (set by the battle screen; null = the generic words).
var matchmaking: MatchmakingService = null
var _summary: MatchSummary = null
var _encouragement: Label = null
var _coach_slot: VBoxContainer = null
var _stats: VBoxContainer = null
var _tier: TierBadge = null
var _review: VBoxContainer = null
var _safety_box: VBoxContainer = null
var _safety: ChatPlayerSafety = null
var _again: Button = null
var _back: Button = null
var _sheet: ChatSheet = null
var _dialogs: ChatDialogs = null
var _outcome_key: String = ""


func _init() -> void:
	name = "MatchResult"
	theme_type_variation = &"FormBox"
	UiBuild.expand(self, true, true)
	var sb: Dictionary = ArenaUi.scroll_box()
	add_child(sb["scroll"] as ScrollContainer)
	var box: VBoxContainer = sb["box"] as VBoxContainer
	_coach_slot = UiBuild.vbox()
	_coach_slot.name = "Outcome"
	box.add_child(_coach_slot)
	_stats = UiBuild.vbox(&"FieldBox")
	_stats.name = "Stats"
	box.add_child(_stats)
	_tier = TierBadge.new()
	box.add_child(_tier)
	_review = UiBuild.vbox(&"FieldBox")
	_review.name = "Review"
	box.add_child(_review)
	_safety_box = UiBuild.vbox(&"FieldBox")
	_safety_box.name = "SafetyBox"
	box.add_child(_safety_box)
	var buttons: HFlowContainer = HFlowContainer.new()
	buttons.name = "Buttons"
	buttons.alignment = FlowContainer.ALIGNMENT_END
	add_child(buttons)
	_back = UiBuild.button("ui.arena.result.back", "arena", &"SecondaryButton")
	_back.name = "Back"
	_back.pressed.connect(back_requested.emit)
	buttons.add_child(_back)
	_again = UiBuild.button("ui.arena.result.again", "retry_light")
	_again.name = "Again"
	_again.pressed.connect(play_again_requested.emit)
	buttons.add_child(_again)
	_sheet = ChatSheet.new()
	add_child(_sheet)
	_dialogs = ChatDialogs.new(_sheet)


func bind_safety(chat: ChatService, social: SocialService, reports: ReportService) -> void:
	_dialogs.bind(chat, social, reports)


## Draws the end of the match: `summary` (server), `view` (nicknames of the snapshot), `again` = "Chơi lại" offered.
func show_result(summary: MatchSummary, view: BattleView, again: bool) -> void:
	UiBuild.clear(_coach_slot)
	UiBuild.clear(_stats)
	UiBuild.clear(_review)
	UiBuild.clear(_safety_box)
	_safety = null
	_tier.show_tier(null)
	_summary = summary
	_encouragement = null
	_again.visible = again and not summary.is_void()
	if summary.is_void():
		_outcome_key = "ui.arena.result.void"
		_coach_slot.add_child(ArenaUi.coach_line(_outcome_key, {}, CloudyMascot.Mood.ENCOURAGE))
		UiBuild.size_buttons(self)
		return
	var mine: Dictionary = summary.result_of(self_id)
	var outcome: String = String(mine.get("outcome", ""))
	var quiz: bool = view != null and view.mode == "timed_quiz"
	_outcome_key = "ui.arena.result.quiz_done" if quiz else CompetitionText.outcome_key(outcome)
	var encourage: bool = not summary.encouragement.is_empty()
	var coach: HBoxContainer = ArenaUi.coach_line(_outcome_key, {"topic": TranslationServer.translate("ui.arena.result.topic_any")},
			CloudyMascot.Mood.HAPPY if outcome != "loss" and not encourage else CloudyMascot.Mood.ENCOURAGE)
	_coach_slot.add_child(coach)
	if encourage:
		# The same bubble of Mây says it (one mascot on the screen): a kind line + what to review.
		var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		row.name = "EncouragementRow"
		var book: TextureRect = UiBuild.icon_rect("emoji_book")
		book.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		row.add_child(book)
		_encouragement = UiBuild.keep_words(UiBuild.data_label("", &"SpeechBubbleLabel"))
		_encouragement.name = "Encouragement"
		UiBuild.expand(_encouragement)
		row.add_child(_encouragement)
		(coach.find_child("CoachLine", true, false) as Label).get_parent().add_child(row)
		refresh_encouragement()
	if not mine.is_empty():
		_stats.add_child(_stat("star", TranslationServer.translate("ui.arena.result.score").format({
			"score": ArenaUi.num(float(int(mine["score"]))), "correct": int(mine["correct_count"]),
			"total": maxi(view.question_count if view != null else 0, int(mine["correct_count"]))})))
		if int(mine["competition_points"]) > 0:
			_stats.add_child(_stat("medal", TranslationServer.translate("ui.arena.result.points").format({
				"points": ArenaUi.num(float(int(mine["competition_points"])))})))
	if not summary.grade_goal.is_empty():
		var bar: GradeGoalBar = GradeGoalBar.new()
		_stats.add_child(bar)
		bar.show_goal(summary.grade_goal)
	elif summary.my_rank > 0:
		_stats.add_child(_stat("medal", TranslationServer.translate("ui.arena.result.quiz_rank").format({"rank": summary.my_rank})))
		var names: PackedStringArray = []
		for t: Dictionary in summary.top:
			var p: BattlePlayer = view.player(String(t["user_id"])) if view != null else null
			if p != null:
				names.append("%d. %s" % [int(t["rank"]), p.nickname])
		if not names.is_empty():
			_stats.add_child(UiBuild.data_label(TranslationServer.translate("ui.arena.result.quiz_top").format({
				"names": ", ".join(names)}), &"MutedLabel"))
	if view != null:
		var others: Array[Dictionary] = []
		for p: BattlePlayer in view.participants:
			if p.user_id != self_id and not p.bot:
				others.append(p.to_card())
		if not others.is_empty():
			_safety_box.add_child(ArenaUi.section("ui.arena.safety.title"))
			_safety = ChatPlayerSafety.new()
			_safety.set_players(others)
			_dialogs.context = {"match_id": summary.match_id}
			_safety.report_requested.connect(func(id: String, nick: String) -> void:
				_dialogs.open_report(ReportService.TARGET_USER, id, nick, id))
			_safety.block_requested.connect(func(id: String, nick: String) -> void:
				_dialogs.relation(ChatDialogs.ACT_BLOCK, id, nick))
			_safety_box.add_child(_safety)
	UiBuild.size_buttons(self)


## Draws the encouragement line again (the subject / topic names arrived — MatchmakingService read them).
func refresh_encouragement() -> void:
	if _encouragement == null or _summary == null:
		return
	var enc: Dictionary = _summary.encouragement
	var subject_id: String = String(enc.get("subject_id", ""))
	var topic_id: String = String(enc.get("topic_id", ""))
	var subject: String = matchmaking.subject_name(subject_id) if matchmaking != null \
			else TranslationServer.translate("ui.arena.subject.unknown")
	var topic: String = matchmaking.topic_name(subject_id, topic_id) if matchmaking != null and not topic_id.is_empty() else ""
	_encouragement.text = TranslationServer.translate(CompetitionText.encouragement_key(enc, not topic.is_empty())).format(
			{"subject": subject, "topic": topic})
	UiBuild.fit_words(_encouragement)


func encouragement_text() -> String:
	return _encouragement.text if _encouragement != null else ""


## The tier after `getMyRatings` (the tier of this mode / subject, or none yet).
func show_tier(tier: TierView) -> void:
	_tier.show_tier(tier)


## The questions to review (`getMatch.review`): the wrong ones first ("Câu cần ôn").
func show_review(record: MatchRecord) -> void:
	UiBuild.clear(_review)
	if record == null or record.review.is_empty():
		return
	_review.add_child(ArenaUi.section("ui.arena.review.to_practice"))
	var rows: Array[Dictionary] = record.review.duplicate()
	rows.sort_custom(func(a: Dictionary, b: Dictionary) -> bool:
		var wa: bool = a.get("my_answer_correct") is bool and bool(a["my_answer_correct"])
		var wb: bool = b.get("my_answer_correct") is bool and bool(b["my_answer_correct"])
		if wa != wb:
			return not wa
		return int(a["index"]) < int(b["index"]))
	for r: Dictionary in rows:
		_review.add_child(MatchHistoryView.review_row(r))


func outcome_key() -> String:
	return _outcome_key


func safety() -> ChatPlayerSafety:
	return _safety


func dialogs() -> ChatDialogs:
	return _dialogs


func tier_text() -> String:
	return _tier.text()


func again_button() -> Button:
	return _again


func back_button() -> Button:
	return _back


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		var l: Label = node as Label
		if l.is_visible_in_tree() or not is_inside_tree():
			out.append(l.tr(l.text) if l.can_auto_translate() else l.text)
	return out


func _stat(icon: String, text: String) -> HBoxContainer:
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.add_child(UiBuild.icon_rect(icon))
	var l: Label = UiBuild.data_label(text, &"HeadingLabel")
	UiBuild.expand(l)
	row.add_child(l)
	return row
