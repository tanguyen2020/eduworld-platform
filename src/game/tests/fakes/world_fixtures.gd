class_name WorldFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): F071 world bodies for FakeHttp, generated once from the backend world seed
## (src/backend/modules/game/seed/world) in the shapes of specs/openapi `WorldArea`, `AreaExploration`,
## `StoryChapterProgress`, `AreaEnterResult`, `AreaLockedProblem` and specs/schemas/npc-dialogue.schema.json.

const DIR: String = "res://tests/fixtures/world"
const ROUTE_AREAS: String = "GET /game/world/areas"
const ROUTE_EXPLORATION: String = "GET /game/world/exploration"
const ROUTE_ENTER: String = "POST /game/world/areas/*:enter"
const ROUTE_DISCOVER: String = "POST /game/world/areas/*/discoveries/*:discover"
const ROUTE_DIALOGUE: String = "GET /game/world/npcs/*/dialogue"
const ROUTE_START_LEVEL: String = "POST /game/levels/*:start"
const SESSION_ID: String = "44444444-4444-4444-8444-444444444444"


static func json(file_name: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s" % [DIR, file_name])) as Dictionary


static func areas() -> Dictionary:
	return json("areas.json")


static func exploration() -> Dictionary:
	return json("exploration.json")


static func dialogue() -> Dictionary:
	return json("dialogue_mr_numbers.json")


static func enter_math() -> Dictionary:
	return json("enter_math.json")


static func area_locked() -> Dictionary:
	return json("area_locked.json")


static func area_models() -> Array[WorldAreaModel]:
	var out: Array[WorldAreaModel] = []
	for d: Variant in areas()["data"] as Array:
		out.append(WorldAreaModel.from_dict(d))
	return out


## Store double of AppState for WorldService (game session id + published world state).
class FakeStore:
	extends Node
	var game_session_id: String = WorldFixtures.SESSION_ID
	var world: Dictionary = {}

	func set_world(state: Dictionary) -> void:
		world = state
