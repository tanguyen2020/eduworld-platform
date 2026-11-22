extends Control
## DialogueBubble (screen `npc_dialogue`, F071 T037, WORLD_RENDERING §9): shows the NPC dialogue served by the server
## for this student's grade — speaker name and lines in the player's language (vi/en from the data, ADR-0056), choices
## as buttons. DialogueRunner walks the graph; DialogueActions performs the actions through services (F071-AC-19).
## UI only: no rule is decided here (AGENTS R1). A dialogue the server cannot serve shows the fallback line.

signal closed()

const FALLBACK_KEY: String = "ui.world.dialogue.unavailable"

var runner: DialogueRunner = DialogueRunner.new()
var actions: DialogueActions = DialogueActions.new()
var npc_id: String = ""
var _npc_name: Dictionary = {}

@onready var _speaker: Label = %Speaker
@onready var _body: Label = %Body
@onready var _choices: VBoxContainer = %Choices
@onready var _next: Button = %Next
@onready var _close: Button = %Close


func _ready() -> void:
	visible = false
	_next.pressed.connect(_on_next)
	_close.pressed.connect(close)
	runner.node_changed.connect(_render)
	runner.finished.connect(close)
	runner.action_handler = actions.handle
	actions.notice_requested.connect(_on_notice)
	actions.activity_opened.connect(_on_activity_opened)
	var services: ServiceLocator = AppState.get_services()
	if services != null and services.quest != null:
		actions.quest_api = services.quest.dialogue_action # give_quest / complete_step (F072)


## `args.npc_id`: fetches the dialogue (ETag cache in WorldService) and starts it.
func open(args: Dictionary = {}) -> void:
	npc_id = String(args.get("npc_id", ""))
	show()
	var world: WorldService = AppState.get_services().world
	var result: ApiResult = await world.fetch_dialogue(npc_id)
	if not result.ok or not runner.load_graph(result.body()):
		_speaker.text = ""
		_on_notice(FALLBACK_KEY)
		return
	_npc_name = result.body().get("npc_name", {}) as Dictionary if result.body().get("npc_name") is Dictionary else {}
	var profile: Dictionary = AppState.player.get("game_profile", {}) as Dictionary if AppState.player.get("game_profile") is Dictionary else {}
	runner.player_level = int(profile.get("level", 1))
	runner.grade_level = int(AppState.player.get("grade_level", 0)) if AppState.player.get("grade_level") is float or AppState.player.get("grade_level") is int else 0
	await runner.start()


func close() -> void:
	if visible:
		hide()
		closed.emit()


func _render(node: Dictionary) -> void:
	_speaker.text = L10n.pick(_npc_name) if String(node.get("speaker", "npc")) == "npc" else ""
	_body.text = DialogueRunner.text_of(node)
	for child: Node in _choices.get_children():
		child.queue_free()
	var choices: Array[Dictionary] = runner.visible_choices()
	for i: int in choices.size():
		var b: Button = Button.new()
		b.text = DialogueRunner.text_of(choices[i])
		b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		b.pressed.connect(_on_choice.bind(i))
		_choices.add_child(b)
	_next.visible = choices.is_empty()


func _on_choice(index: int) -> void:
	await runner.choose(index)


func _on_next() -> void:
	if runner.is_finished():
		close()
		return
	await runner.advance()


func _on_notice(key: String) -> void:
	_body.text = tr(key)
	_next.visible = true


## Learning opened from the NPC: the bubble closes; WorldRoot returns the player here after the summary (F071-AC-20).
func _on_activity_opened(_screen: StringName) -> void:
	close()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()
