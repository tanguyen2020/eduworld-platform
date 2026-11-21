extends GdUnitTestSuite
## F064 T015 (FR-EXM-003; GODOT_ARCHITECTURE §6, AGENTS R1): the Main Hub greybox has the Exam Zone entry — an
## Interactable `board.exam_sessions` (kind `exam_board`) whose interaction is only a UI intent `exam_lobby` (the exam
## list); the Interactable itself calls no service. The full `exam_zone` scene is F071.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"

var _intents: Array[Array] = []


func before_test() -> void:
	_intents.clear()
	AppState.spawn_point_id = ""
	EventBus.ui_requested.connect(_on_ui_requested)


func after_test() -> void:
	EventBus.ui_requested.disconnect(_on_ui_requested)


func _on_ui_requested(screen_id: StringName, args: Dictionary) -> void:
	_intents.append([screen_id, args])


func test_exam_board_opens_the_exam_lobby() -> void:
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(2)
	var zone: ZoneRoot = runner.scene() as ZoneRoot
	var board: Interactable = zone.get_node("Interactables/ExamBoard") as Interactable
	assert_str(board.interactable_id).is_equal("board.exam_sessions")
	assert_str(board.kind).is_equal("exam_board")
	assert_str(board.prompt_key).starts_with("ui.hub.action.")
	zone.player.global_position = board.global_position + Vector3(0.0, 0.2, 0.0)
	await runner.simulate_frames(10)
	assert_object(zone.focused).is_same(board)
	assert_bool(zone.interact_focused()).is_true()
	assert_int(_intents.size()).is_equal(1)
	assert_str(String(_intents[0][0])).is_equal("exam_lobby")
	assert_dict(_intents[0][1] as Dictionary).is_empty()
	assert_bool(UiScreenRegistry.has(&"exam_lobby")).is_true()


func test_exam_screen_is_an_exam_activity_for_presence() -> void:
	# AsyncAPI presence `activity` enum has `exam`; the exam screen reports it like the activity player reports learning.
	assert_array(PresenceService.SCREEN_STATUS[&"exam_lobby"] as Array).is_equal([PresenceService.IN_ACTIVITY, "exam"])
