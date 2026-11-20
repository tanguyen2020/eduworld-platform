class_name ProfileFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): F070 player-profile bodies for FakeHttp. Hand-written test data in
## res://tests/fixtures/profile using the item/colour codes of the backend seed (`character_catalog.v1.json`) and the
## shapes of specs/openapi `GameProfile`, `CharacterCatalog`, `NicknameSuggestion`, `AvatarPreset`.

const DIR: String = "res://tests/fixtures/profile"
const ROUTE_PROFILE: String = "GET /game/profile"
const ROUTE_PATCH_PROFILE: String = "PATCH /game/profile"
const ROUTE_CATALOG: String = "GET /game/character/catalog"
const ROUTE_SUGGESTIONS: String = "GET /game/profile/nickname-suggestions"
const ROUTE_AVATAR: String = "PUT /game/profile/avatar"
const ROUTE_PRESETS: String = "GET /game/avatar-presets"
const ROUTE_SAVE_PRESET: String = "PUT /game/avatar-presets/*"
const CATALOG_ETAG: String = "\"catalog-v1\""


static func json(file_name: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s" % [DIR, file_name])) as Dictionary


static func catalog() -> Dictionary:
	return json("character_catalog.json")


static func catalog_model() -> CharacterCatalogModel:
	return CharacterCatalogModel.from_dict(catalog())


static func profile_new() -> Dictionary:
	return json("game_profile_new.json")


static func profile_ready() -> Dictionary:
	return json("game_profile_ready.json")


static func suggestions() -> Dictionary:
	return json("nickname_suggestions.json")


static func suggestions_more() -> Dictionary:
	return json("nickname_suggestions_more.json")


## `GameProfile` after the avatar and the nickname of the first suggestion are saved.
static func profile_after_setup(nickname: String = "Cú Mèo Chăm Chỉ 27") -> Dictionary:
	var body: Dictionary = profile_ready()
	body["nickname"] = nickname
	return body


static func preset_list(slots: Array[int]) -> Dictionary:
	var data: Array = []
	var config: Dictionary = profile_ready()["avatar_config"] as Dictionary
	for slot: int in slots:
		data.append({"slot": slot, "name": "Bộ %d" % slot, "avatar_config": config.duplicate(true),
				"updated_at": "2026-11-20T07:30:00Z"})
	return {"data": data}


## Scripts the routes a first-time player needs on FakeHttp (profile, catalog, nickname suggestions).
static func script_first_time(http: FakeHttp) -> void:
	http.clear(ROUTE_PROFILE)
	http.reply(ROUTE_PROFILE, 200, profile_new())
	http.reply(ROUTE_CATALOG, 200, catalog(), {"ETag": CATALOG_ETAG})
	http.reply(ROUTE_SUGGESTIONS, 200, suggestions())


## Scripts the routes of a player who already has an avatar and a nickname.
static func script_ready(http: FakeHttp) -> void:
	http.clear(ROUTE_PROFILE)
	http.reply(ROUTE_PROFILE, 200, profile_ready())
	http.reply(ROUTE_CATALOG, 200, catalog(), {"ETag": CATALOG_ETAG})
