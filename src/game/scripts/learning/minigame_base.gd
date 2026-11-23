class_name MiniGameBase
extends Control
## Contract of every mini-game scene (MINIGAME_ENGINE §4.1, F073-AC-22, F073-AC-24). MiniGameHost drives it:
## `setup(config, session, context)` once → `present_item(item)` per item → the scene reports what the student did with
## `response_prepared(item_id, response)` → the host sends it through LearningService and hands the server's grading back
## with `show_feedback(result)` → `finish(summary)` at the end → `teardown()`.
## Rules (ADR-0016, AGENTS R1/R5/R6): a scene never calls the network, never decides right/wrong, never adds points; the
## response is exactly the `QuestionResponse` shape of the item's question type (the F063 answer models build it); the
## right/wrong effect (icon + sound, never colour alone) plays only after `show_feedback`, or at the end when the
## configuration defers it (`feedback_display = end_of_round | end_of_game`). Grades 1–5 get touch targets of at least
## 64 px (`DENSITY_G1_3_BUTTON_MIN_HEIGHT`); mouse, touch and keyboard all work (Enter, 1–6, Tab).
## Signal names are past tense (GODOT_ARCHITECTURE §7): `setup_completed` is the doc's `ready_to_start`,
## `response_prepared` its `response_ready` (same name as QuestionRendererBase).

@warning_ignore("unused_signal")
signal setup_completed()
## `item_id` = the item `position` as a string; `response` = the `QuestionResponse` of the item's type.
@warning_ignore("unused_signal")
signal response_prepared(item_id: String, response: Dictionary)
@warning_ignore("unused_signal")
signal hint_requested(item_id: String)
@warning_ignore("unused_signal")
signal quit_requested()

## Minimum touch target of grades 1–5 (DESIGN_SYSTEM density g1_3, NFR-A11Y-001).
const TOUCH_MIN_PX: int = UiTokens.DENSITY_G1_3_BUTTON_MIN_HEIGHT
const PRIMARY_BANDS: PackedStringArray = ["g1_3", "g4_5"]
const ICON_CORRECT: String = "✓"
const ICON_INCORRECT: String = "✗"
const ICON_SAVED: String = "•"

var config: MiniGameConfigModel = null
var session: MiniGameSessionModel = null
var context: RendererContext = null
var minigame_type: String = ""
## Visual variant from `presentation.scene_variant` ("" = default look).
var variant: String = ""
var item: QuestionRenderModel = null
var locked: bool = false
var remaining_ms: int = -1
## Right/wrong effects shown so far (tests: none before `show_feedback`).
var effects_played: int = 0
var sounds_played: int = 0
## Kind of the last effect (`correct`, `incorrect`, `saved`) and its icon.
var last_effect: String = ""
var last_icon: String = ""
## Graded answers whose effect waits for the end (`feedback_display`).
var deferred: Array[ResponseResultModel] = []
var _sfx: AudioStreamPlayer = null


## Called once after instancing.
func setup(cfg: MiniGameConfigModel, sess: MiniGameSessionModel, ctx: RendererContext = null) -> void:
	config = cfg if cfg != null else MiniGameConfigModel.new()
	session = sess
	context = ctx if ctx != null else RendererContext.new()
	minigame_type = MiniGameRegistry.canonical(sess.minigame_type) if sess != null else minigame_type
	variant = MiniGameRegistry.variant_of(minigame_type, config.scene_variant)
	_on_setup()
	_apply_variant(MiniGameRegistry.variant_style(minigame_type, variant))
	setup_completed.emit()


## Shows one item (render payload only — no answer key).
func present_item(next: QuestionRenderModel) -> void:
	item = next
	locked = false
	_on_present(next)
	apply_touch_targets()


## The server's grading of one answer.
func show_feedback(result: ResponseResultModel) -> void:
	if result == null:
		return
	if config != null and config.defers_effects() and result.has_verdict:
		deferred.append(result)
		_on_deferred(result)
		return
	_play_effect(result)


## Time left on the server clock (deadline_at with the clock offset), -1 without a deadline.
func update_timer(ms: int) -> void:
	remaining_ms = ms
	_on_timer(ms)


## End of the game: the server's summary; deferred effects play now.
func finish(summary: MiniGameSummaryModel) -> void:
	set_locked(true)
	var marks: Array[ResponseResultModel] = deferred.duplicate()
	if not marks.is_empty():
		effects_played += marks.size()
		last_effect = MiniGameFeedbackSfx.KIND_CORRECT if summary != null and summary.completion_met \
				else MiniGameFeedbackSfx.KIND_SAVED
		play_sound(last_effect)
	deferred.clear()
	_on_finish(summary, marks)


func teardown() -> void:
	_on_teardown()
	item = null
	deferred.clear()
	if _sfx != null:
		_sfx.stop()


## Enter / main button: commits the current answer. False when nothing was sent.
func press_primary() -> bool:
	return false


## Keyboard `1–6`.
func select_index(_index: int) -> void:
	pass


func set_locked(value: bool) -> void:
	locked = value


func is_answer_complete() -> bool:
	return false


## The current item cannot be drawn by this scene/client (the host lets the student skip it).
func current_item_unsupported() -> bool:
	return false


## Id of an item in `response_prepared` / the host (its position).
static func item_id_of(q: QuestionRenderModel) -> String:
	return str(q.position) if q != null else ""


## Grades 1–5: the player's band (g1_3, g4_5), or a configuration made only for grades 1–5.
func uses_large_targets() -> bool:
	if context != null and not context.grade_band.is_empty():
		return context.grade_band in PRIMARY_BANDS
	return config != null and config.is_primary()


## Raises every touchable control under `root` to TOUCH_MIN_PX (grades 1–5).
func apply_touch_targets(root: Node = self) -> void:
	if not uses_large_targets():
		return
	for node: Node in root.find_children("*", "Control", true, false):
		if node is BaseButton or node is Slider:
			var c: Control = node as Control
			c.custom_minimum_size = Vector2(maxf(c.custom_minimum_size.x, TOUCH_MIN_PX), maxf(c.custom_minimum_size.y, TOUCH_MIN_PX))
		elif node is LineEdit:
			var e: Control = node as Control
			e.custom_minimum_size = Vector2(e.custom_minimum_size.x, maxf(e.custom_minimum_size.y, TOUCH_MIN_PX))


## Every touchable control under `root` meets TOUCH_MIN_PX (tests, F073-AC-24).
func touch_targets_ok(root: Node = self) -> bool:
	for node: Node in root.find_children("*", "Control", true, false):
		if not (node is BaseButton or node is Slider or node is LineEdit) or not (node as Control).is_visible_in_tree():
			continue
		var m: Vector2 = (node as Control).get_combined_minimum_size()
		if m.y < TOUCH_MIN_PX or (not node is LineEdit and m.x < TOUCH_MIN_PX):
			return false
	return true


func play_sound(kind: String) -> void:
	if _sfx == null:
		_sfx = MiniGameFeedbackSfx.make_player()
		add_child(_sfx)
	_sfx.stream = MiniGameFeedbackSfx.stream(kind)
	if _sfx.is_inside_tree():
		_sfx.play()
	sounds_played += 1


static func icon_of(kind: String) -> String:
	match kind:
		MiniGameFeedbackSfx.KIND_CORRECT:
			return ICON_CORRECT
		MiniGameFeedbackSfx.KIND_INCORRECT:
			return ICON_INCORRECT
	return ICON_SAVED


static func kind_of(result: ResponseResultModel) -> String:
	if result == null or not result.has_verdict:
		return MiniGameFeedbackSfx.KIND_SAVED
	return MiniGameFeedbackSfx.KIND_CORRECT if result.is_correct else MiniGameFeedbackSfx.KIND_INCORRECT


func _play_effect(result: ResponseResultModel) -> void:
	var kind: String = kind_of(result)
	if result.has_verdict:
		effects_played += 1
	last_effect = kind
	last_icon = icon_of(kind)
	_show_effect(kind, result)
	play_sound(kind)


# ---------- overridden by scenes ----------

func _on_setup() -> void:
	pass


func _apply_variant(_style: String) -> void:
	pass


func _on_present(_next: QuestionRenderModel) -> void:
	pass


## Draws the icon + text (+ server points) of a verdict, or "saved" when the timing hides it.
func _show_effect(_kind: String, _result: ResponseResultModel) -> void:
	pass


## A graded answer whose effect waits for the end (no right/wrong shown now).
func _on_deferred(_result: ResponseResultModel) -> void:
	pass


func _on_timer(_ms: int) -> void:
	pass


func _on_finish(_summary: MiniGameSummaryModel, _marks: Array[ResponseResultModel]) -> void:
	pass


func _on_teardown() -> void:
	pass
