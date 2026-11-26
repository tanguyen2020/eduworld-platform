class_name AudioLessonRenderer
extends ActivityRendererBase
## Renderer of `audio_lesson` (F066 US3, AUDIO_LANGUAGE_LEARNING, STUDENT_GAME_UX_LEARNING §6): an Ogg Vorbis file on
## the Voice bus with the world music ducked, "Lượt nghe: n/max", 0.75×/1× when allowed, the transcript as the
## version allows it, questions at their `at_ms` (the audio pauses) or after the end through the F063 question
## renderers, heartbeats through `progress_reported`. A new play is asked to the server first (`play_started`) and
## starts only once accepted; at the limit the play button greys out and answering stays possible (F066-AC-19). No
## recording node, no microphone (F066-AC-20). Display and intents only: plays, completion and grading are the server's.

const SUSPEND_REASON: StringName = &"audio"
const AUDIO_VARIANT: String = "ogg"
const TOAST_SEC: float = 3.0
const SPEEDS: Array[float] = [1.0, 0.75]

var payload: Dictionary = {}
## Questions `{cue_id, position, at_ms?}` by position (timed ones first, then those asked after the end).
var questions: Array[Dictionary] = []
var answered: PackedInt32Array = []
var duration_ms: int = 0
var speed_allowed: bool = true
var transcript_mode: String = "after_complete"
var playback: AudioPlayback = null
var plays: AudioPlayTracker = AudioPlayTracker.new()
var reporter: VideoProgressReporter = VideoProgressReporter.new()
var question: QuestionRendererBase = null
var open_question: Dictionary = {}
var transcript_text: String = ""
var heard_once: bool = false
var _speed_index: int = 0
var _toast_sec: float = 0.0
var _shown_at_msec: int = 0
var _suspended: bool = false


func setup(attempt_model: AttemptModel, ctx: RendererContext) -> void:
	super.setup(attempt_model, ctx)
	if playback == null:
		playback = AudioPlayback.new(%Player as AudioStreamPlayer)
	playback.playback_finished.connect(_on_finished)
	QualityManager.suspend_world(SUSPEND_REASON)
	AudioDirector.duck_music(SUSPEND_REASON, true)
	_suspended = true
	_read_payload()
	apply_progress(attempt.progress)
	for p: Variant in attempt.saved.keys():
		answered.append(int(p))
	var resume: int = attempt.resume_position_ms
	if resume > 0 and plays.plays_used > 0 and not attempt.is_finished():
		plays.ended = false
		playback.seek(resume)
		reporter.started(resume)
	(%Overlay as Control).visible = false
	(%TranscriptPanel as Control).visible = false
	_bind_controls()
	_refresh()
	await _load_audio()


func _read_payload() -> void:
	payload = attempt.render.render_payload if attempt.render != null else {}
	duration_ms = maxi(0, int(payload.get("duration_ms", 0)))
	reporter.heartbeat_sec = clampf(float(payload.get("heartbeat_seconds", VideoProgressReporter.DEFAULT_HEARTBEAT_SEC)), 1.0, 60.0)
	reporter.ranges.duration_ms = duration_ms
	var interaction: Dictionary = payload.get("interaction", {}) as Dictionary if payload.get("interaction") is Dictionary else {}
	speed_allowed = bool(interaction.get("allow_speed_change", true))
	transcript_mode = String(interaction.get("show_transcript", "after_complete"))
	plays.configure(interaction.get("max_plays"), int(payload.get("plays_used", 0)))
	questions = VideoLessonRenderer._dicts(payload.get("questions"))
	questions.sort_custom(func(a: Dictionary, b: Dictionary) -> bool: return int(a.get("position", 0)) < int(b.get("position", 0)))


## The `audio` media entry of the render (Ogg Vorbis), {} when absent.
static func audio_entry(media: Array[Dictionary]) -> Dictionary:
	for m: Dictionary in media:
		if String(m.get("kind", "")) == "audio" and String(m.get("variant", "")) == AUDIO_VARIANT:
			return m
	return {}


static func transcript_entry(media: Array[Dictionary]) -> Dictionary:
	for m: Dictionary in media:
		if String(m.get("kind", "")) == "transcript":
			return m
	return {}


func _media() -> Array[Dictionary]:
	return attempt.render.media if attempt.render != null else ([] as Array[Dictionary])


## Coroutine: downloads the Ogg file once (cache by sha256) and loads it.
func _load_audio() -> bool:
	var entry: Dictionary = audio_entry(_media())
	if entry.is_empty():
		_toast("ui.audio.not_ready")
		return false
	var path: String = ""
	if context != null and context.video != null:
		path = await context.video.fetch(String(entry.get("url", "")))
	if not playback.load_file(path):
		_toast("ui.audio.not_ready")
		return false
	if duration_ms <= 0:
		duration_ms = playback.duration_ms()
		reporter.ranges.duration_ms = duration_ms
	return true


# ---------- plays (AC-18, AC-19) ----------

## The play button: pauses a playing audio, resumes the current play, or asks the server for a new play.
func press_play() -> void:
	if playback.is_playing():
		playback.pause()
		progress_reported.emit(reporter.paused(playback.position_ms()))
		_refresh()
		return
	if (%Overlay as Control).visible:
		return
	if not plays.needs_new_play(playback.position_ms()):
		playback.play()
		reporter.started(playback.position_ms())
		_refresh()
		return
	if plays.exhausted:
		_toast("ui.audio.plays_exhausted")
		_refresh()
		return
	if not plays.request():
		return
	var r: Dictionary = reporter.paused(playback.position_ms())
	r["position_ms"] = 0
	r["play_started"] = true
	progress_reported.emit(r)
	_refresh()


func apply_progress(state: Dictionary) -> void:
	if state.is_empty():
		return
	if state.get("watched_ranges") is Array:
		reporter.ranges.load_from(state["watched_ranges"])
	if plays.accepted(state):
		playback.seek(0)
		playback.play()
		reporter.started(0)
	_refresh()


## `AUDIO_PLAY_LIMIT_REACHED` (or a failed request): the play does not start; answering stays possible.
func progress_refused(code: String) -> void:
	var was_waiting: bool = plays.awaiting
	plays.refused(code)
	if was_waiting and plays.exhausted:
		_toast("ui.audio.plays_exhausted")
	_refresh()


func toggle_speed() -> void:
	if not speed_allowed:
		return
	_speed_index = (_speed_index + 1) % SPEEDS.size()
	reporter.playback_rate = SPEEDS[_speed_index]
	playback.set_speed(SPEEDS[_speed_index])
	_refresh()


func _process(delta: float) -> void:
	if _toast_sec > 0.0:
		_toast_sec -= delta
		if _toast_sec <= 0.0:
			(%Toast as Control).visible = false
	if playback == null or not playback.is_playing():
		return
	var pos: int = playback.position_ms()
	_render_time(pos)
	var report: Dictionary = reporter.advanced(pos, delta)
	if not report.is_empty():
		progress_reported.emit(report)
	for q: Dictionary in questions:
		if q.has("at_ms") and not int(q.get("position", 0)) in answered and int(q.get("at_ms", 0)) <= pos:
			open_question_overlay(q)
			return


func _on_finished() -> void:
	plays.finished()
	heard_once = true
	reporter.position_ms = maxi(reporter.position_ms, duration_ms)
	progress_reported.emit(reporter.paused(maxi(playback.position_ms(), duration_ms)))
	playback.seek(0)
	_refresh()
	_next_question_or_finish()


## After the end: the next unanswered question (timed ones the student skipped by seeking, then the ones after the
## end); none left → the attempt can be submitted.
func _next_question_or_finish() -> bool:
	for q: Dictionary in questions:
		if not int(q.get("position", 0)) in answered:
			open_question_overlay(q)
			return true
	finish_requested.emit()
	return false


# ---------- questions ----------

func open_question_overlay(q: Dictionary) -> void:
	if playback.is_playing():
		playback.pause()
		progress_reported.emit(reporter.paused(playback.position_ms()))
	open_question = q
	var position: int = int(q.get("position", 0))
	var item: QuestionRenderModel = attempt.item_at(position)
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
	item_shown.emit(position)
	answer_changed.emit(false)
	_refresh()


func _on_response_prepared(position: int, response: Dictionary) -> void:
	response_submitted.emit({"kind": "question", "position": position, "response": response,
		"client_elapsed_ms": maxi(0, Time.get_ticks_msec() - _shown_at_msec)})


func check() -> bool:
	return question != null and (%Overlay as Control).visible and question.commit()


func show_result(_result: ResponseResultModel) -> void:
	if question != null:
		question.set_locked(true)
	var position: int = int(open_question.get("position", 0))
	if position > 0 and not position in answered:
		answered.append(position)


## "Tiếp" after the feedback: the overlay closes; the audio plays on, or the next question after the end opens; false
## when nothing is left (the host submits).
func advance() -> bool:
	if not (%Overlay as Control).visible:
		return heard_once and _unanswered() > 0
	(%Overlay as Control).visible = false
	open_question = {}
	_refresh()
	if heard_once:
		return _next_question_or_finish()
	if not plays.ended:
		playback.play()
		reporter.started(playback.position_ms())
	return true


func _unanswered() -> int:
	var n: int = 0
	for q: Dictionary in questions:
		if not int(q.get("position", 0)) in answered:
			n += 1
	return n


func is_last() -> bool:
	return heard_once and _unanswered() == 0


func is_answer_complete() -> bool:
	return question != null and (%Overlay as Control).visible and question.is_complete() and not question.locked


func wants_action_bar() -> bool:
	return (%Overlay as Control).visible or (heard_once and _unanswered() == 0)


func current_position() -> int:
	return int(open_question.get("position", 0))


func current_item() -> QuestionRenderModel:
	return attempt.item_at(current_position()) if attempt != null else null


func progress() -> Vector2i:
	return Vector2i(questions.size() - _unanswered(), questions.size())


func select_index(i: int) -> void:
	if question != null:
		question.select_index(i)


func set_locked(locked: bool) -> void:
	if question != null:
		question.set_locked(locked)


func final_report() -> Dictionary:
	if playback == null:
		return {}
	return reporter.paused(playback.position_ms())


func teardown() -> void:
	if playback != null:
		playback.pause()
	if _suspended:
		QualityManager.resume_world(SUSPEND_REASON)
		AudioDirector.duck_music(SUSPEND_REASON, false)
		_suspended = false


# ---------- transcript ----------

## Whether the transcript can be shown now (`always`, or `after_complete` once the server served it).
func transcript_available() -> bool:
	return transcript_mode != "never" and not transcript_entry(_media()).is_empty()


## Coroutine: shows or hides the transcript text.
func toggle_transcript() -> void:
	var panel: Control = %TranscriptPanel as Control
	if panel.visible:
		panel.visible = false
		return
	if not transcript_available():
		return
	if transcript_text.is_empty() and context != null and context.media != null:
		transcript_text = (await context.media.bytes(String(transcript_entry(_media()).get("url", "")))).get_string_from_utf8()
	(%TranscriptText as Label).text = transcript_text
	panel.visible = true


# ---------- controls ----------

func _bind_controls() -> void:
	var play: Button = %Play as Button
	if not play.pressed.is_connected(press_play):
		play.pressed.connect(press_play)
		(%Speed as Button).pressed.connect(toggle_speed)
		(%Transcript as Button).pressed.connect(toggle_transcript)
		(%Seek as HSlider).drag_ended.connect(func(_changed: bool) -> void: _seek_to(int((%Seek as HSlider).value)))


## Seeking inside the current play is free (no plays counted, research R1).
func _seek_to(ms: int) -> void:
	if plays.ended:
		return
	var was: bool = playback.is_playing()
	progress_reported.emit(reporter.paused(playback.position_ms()))
	playback.seek(ms)
	reporter.started(ms)
	if was:
		playback.play()


func _refresh() -> void:
	var label: Label = %Plays as Label
	label.visible = plays.limited()
	label.text = tr("ui.audio.plays_left").format({"used": plays.plays_used, "max": plays.max_plays})
	var play: Button = %Play as Button
	play.disabled = plays.awaiting or (plays.exhausted and plays.needs_new_play(playback.position_ms() if playback != null else 0) \
		and not (playback != null and playback.is_playing()))
	play.tooltip_text = tr("ui.audio.plays_exhausted") if play.disabled and plays.exhausted else ""
	(%Exhausted as Control).visible = plays.exhausted and play.disabled
	var speed: Button = %Speed as Button
	speed.visible = speed_allowed
	speed.text = tr("ui.audio.speed").format({"rate": "%.2f×" % SPEEDS[_speed_index] if SPEEDS[_speed_index] < 1.0 else "1×"})
	(%Transcript as Button).visible = transcript_available()
	if playback != null:
		_render_time(playback.position_ms())


func _render_time(pos: int) -> void:
	var seek: HSlider = %Seek as HSlider
	seek.max_value = maxf(1.0, float(duration_ms if duration_ms > 0 else playback.duration_ms()))
	seek.set_value_no_signal(float(pos))
	seek.editable = not plays.ended
	(%Time as Label).text = "%s / %s" % [VideoLessonRenderer.format_time(pos), VideoLessonRenderer.format_time(int(seek.max_value))]


func _toast(key: String) -> void:
	(%Toast as Label).text = key
	(%Toast as Control).visible = true
	_toast_sec = TOAST_SEC
