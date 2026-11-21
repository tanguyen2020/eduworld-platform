extends GdUnitTestSuite
## F071-AC-13 (T023, TC-PERF-018): every zone scene passes the zone lint (unique SpawnPoint / Interactable / AreaPortal
## ids, known area ids, token-preset materials, content safety words) and stays inside the static scene budget of the
## Main Hub benchmark (`scene_budget_hub`, CI tolerance); a broken zone is caught.

const ZoneRules := preload("res://tools/lib/zone_rules.gd")
const ZoneLint := preload("res://tools/zone_lint.gd")
const Bench := preload("res://tests/perf/main_hub_benchmark.gd")
const BROKEN: String = "user://f071_broken_zone.tscn"


func test_all_zones_pass_the_lint() -> void:
	var known: PackedStringArray = ZoneLint.known_areas()
	assert_int(known.size()).is_equal(12)
	var zones: PackedStringArray = ZoneRules.zone_paths()
	assert_int(zones.size()).is_equal(12)
	for path: String in zones:
		assert_array(Array(ZoneRules.check(path, known))).override_failure_message(path).is_empty()


func test_zones_stay_in_the_scene_budget() -> void:
	for path: String in ZoneRules.zone_paths():
		var zone: ZoneRoot = (load(path) as PackedScene).instantiate() as ZoneRoot
		zone.interactive = false
		add_child(zone)
		var stats: Dictionary = Bench.scene_stats(zone)
		assert_int(int(stats["triangles"])).override_failure_message(path).is_less_equal(int(Bench.BUDGET_TRIANGLES * Bench.TOLERANCE))
		assert_int(int(stats["draw_calls"])).override_failure_message(path).is_less_equal(int(Bench.BUDGET_DRAW_CALLS * Bench.TOLERANCE))
		zone.free()


func test_a_broken_zone_is_caught() -> void:
	var zone: ZoneRoot = (load("res://scenes/world/zones/practice_zone/Zone.tscn") as PackedScene).instantiate() as ZoneRoot
	var dup: NpcAnchor = NpcAnchor.new()
	dup.interactable_id = "npc.breeze"
	dup.npc_id = "npc.breeze"
	dup.name = "Sword_Stand"
	zone.get_node("Interactables").add_child(dup)
	dup.owner = zone
	var packed: PackedScene = PackedScene.new()
	packed.pack(zone)
	ResourceSaver.save(packed, BROKEN)
	zone.free()
	var text: String = "\n".join(ZoneRules.check(BROKEN, PackedStringArray(["practice_zone", "main_hub"])))
	assert_str(text).contains("duplicated id npc.breeze")
	assert_str(text).contains("unsafe content word 'sword'")
	DirAccess.remove_absolute(ProjectSettings.globalize_path(BROKEN))
