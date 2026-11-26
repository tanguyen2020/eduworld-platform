class_name VideoLessonRenderer
extends ActivityRendererBase
## Renderer of `video_lesson` (F065 US2/US3, VIDEO_LEARNING, STUDENT_GAME_UX_LEARNING §5): a Theora variant chosen by
## the quality preset (480p on Low/Web, 720p otherwise; down to 480p after repeated stalls, keeping the position),
## WebVTT subtitles drawn with the Theme caption style (vi/en/off, default = UI language), chapters, a seek bar locked
## to the watched frontier under `watched_only`, embedded questions that pause the video and open the F063 question
## renderer, branch prompts of an interactive video, a resume prompt, and heartbeats through `progress_reported`.
## Display and intents only: completion, watched ratio and grading are the server's (ADR-0016, AGENTS R1/R5).
## F099 (STUDENT_GAME_UX_LEARNING §5): big main button with icon + words ("Phát" / "Tạm dừng" / "Xem lại"), "Lùi 10
## giây", a big "Phát" in the middle of the paused picture (tapping the picture plays / pauses), a MediaTrack with
## chapter notches and question discs (?/✓), the caption card (CaptionCard, speaker written as words), the board look
## for questions and branches, and the coach line through `coach_hint()`.

const SUSPEND_REASON: StringName = &"video"
const VARIANT_LOW: String = "ogv_480"
const VARIANT_HIGH: String = "ogv_720"
## A stall is the position frozen this long while playing; after STALLS_BEFORE_DOWNGRADE stalls 720p → 480p (AC-15).
const STALL_SEC: float = 3.0
const STALLS_BEFORE_DOWNGRADE: int = 2
const TOAST_SEC: float = 3.0
const SEEK_LOCKED_KEY: String = "ui.video.seek_locked"
## F099: "Lùi 10 giây" (STUDENT_GAME_UX_LEARNING §5.1).
const REWIND_MS: int = 10000
const PLAY_KEY: String = "ui.media.play"
const PAUSE_KEY: String = "ui.media.pause"
const REPLAY_KEY: String = "ui.media.replay"
const ICON_PLAY: Texture2D = preload("res://resources/ui/icons/play_light.svg")
const ICON_PAUSE: Texture2D = preload("res://resources/ui/icons/pause_light.svg")
const ICON_REPLAY: Texture2D = preload("res://resources/ui/icons/retry_light.svg")
## F099: coach lines (kind words, no countdown, no comparison — AGENTS R19).
const COACH_START_KEY: String = "ui.video.coach_start"
const COACH_WATCH_KEY: String = "ui.video.coach_watch"
const COACH_WATCH_QUESTIONS_KEY: String = "ui.video.coach_watch_questions"
const COACH_SEEK_LOCKED_KEY: String = "ui.video.coach_seek_locked"
const COACH_DONE_KEY: String = "ui.video.coach_done"
const COACH_BRANCH_KEY: String = "ui.video.coach_branch"

var payload: Dictionary = {}
var cues: Array[Dictionary] = []
var branches: Array[Dictionary] = []
var chapters: Array[Dictionary] = []
var segments: Dictionary = {}
var subtitle_tracks: Array[Dictionary] = []
var policy: String = WatchedRanges.SEEK_WATCHED_ONLY
var pending_cues: PackedStringArray = []
var answered_cues: PackedStringArray = []
var chosen_branches: PackedStringArray = []
var segment_id: String = ""
var variant: String = ""
var downgraded: bool = false
var stalls: int = 0
var ended: bool = false
var duration_ms: int = 0
var subtitle_language: String = ""
var playback: VideoPlayback = null
var reporter: VideoProgressReporter = VideoProgressReporter.new()
var subtitles: VttParser = VttParser.new()
var question: QuestionRendererBase = null
var open_cue: Dictionary = {}
var open_branch: Dictionary = {}
var _subtitle_files: Dictionary = {}
var _stall_sec: float = 0.0
var _last_pos: int = -1
var _toast_sec: float = 0.0
var _shown_at_msec: int = 0
var _suspended: bool = false
var _seek_locked_sec: float = 0.0
var _hint: String = ""


func setup(attempt_model: AttemptModel, ctx: RendererContext) -> void:
	super.setup(attempt_model, ctx)
	if playback == null:
		playback = VideoPlayback.new(%Video as VideoStreamPlayer)
	playback.playback_finished.connect(_on_finished)
	QualityManager.suspend_world(SUSPEND_REASON)
	_suspended = true
	_read_payload()
	apply_progress(attempt.progress)
	_build_controls()
	(%Resume as Control).visible = false
	(%Overlay as Control).visible = false
	(%Branch as Control).visible = false
	if attempt.resume_position_ms > 0 and not attempt.is_finished():
		(%ResumeLabel as Label).text = tr("ui.video.resume_prompt").format({"time": format_time(attempt.resume_position_ms)})
		(%Resume as Control).visible = true
	else:
		await start_at(0)
	_load_subtitles()
	_refresh_controls()


func _read_payload() -> void:
	payload = attempt.render.render_payload if attempt.render != null else {}
	duration_ms = maxi(0, int(payload.get("duration_ms", 0)))
	reporter.heartbeat_sec = clampf(float(payload.get("heartbeat_seconds", VideoProgressReporter.DEFAULT_HEARTBEAT_SEC)), 1.0, 60.0)
	reporter.ranges.duration_ms = duration_ms
	var interaction: Dictionary = payload.get("interaction", {}) as Dictionary if payload.get("interaction") is Dictionary else {}
	policy = String(interaction.get("allow_seek", WatchedRanges.SEEK_WATCHED_ONLY))
	cues = _dicts(payload.get("cues"))
	cues.sort_custom(func(a: Dictionary, b: Dictionary) -> bool: return int(a.get("at_ms", 0)) < int(b.get("at_ms", 0)))
	branches = _dicts(payload.get("branches"))
	chapters = _dicts(payload.get("chapters"))
	for s: Dictionary in _dicts(payload.get("segments")):
		segments[String(s.get("segment_id", ""))] = bool(s.get("is_ending", false))
	subtitle_tracks.clear()
	var media: Array[Dictionary] = attempt.render.media if attempt.render != null else ([] as Array[Dictionary])
	for m: Dictionary in media:
		if String(m.get("kind", "")) == "subtitle":
			var track: Dictionary = m.duplicate()
			for t: Dictionary in _dicts(payload.get("subtitles")):
				if String(t.get("language", "")) == String(m.get("language", "")):
					track["is_default"] = bool(t.get("is_default", false))
			subtitle_tracks.append(track)
	pending_cues.clear()
	for c: Dictionary in cues:
		pending_cues.append(String(c.get("cue_id", "")))


static func _dicts(v: Variant) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if v is Array:
		for e: Variant in (v as Array):
			if e is Dictionary:
				out.append(e as Dictionary)
	return out


## The media entry to play: `segment` ("" = main video), 480p on Low/Web or after a downgrade, else 720p.
static func pick_variant(media: Array[Dictionary], segment: String, preset: String, is_web: bool, low: bool) -> Dictionary:
	var want: String = VARIANT_LOW if low or is_web or preset == QualityPolicy.LOW else VARIANT_HIGH
	var fallback: Dictionary = {}
	for m: Dictionary in media:
		if String(m.get("kind", "")) != "video" or String(m.get("segment_id", "")) != segment:
			continue
		if String(m.get("variant", "")) == want:
			return m
		if fallback.is_empty() or String(m.get("variant", "")) == VARIANT_LOW:
			fallback = m
	return fallback


static func format_time(ms: int) -> String:
	var s: int = maxi(0, ms) / 1000
	return "%02d:%02d" % [s / 60, s % 60]


# ---------- playback ----------

## Coroutine: loads the variant of the current segment and plays from `at_ms`.
func start_at(at_ms: int) -> void:
	(%Resume as Control).visible = false
	if not await _load_variant():
		_toast("ui.activity_player.error_load")
		return
	playback.seek(at_ms)
	playback.play()
	reporter.started(at_ms)
	_last_pos = -1
	_refresh_controls()


func _load_variant() -> bool:
	var media: Array[Dictionary] = attempt.render.media if attempt.render != null else ([] as Array[Dictionary])
	var entry: Dictionary = pick_variant(media, segment_id, QualityManager.preset, OS.has_feature("web"), downgraded)
	if entry.is_empty():
		return false
	variant = String(entry.get("variant", ""))
	var path: String = ""
	if context != null and context.video != null:
		path = await context.video.fetch(String(entry.get("url", "")))
	return playback.load_file(path)


## A heartbeat now (pause, seek, exit).
func _report_now() -> void:
	var r: Dictionary = reporter.paused(playback.position_ms())
	progress_reported.emit(r)


func _process(delta: float) -> void:
	if _toast_sec > 0.0:
		_toast_sec -= delta
		if _toast_sec <= 0.0:
			(%Toast as Control).visible = false
	if _seek_locked_sec > 0.0:
		_seek_locked_sec -= delta
		if _seek_locked_sec <= 0.0:
			_refresh_controls()
	if playback == null or not playback.is_playing():
		return
	var pos: int = playback.position_ms()
	_watch_stall(pos, delta)
	_render_time(pos)
	if not subtitle_language.is_empty():
		var line: String = subtitles.caption_at(pos)
		(%Subtitle as Label).text = line
		(%SubtitlePanel as Control).visible = not line.is_empty()
	var report: Dictionary = reporter.advanced(pos, delta)
	if not report.is_empty():
		progress_reported.emit(report)
	for c: Dictionary in cues:
		var id: String = String(c.get("cue_id", ""))
		if id in pending_cues and not id in answered_cues and bool(c.get("pause", true)) and int(c.get("at_ms", 0)) <= pos:
			open_cue_overlay(c)
			return
	if segment_id.is_empty():
		for b: Dictionary in branches:
			if not String(b.get("branch_point_id", "")) in chosen_branches and int(b.get("at_ms", 0)) <= pos:
				open_branch_prompt(b)
				return


## Stall watch (AC-15): two stalls of more than STALL_SEC on 720p switch to 480p at the same position.
func _watch_stall(pos: int, delta: float) -> void:
	if pos != _last_pos:
		_last_pos = pos
		_stall_sec = 0.0
		return
	_stall_sec += delta
	if _stall_sec < STALL_SEC:
		return
	_stall_sec = 0.0
	stalls += 1
	if stalls >= STALLS_BEFORE_DOWNGRADE and not downgraded and variant != VARIANT_LOW:
		downgraded = true
		start_at(pos)


func _render_time(pos: int) -> void:
	var seek: MediaTrack = %Seek as MediaTrack
	seek.max_value = maxf(1.0, float(duration_ms if duration_ms > 0 else playback.duration_ms()))
	seek.set_value_no_signal(float(pos))
	seek.watched_value = float(reporter.ranges.frontier())
	(%Time as Label).text = "%s / %s" % [format_time(pos), format_time(int(seek.max_value))]
	# F099: the chapter button names the chapter now playing (words next to the time).
	var chapter: int = chapter_at(pos)
	var chapters_btn: OptionButton = %Chapters as OptionButton
	if chapter >= 0 and chapter < chapters_btn.item_count and chapters_btn.selected != chapter:
		chapters_btn.select(chapter)


## Index of the chapter playing at `pos` (-1 without chapters).
func chapter_at(pos: int) -> int:
	var out: int = -1
	for i: int in chapters.size():
		if int(chapters[i].get("start_ms", 0)) <= pos:
			out = i
	return out


## The student drags the seek bar or picks a chapter (AC-17): past the watched frontier is refused under watched_only /
## backward_only until the attempt is finished (toast `ui.video.seek_locked`). Returns the position actually reached.
func request_seek(target_ms: int) -> int:
	var current: int = playback.position_ms()
	var allowed: int = target_ms
	if policy != WatchedRanges.SEEK_FREE and not attempt.is_finished():
		allowed = clampi(target_ms, 0, maxi(reporter.ranges.frontier(), current))
	if allowed != target_ms:
		_toast(SEEK_LOCKED_KEY)
	var was_playing: bool = playback.is_playing()
	_report_now()
	playback.seek(allowed)
	reporter.started(allowed)
	ended = false
	if was_playing:
		playback.play()
	_render_time(allowed)
	_refresh_controls()
	return allowed


func toggle_play() -> void:
	if playback.is_playing():
		playback.pause()
		_report_now()
	elif not (%Overlay as Control).visible and not (%Branch as Control).visible:
		if ended:
			# F099 "Xem lại": back to the start (always allowed — the student only goes backwards).
			request_seek(0)
		playback.play()
		reporter.started(playback.position_ms())
	_refresh_controls()


## F099 "Lùi 10 giây": a backward seek through the usual seek rules.
func rewind() -> void:
	if playback == null:
		return
	request_seek(maxi(0, playback.position_ms() - REWIND_MS))


# ---------- embedded questions (AC-18) ----------

func open_cue_overlay(cue: Dictionary) -> void:
	playback.pause()
	_report_now()
	open_cue = cue
	var item: QuestionRenderModel = attempt.item_at(int(cue.get("position", 0)))
	var content: Control = %OverlayContent as Control
	for child: Node in content.get_children():
		child.queue_free()
	question = QuestionRendererRegistry.instantiate(item.type) if item != null else null
	if question == null:
		var notice: Label = Label.new()
		notice.text = "ui.activity_player.update_required"
		content.add_child(notice)
	else:
		wire_question(question)
		content.add_child(question)
		question.setup(item, context)
		question.answer_changed.connect(func(complete: bool) -> void: answer_changed.emit(complete and not question.locked))
		question.response_prepared.connect(_on_response_prepared)
	(%Overlay as Control).visible = true
	_shown_at_msec = Time.get_ticks_msec()
	item_shown.emit(int(cue.get("position", 0)))
	answer_changed.emit(false)
	_refresh_controls()


func _on_response_prepared(position: int, response: Dictionary) -> void:
	response_submitted.emit({"kind": "cue_answer", "cue_id": String(open_cue.get("cue_id", "")), "position": position,
		"response": response, "client_elapsed_ms": maxi(0, Time.get_ticks_msec() - _shown_at_msec)})


func check() -> bool:
	return question != null and (%Overlay as Control).visible and question.commit()


func show_result(result: ResponseResultModel) -> void:
	if not result.next_segment_id.is_empty():
		_branch_chosen(result.next_segment_id)
		return
	if question != null:
		question.set_locked(true)
		# F099: the graded tiles look like any question (chosen + right answer coloured, the others resting).
		question.show_verdict(result)
	var id: String = String(open_cue.get("cue_id", ""))
	if not id.is_empty() and not id in answered_cues:
		answered_cues.append(id)
	if result.has_verdict and result.is_correct and id in pending_cues:
		pending_cues.remove_at(pending_cues.find(id))


## "Tiếp" after the feedback: the overlay closes and the video plays on; false once the video has ended and nothing
## required is pending (the host submits, AC-21).
func advance() -> bool:
	if (%Overlay as Control).visible:
		(%Overlay as Control).visible = false
		open_cue = {}
		if ended:
			return _after_end()
		playback.play()
		reporter.started(playback.position_ms())
		_refresh_controls()
		return true
	return not ended


func is_last() -> bool:
	return ended and _pending_required().is_empty()


func is_answer_complete() -> bool:
	return question != null and (%Overlay as Control).visible and question.is_complete() and not question.locked


func wants_action_bar() -> bool:
	return (%Overlay as Control).visible


func current_position() -> int:
	return int(open_cue.get("position", 0))


func current_item() -> QuestionRenderModel:
	return attempt.item_at(current_position()) if attempt != null else null


func progress() -> Vector2i:
	return Vector2i(cues.size() - pending_cues.size(), cues.size())


func select_index(i: int) -> void:
	if question != null:
		question.select_index(i)


func set_locked(locked: bool) -> void:
	if question != null:
		question.set_locked(locked)


func _pending_required() -> PackedStringArray:
	var out: PackedStringArray = []
	for c: Dictionary in cues:
		if bool(c.get("required", true)) and String(c.get("cue_id", "")) in pending_cues:
			out.append(String(c.get("cue_id", "")))
	return out


# ---------- end of the video (AC-21) ----------

func _on_finished() -> void:
	ended = true
	reporter.position_ms = maxi(reporter.position_ms, duration_ms)
	progress_reported.emit(reporter.paused(maxi(playback.position_ms(), duration_ms)))
	_refresh_controls()
	if segment_id.is_empty() and not branches.is_empty() and chosen_branches.size() < branches.size():
		return
	_after_end()


## A required cue still pending sends the video back to it; otherwise the attempt can be submitted.
func _after_end() -> bool:
	var missing: PackedStringArray = _pending_required()
	if missing.is_empty():
		finish_requested.emit()
		return false
	for c: Dictionary in cues:
		if String(c.get("cue_id", "")) == missing[0]:
			if missing[0] in answered_cues:
				answered_cues.remove_at(answered_cues.find(missing[0]))
			playback.seek(int(c.get("at_ms", 0)))
			ended = false
			open_cue_overlay(c)
	return true


# ---------- branches (US3, AC-24) ----------

func open_branch_prompt(branch: Dictionary) -> void:
	playback.pause()
	_report_now()
	open_branch = branch
	(%BranchPrompt as Label).text = context.text(branch.get("prompt")) if context != null else ""
	var box: Control = %BranchOptions as Control
	for child: Node in box.get_children():
		child.queue_free()
	var i: int = 0
	for o: Dictionary in _dicts(branch.get("options")):
		var b: Button = Button.new()
		# F099: the choices are answer tiles (colour set + number badge, band size).
		b.theme_type_variation = AnswerBadge.tile_type(i)
		b.add_child(AnswerBadge.new(i + 1, i))
		i += 1
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.text = context.text(o.get("label")) if context != null else String(o.get("option_id", ""))
		b.pressed.connect(choose_branch.bind(String(o.get("option_id", ""))))
		box.add_child(b)
	(%Branch as Control).visible = true
	_refresh_controls()


func choose_branch(option_id: String) -> void:
	if open_branch.is_empty():
		return
	response_submitted.emit({"kind": "branch_choice", "branch_point_id": String(open_branch.get("branch_point_id", "")),
		"option_id": option_id})


func _branch_chosen(next_segment: String) -> void:
	if not open_branch.is_empty():
		chosen_branches.append(String(open_branch.get("branch_point_id", "")))
	open_branch = {}
	(%Branch as Control).visible = false
	_refresh_controls()
	segment_id = next_segment
	ended = false
	await start_at(0)


# ---------- server state ----------

func apply_progress(state: Dictionary) -> void:
	if state.is_empty():
		return
	if state.get("watched_ranges") is Array:
		reporter.ranges.load_from(state["watched_ranges"])
	if state.get("pending_cue_ids") is Array:
		pending_cues.clear()
		for id: Variant in (state["pending_cue_ids"] as Array):
			pending_cues.append(String(id))
	if state.get("segment_id") is String and not (state["segment_id"] as String).is_empty():
		segment_id = state["segment_id"] as String


## VIDEO_SEEK_NOT_ALLOWED (AC-22): back to the watched frontier; other failures (network) change nothing.
func progress_refused(code: String) -> void:
	if code != LearningService.CODE_SEEK_NOT_ALLOWED:
		return
	var edge: int = reporter.ranges.frontier()
	playback.seek(edge)
	reporter.started(edge)
	_toast(SEEK_LOCKED_KEY)


func final_report() -> Dictionary:
	if playback == null:
		return {}
	return reporter.paused(playback.position_ms())


func teardown() -> void:
	if playback != null:
		playback.pause()
	if _suspended:
		QualityManager.resume_world(SUSPEND_REASON)
		_suspended = false


# ---------- subtitles and controls (AC-16) ----------

func _load_subtitles() -> void:
	var lang: String = VttParser.pick_language(subtitle_tracks, TranslationServer.get_locale())
	await set_subtitle_language(lang)


## Coroutine: shows the subtitles of `language` ("" = off).
func set_subtitle_language(language: String) -> void:
	subtitle_language = language
	(%SubtitlePanel as Control).visible = false
	# F099: the caption state is written in words (language code or "Tắt"), never shown by colour only.
	(%Captions as Button).text = tr("ui.video.subtitles") + (" · " + language.to_upper() if not language.is_empty() \
			else " · " + tr("ui.media.off"))
	if language.is_empty():
		return
	var text: String = String(_subtitle_files.get(language, ""))
	if text.is_empty():
		for t: Dictionary in subtitle_tracks:
			if String(t.get("language", "")) == language and context != null and context.media != null:
				text = (await context.media.bytes(String(t.get("url", "")))).get_string_from_utf8()
		_subtitle_files[language] = text
	if not subtitles.parse(text):
		subtitles.cues.clear()


## CC button: next language, then off.
func cycle_subtitles() -> void:
	var langs: PackedStringArray = []
	for t: Dictionary in subtitle_tracks:
		langs.append(String(t.get("language", "")))
	langs.append("")
	var i: int = langs.find(subtitle_language)
	await set_subtitle_language(langs[(i + 1) % langs.size()])


func _build_controls() -> void:
	var chapters_btn: OptionButton = %Chapters as OptionButton
	chapters_btn.clear()
	for ch: Dictionary in chapters:
		chapters_btn.add_item(context.text(ch.get("title")) if context != null else String(ch.get("chapter_id", "")))
	chapters_btn.visible = not chapters.is_empty()
	if not chapters_btn.item_selected.is_connected(_on_chapter_selected):
		chapters_btn.item_selected.connect(_on_chapter_selected)
		(%PlayPause as Button).pressed.connect(toggle_play)
		(%BigPlay as Button).pressed.connect(toggle_play)
		(%Rewind as Button).pressed.connect(rewind)
		(%Captions as Button).pressed.connect(cycle_subtitles)
		(%Seek as MediaTrack).seek_released.connect(func(ms: float) -> void: request_seek(int(ms)))
		(%Screen as Control).gui_input.connect(_on_screen_input)
		(%ResumeYes as Button).pressed.connect(func() -> void: start_at(attempt.resume_position_ms))
		(%ResumeNo as Button).pressed.connect(func() -> void: start_at(0))
	(%Captions as Button).visible = not subtitle_tracks.is_empty()
	var marks: PackedFloat32Array = []
	for ch: Dictionary in chapters:
		marks.append(float(ch.get("start_ms", 0)))
	(%Seek as MediaTrack).chapters = marks


func _on_chapter_selected(index: int) -> void:
	if index >= 0 and index < chapters.size():
		request_seek(int(chapters[index].get("start_ms", 0)))


func _toast(key: String) -> void:
	(%Toast as Label).text = key
	(%Toast as Control).visible = true
	_toast_sec = TOAST_SEC
	if key == SEEK_LOCKED_KEY:
		_seek_locked_sec = TOAST_SEC
		_refresh_controls()


## Tapping the picture plays / pauses (F099).
func _on_screen_input(event: InputEvent) -> void:
	if event is InputEventMouseButton and (event as InputEventMouseButton).pressed \
			and (event as InputEventMouseButton).button_index == MOUSE_BUTTON_LEFT:
		toggle_play()


## F099: the main button, the big play button, the question discs and the coach line follow the player state.
func _refresh_controls() -> void:
	var playing: bool = playback != null and playback.is_playing()
	var blocked: bool = (%Overlay as Control).visible or (%Branch as Control).visible or (%Resume as Control).visible
	var main: Button = %PlayPause as Button
	main.text = PAUSE_KEY if playing else (REPLAY_KEY if ended else PLAY_KEY)
	main.icon = ICON_PAUSE if playing else (ICON_REPLAY if ended else ICON_PLAY)
	# A question, a branch or the resume prompt takes the whole stage: the bar and the buttons step aside.
	(get_node("TrackRow") as Control).visible = not (%Overlay as Control).visible and not (%Branch as Control).visible
	(get_node("Controls") as Control).visible = (get_node("TrackRow") as Control).visible
	main.disabled = blocked
	(%Rewind as Button).disabled = blocked
	var big: Button = %BigPlay as Button
	big.visible = not playing and not blocked
	big.text = REPLAY_KEY if ended else PLAY_KEY
	big.icon = ICON_REPLAY if ended else ICON_PLAY
	var marks: Array[Dictionary] = []
	for c: Dictionary in cues:
		var id: String = String(c.get("cue_id", ""))
		marks.append({"at": float(c.get("at_ms", 0)), "done": id in answered_cues or not id in pending_cues})
	(%Seek as MediaTrack).markers = marks
	var hint: String = coach_hint()
	if hint != _hint:
		_hint = hint
		coach_hint_changed.emit()


func coach_hint() -> String:
	if (%Branch as Control).visible:
		return COACH_BRANCH_KEY
	if (%Overlay as Control).visible or (%Resume as Control).visible:
		return ""
	if _seek_locked_sec > 0.0:
		return COACH_SEEK_LOCKED_KEY
	if ended:
		return COACH_DONE_KEY
	if playback != null and playback.is_playing():
		return COACH_WATCH_QUESTIONS_KEY if not cues.is_empty() else COACH_WATCH_KEY
	return COACH_START_KEY
