class_name ActivityRendererBase
extends Control
## Contract of every activity renderer (plan F063 Phase 0, GODOT_ARCHITECTURE §6.2, modelled on MiniGameBase of
## MINIGAME_ENGINE §4.1); reused by F064 (exam), F065 (video), F066 (audio, simulation), F073 (mini-game).
## `ActivityHost` owns the frame (header, feedback strip, hints, action bar) and the service calls; a renderer only
## draws its content and reports what the student did. It never grades and never holds an answer key (ADR-0016).
## Signal names are past tense (GODOT_ARCHITECTURE §7): `response_submitted` is the plan's `response_ready`,
## `progress_reported` its `progress_ready`.

## An `ActivityResponseSubmission` (`{kind, position | cue_id…, response, client_elapsed_ms}`) ready to send.
@warning_ignore("unused_signal")
signal response_submitted(submission: Dictionary)
## An `ActivityProgressReport` (video/audio heartbeat).
@warning_ignore("unused_signal")
signal progress_reported(report: Dictionary)
@warning_ignore("unused_signal")
signal hint_requested(position: int)
@warning_ignore("unused_signal")
signal finish_requested()
@warning_ignore("unused_signal")
signal quit_requested()
## Whether the current answer can be checked (the host enables "Kiểm tra").
@warning_ignore("unused_signal")
signal answer_changed(complete: bool)
## The item now on screen (host refreshes progress, hints and voice-over).
@warning_ignore("unused_signal")
signal item_shown(position: int)

var attempt: AttemptModel = null
var context: RendererContext = null


func setup(attempt_model: AttemptModel, ctx: RendererContext) -> void:
	attempt = attempt_model
	context = ctx


## "Kiểm tra" pressed: emits `response_submitted` when the answer is complete; false otherwise.
func check() -> bool:
	return false


## The server's grading of the last submission.
func show_result(_result: ResponseResultModel) -> void:
	pass


func show_hint(_hint: HintResultModel) -> void:
	pass


## Moves to the next item; false when the current one was the last.
func advance() -> bool:
	return false


func current_position() -> int:
	return 0


func current_item() -> QuestionRenderModel:
	return null


func is_last() -> bool:
	return true


## 1-based index of the current item and the item count (progress indicator).
func progress() -> Vector2i:
	return Vector2i(0, 0)


func is_answer_complete() -> bool:
	return false


## Keyboard `1–6`: selects the n-th choice of the current item.
func select_index(_index: int) -> void:
	pass


## Time up / submitting: no more input.
func set_locked(_locked: bool) -> void:
	pass


## The current item has no renderer in this client (F063-AC-26).
func current_item_unsupported() -> bool:
	return false


func teardown() -> void:
	pass


## Whether the host shows its main button now (a video shows it only while an embedded question is open, F065).
func wants_action_bar() -> bool:
	return true


## The server's `ActivityProgressState` after a heartbeat (media renderers mirror the accepted ranges and pending cues).
func apply_progress(_state: Dictionary) -> void:
	pass


## A heartbeat refused by the server (`VIDEO_SEEK_NOT_ALLOWED` → back to the watched frontier).
func progress_refused(_code: String) -> void:
	pass


## The last `ActivityProgressReport` to send before the player closes or pauses ({} when none).
func final_report() -> Dictionary:
	return {}
