extends GdUnitTestSuite
## F060-AC-05 (R1, G-4) and F060-AC-06 (GODOT_ARCHITECTURE §7): layer lint and signal lint catch violations and
## accept the project as it is.

const LayerRules := preload("res://tools/lib/layer_rules.gd")
const SignalRules := preload("res://tools/lib/signal_rules.gd")
const ToolFiles := preload("res://tools/lib/tool_files.gd")


func test_ui_script_with_http_request_fails() -> void:
	var source: String = "extends Control\nfunc _ready() -> void:\n\tvar http: HTTPRequest = HTTPRequest.new()\n"
	var violations: PackedStringArray = LayerRules.check("res://scenes/ui/login/login.gd", source)
	assert_int(violations.size()).is_greater(0)
	assert_str(violations[0]).contains("HTTPRequest")


func test_gameplay_script_with_websocket_or_api_client_fails() -> void:
	var source: String = "extends Node3D\nvar ws: WebSocketPeer\nfunc go() -> void:\n\tApiClient.get_json(\"/x\")\n"
	var violations: PackedStringArray = LayerRules.check("res://scripts/world/zone_loader.gd", source)
	assert_int(violations.size()).is_equal(2)


func test_scene_with_network_node_fails() -> void:
	var scene: String = "[gd_scene format=3]\n\n[node name=\"Net\" type=\"HTTPRequest\" parent=\".\"]\n"
	assert_int(LayerRules.check("res://scenes/ui/x/X.tscn", scene).size()).is_equal(1)


func test_comments_and_strings_do_not_count() -> void:
	var source: String = "extends Control\n# HTTPRequest is forbidden here\nvar label_text: String = \"HTTPClient\"\n"
	assert_int(LayerRules.check("res://scenes/ui/x/x.gd", source).size()).is_equal(0)


func test_service_layer_is_not_checked() -> void:
	assert_bool(LayerRules.applies_to("res://scripts/networking/api_client.gd")).is_false()
	assert_bool(LayerRules.applies_to("res://scenes/ui/debug/ui_gallery.gd")).is_true()


func test_project_has_no_layer_violation() -> void:
	var violations: PackedStringArray = []
	for path: String in ToolFiles.list_files("res://", PackedStringArray(["gd", "tscn"])):
		if LayerRules.applies_to(path):
			violations.append_array(LayerRules.check(path, ToolFiles.read_text(path)))
	assert_array(Array(violations)).is_empty()


func test_signal_not_in_past_tense_fails() -> void:
	var violations: PackedStringArray = SignalRules.check("res://scripts/core/event_bus.gd", "extends Node\nsignal quest_update(id: String)\n")
	assert_int(violations.size()).is_equal(1)
	assert_str(violations[0]).contains("past tense")


func test_signal_rules_naming_and_types() -> void:
	assert_int(SignalRules.check_declaration("quest_progress_updated", "quest_id: String").size()).is_equal(0)
	assert_int(SignalRules.check_declaration("party_updated", "members: Array[String], meta: Dictionary").size()).is_equal(0)
	assert_int(SignalRules.check_declaration("answer_sent", "").size()).is_equal(0)
	assert_int(SignalRules.check_declaration("QuestDone", "").size()).is_greater(0)
	assert_int(SignalRules.check_declaration("zone_loaded", "area_id").size()).is_equal(1)


func test_event_bus_declares_only_signals() -> void:
	var source: String = "extends Node\nsignal zone_changed(area_id: String)\nvar cache: Dictionary = {}\n"
	assert_int(SignalRules.check(SignalRules.EVENT_BUS_PATH, source).size()).is_equal(1)


func test_project_signals_follow_conventions() -> void:
	var violations: PackedStringArray = []
	for path: String in ToolFiles.list_files("res://", PackedStringArray(["gd"]), PackedStringArray(["res://tests"])):
		violations.append_array(SignalRules.check(path, ToolFiles.read_text(path)))
	assert_array(Array(violations)).is_empty()
