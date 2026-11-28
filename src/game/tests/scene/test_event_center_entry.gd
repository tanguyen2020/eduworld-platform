extends GdUnitTestSuite
## T003 / T023 (F111-AC-09, F111-AC-29): the Event center opens from the calendar button of the HUD side bar, the
## "Sự kiện" tile of the "Thêm" menu and the event board of Quảng trường Tia Sáng (`event_board` → `event_center`);
## the HUD dot is a static mark (no animation) shown only when there is something new and gone once the screen opened;
## the screen opens on its tab and marks the events seen.

var s: F111SceneHarness = null


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g4_5")


func after_test() -> void:
	s.dispose()


func test_hud_button_with_a_static_dot() -> void:
	var b: EventCenterButton = auto_free(EventCenterButton.new()) as EventCenterButton
	add_child(b)
	b.bind_store(s.services.live_events.store)
	var asked: Array = []
	var sink: Callable = func(screen: StringName, args: Dictionary) -> void: asked.append([screen, args])
	EventBus.ui_requested.connect(sink)
	assert_bool(b.dot_visible()).is_false()
	await s.services.live_events.load_list()
	assert_bool(b.dot_visible()).is_true()
	assert_array(b.find_children("*", "AnimationPlayer", true, false)).is_empty()
	b.pressed.emit()
	EventBus.ui_requested.disconnect(sink)
	assert_str(String(asked[0][0])).is_equal("event_center")
	s.services.live_events.store.mark_seen()
	assert_bool(b.dot_visible()).is_false()
	assert_str(b.tooltip_text).is_equal("ui.events.hud.tooltip")
	assert_object(b.icon).is_not_null()


func test_menu_tile_and_hub_board_open_the_event_center() -> void:
	var tiles: Array = (load("res://scenes/ui/hud/game_menu.gd") as GDScript).get_script_constant_map()["TILES"] as Array
	assert_bool(tiles.any(func(t: Array) -> bool: return t[0] == &"event_center")).is_true()
	assert_str(String(ZoneRoot.KIND_SCREENS.get("event_board", &""))).is_equal("event_center")
	var zone: String = FileAccess.get_file_as_string("res://scenes/world/zones/main_hub/Zone.tscn")
	assert_bool(zone.contains("kind = \"event_board\"")).is_true()


func test_screen_opens_on_the_event_and_marks_everything_seen() -> void:
	var screen: EventCenterScreen = auto_free((load("res://scenes/liveops/EventCenterScreen.tscn") as PackedScene).instantiate()) as EventCenterScreen
	screen.services = s.services
	add_child(screen)
	screen.size = Vector2(1280, 720)
	await screen.open({"id": F111Fixtures.EVENT_FESTIVAL})
	await s.settle(4)
	assert_str(screen.tab).is_equal(EventCenterScreen.TAB_EVENTS)
	assert_bool(screen.detail_view().visible).is_true()
	assert_bool(s.services.live_events.store.has_new).is_false()
	screen.detail_view().back_button().pressed.emit()
	await s.settle(2)
	assert_object(screen.card(F111Fixtures.EVENT_FESTIVAL)).is_not_null()
	screen.close()
