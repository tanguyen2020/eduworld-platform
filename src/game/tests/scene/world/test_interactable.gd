extends GdUnitTestSuite
## F062-AC-04 (FR-GAME-003, WORLD_RENDERING §5, AGENTS R1; T009): when the player enters an Interactable the zone
## reports the focus (id + prompt key) so the HUD shows the ContextActionButton with that prompt; acting emits only
## `interacted(interactable_id)`, and the zone turns it into a UI intent (the board opens the "Nhà trường giao" tab).
## The Interactable script calls no service and no network.

const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"
const INTERACTABLE_SCRIPT: String = "res://scripts/world/interactable.gd"
## Free point of the Main Hub: farther than the WorldAnchor reach (2 m) plus the player from every anchor (F094-AC-10).
const WALK_OFF: Vector3 = Vector3(0.0, 0.2, 18.0)

var _intents: Array[Array] = []


func before_test() -> void:
	_intents.clear()
	AppState.spawn_point_id = ""
	EventBus.ui_requested.connect(_on_ui_requested)


func after_test() -> void:
	EventBus.ui_requested.disconnect(_on_ui_requested)


func _on_ui_requested(screen_id: StringName, args: Dictionary) -> void:
	_intents.append([screen_id, args])


func test_entering_the_area_focuses_and_interacting_emits_only_the_id() -> void:
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(2)
	var zone: ZoneRoot = runner.scene() as ZoneRoot
	# Start from a free point: the default spawn (radius 1.5) can overlap the reach of quest.hub.welcome_sign (F094, F093-1).
	zone.player.global_position = WALK_OFF
	await runner.simulate_frames(5)
	var board: Interactable = zone.get_node("Interactables/SchoolAssignmentsBoard") as Interactable
	var focus: Array[Array] = []
	zone.interaction_focus_changed.connect(func(id: String, key: String) -> void: focus.append([id, key]))
	var emitted: Array[String] = []
	board.interacted.connect(func(id: String) -> void: emitted.append(id))
	# Walk the player into the board's trigger area and let physics report the overlap.
	zone.player.global_position = board.global_position + Vector3(0.0, 0.2, 0.0)
	await runner.simulate_frames(10)
	assert_bool(board.player_inside).is_true()
	assert_array(focus).contains([["board.school_assignments", "ui.hub.action.read"]])
	assert_object(zone.focused).is_same(board)
	assert_bool(zone.interact_focused()).is_true()
	assert_array(emitted).is_equal(["board.school_assignments"])
	assert_int(_intents.size()).is_equal(1)
	assert_str(String(_intents[0][0])).is_equal("quest_journal")
	assert_str(String((_intents[0][1] as Dictionary).get("tab", ""))).is_equal("assigned")
	# Leaving clears the focus (the HUD hides the button). The walk-off point is on the south lane, out of reach of every
	# anchor: (0, 0, 12) is the F072 quest object quest.hub.welcome_sign of the world seed (F094, F093-1).
	zone.player.global_position = WALK_OFF
	await runner.simulate_frames(10)
	assert_bool(board.player_inside).is_false()
	assert_array(focus.back() as Array).is_equal(["", ""])
	for node: Node in zone.find_children("*", "Interactable", true, false):
		assert_float((node as Node3D).global_position.distance_to(WALK_OFF)).override_failure_message(String(node.name)).is_greater(3.0)
	assert_bool(zone.interact_focused()).is_false()


func test_hud_button_shows_the_prompt_of_the_focus() -> void:
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/ui/hud/MainHud.tscn")
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	var button: Button = hud.find_child("ContextActionButton", true, false) as Button
	assert_bool(button.visible).is_false()
	hud.call("set_context_action", "board.school_assignments", "ui.hub.action.read")
	assert_bool(button.visible).is_true()
	assert_str(button.text).is_equal("ui.hub.action.read")
	assert_str(button.tr(button.text)).is_not_equal("ui.hub.action.read")
	hud.call("set_context_action", "", "")
	assert_bool(button.visible).is_false()


func test_interactable_has_no_business_logic_or_network() -> void:
	var source: String = FileAccess.get_file_as_string(INTERACTABLE_SCRIPT)
	for forbidden: String in ["ApiClient", "HTTPRequest", "AppState", "services", "AssignmentService", "preload("]:
		assert_str(source).override_failure_message(forbidden).not_contains(forbidden)
	var item: Interactable = Interactable.new()
	item.interactable_id = "npc.math_owl_sage"
	var ids: Array[String] = []
	item.interacted.connect(func(id: String) -> void: ids.append(id))
	item.interact()
	assert_array(ids).is_equal(["npc.math_owl_sage"])
	item.free()
