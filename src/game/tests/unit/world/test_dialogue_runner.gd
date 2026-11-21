extends GdUnitTestSuite
## F071-AC-19, AC-20 (T035): the dialogue graph of the server is walked by DialogueRunner; business actions
## (`give_quest`, `open_minigame`) go to the services through DialogueActions and the runner only shows what came back —
## it never changes quest or reward state by itself; a quest API that is not available shows the fallback line;
## learning opened from an NPC returns the player next to the NPC once after the summary, with nothing chained.

var runner: DialogueRunner = null
var actions: DialogueActions = null
var opened: Array = []


func before_test() -> void:
	runner = DialogueRunner.new()
	actions = DialogueActions.new()
	opened = []
	actions.open_screen = func(screen: StringName, args: Dictionary) -> void: opened.append([screen, args])
	runner.action_handler = actions.handle
	runner.grade_level = 3
	assert_bool(runner.load_graph(WorldFixtures.dialogue())).is_true()


func test_graph_starts_at_the_start_node_with_bilingual_text() -> void:
	var node: Dictionary = await runner.start()
	assert_str(String(node["node_id"])).is_equal("greet")
	assert_str(DialogueRunner.text_of(node, "en-US")).is_equal("The number bridge is broken, will you help me?")
	assert_str(DialogueRunner.text_of(node, "vi-VN")).contains("Cây cầu số")
	assert_int(runner.visible_choices().size()).is_equal(2)


func test_give_quest_calls_the_quest_api_and_shows_its_answer() -> void:
	var calls: Array[Dictionary] = []
	actions.quest_api = func(action: Dictionary) -> ApiResult:
		calls.append(action)
		var r: ApiResult = ApiResult.new()
		r.ok = true
		r.status = 200
		return r
	await runner.start()
	var node: Dictionary = await runner.choose(0)
	assert_int(calls.size()).is_equal(1)
	assert_str(String(calls[0]["type"])).is_equal("give_quest")
	assert_str(String(node["node_id"])).is_equal("accept")
	assert_bool((runner.last_results[0] as ApiResult).ok).is_true()
	assert_array(opened).is_empty()


func test_quest_api_unavailable_shows_the_fallback_line() -> void:
	var notices: Array[String] = []
	actions.notice_requested.connect(func(key: String) -> void: notices.append(key))
	await runner.start()
	await runner.choose(0)
	assert_array(notices).contains_exactly([DialogueActions.NOTICE_UNAVAILABLE])
	assert_str((runner.last_results[0] as ApiResult).code).is_equal(DialogueActions.CODE_UNAVAILABLE)


func test_open_minigame_opens_the_screen_and_nothing_else() -> void:
	var graph: Dictionary = WorldFixtures.dialogue()
	(((graph["nodes"] as Array)[0] as Dictionary)["choices"] as Array)[0]["actions"] = [{"type": "open_minigame",
		"minigame_config_id": "0193a5f4-6c1e-7b2a-9d1f-000000071401"}]
	runner.load_graph(graph)
	var activity: Array[StringName] = []
	actions.activity_opened.connect(func(s: StringName) -> void: activity.append(s))
	await runner.start()
	await runner.choose(0)
	assert_int(opened.size()).is_equal(1)
	assert_str(String(opened[0][0])).is_equal(String(DialogueActions.SCREEN_MINIGAME))
	assert_int(activity.size()).is_equal(1)


func test_later_choice_ends_without_any_action() -> void:
	await runner.start()
	var node: Dictionary = await runner.choose(1)
	assert_str(String(node["node_id"])).is_equal("later")
	assert_array(opened).is_empty()
	await runner.advance()
	assert_bool(runner.is_finished()).is_true()


func test_quest_conditions_are_never_assumed_met() -> void:
	assert_bool(runner.condition_met({"quest_status": {"quest_id": "x", "in": ["completed"]}})).is_false()
	assert_bool(runner.condition_met({"grade_level_in": [3]})).is_true()
	assert_bool(runner.condition_met({"all": [{"grade_level_in": [3]}, {"min_level": 5}]})).is_false()


func test_activity_from_an_npc_returns_next_to_it_once() -> void:
	var back: ActivityReturn = ActivityReturn.new()
	back.remember("learning_zone.math", Vector3(4, 0, 2))
	assert_bool(back.on_closed(&"settings", "learning_zone.math")).is_false()
	assert_bool(back.on_closed(&"activity_host", "learning_zone.math")).is_true()
	assert_vector(back.position).is_equal(Vector3(4, 0, 2))
	assert_bool(back.on_closed(&"activity_host", "learning_zone.math")).is_false()
