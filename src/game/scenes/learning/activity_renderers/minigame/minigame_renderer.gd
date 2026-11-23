class_name MinigameActivityRenderer
extends ActivityRendererBase
## Renderer of `activity_type = minigame` (F073, MINIGAME_ENGINE §4; ACTIVITY_ENGINE plugin `minigame`) inside the F063
## activity player: a quest step or a school assignment that wraps a mini-game configuration. The attempt's render
## payload carries `{minigame_type, minigame_config_version_id, config}`; this renderer hosts a MiniGameHost (embedded)
## which runs the game on the same attempt through the LearningService ActivityHost already uses (`attach_service`) —
## the renderer itself never calls the network. ActivityHost keeps its frame (exit/pause, server clock, offline re-send,
## time up); the mini-game shows its own buttons, so the host action bar stays hidden. "Tiếp tục" on the mini-game
## summary asks ActivityHost for its result screen (`finish_requested`); an unknown `minigame_type` asks for the update
## screen (`template_load_failed`, like the simulation renderer).

## Same contract as the simulation renderer: ActivityHost shows "Cần cập nhật game" + telemetry (F073-AC-21).
@warning_ignore("unused_signal")
signal template_load_failed(reason: String)

const HOST_SCENE: PackedScene = preload("res://scenes/minigames/MiniGameHost.tscn")
const REASON_UNKNOWN_MINIGAME: String = "minigame_type_unknown"

var host: MiniGameHost = null
var service: LearningService = null


## ActivityHost hands over its LearningService (the attempt, its answer queue and keys live there).
func attach_service(learning: LearningService) -> void:
	service = learning


func setup(attempt_model: AttemptModel, ctx: RendererContext) -> void:
	super.setup(attempt_model, ctx)
	host = HOST_SCENE.instantiate() as MiniGameHost
	host.embedded = true
	host.grade_band = ctx.grade_band if ctx != null else ""
	host.size_flags_vertical = Control.SIZE_EXPAND_FILL
	add_child(host)
	host.bind_service(service)
	host.finished.connect(func() -> void: finish_requested.emit())
	host.update_required.connect(func(_type: String) -> void: template_load_failed.emit(REASON_UNKNOWN_MINIGAME))
	var session: MiniGameSessionModel = MiniGameSessionModel.from_attempt(attempt_model)
	if session == null:
		template_load_failed.emit(REASON_UNKNOWN_MINIGAME)
		return
	host.play(session, ctx)


func check() -> bool:
	return host != null and host.press_primary()


func select_index(index: int) -> void:
	if host != null:
		host.select_index(index)


func set_locked(locked: bool) -> void:
	if host != null:
		host.lock(locked)


func wants_action_bar() -> bool:
	return false


func is_last() -> bool:
	return host == null or host.is_last()


func current_item() -> QuestionRenderModel:
	return host.current_item() if host != null else null


func current_position() -> int:
	var item: QuestionRenderModel = current_item()
	return item.position if item != null else 0


func teardown() -> void:
	if host != null:
		host.teardown()
