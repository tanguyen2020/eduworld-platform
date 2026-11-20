class_name ProfileService
extends RefCounted
## Player profile, nickname and avatar of the signed-in student (F070, CHARACTER_SYSTEM §5, STUDENT_GAME_UX_CORE S02).
## Created by the ServiceLocator with ApiClient (tests inject FakeHttp through it); scenes call its intents only
## (AGENTS R1). Routes: `getGameProfile`, `suggestNicknames`, `updateGameProfile` (JSON Merge Patch with
## `nickname_option_id` only — no free-text nickname anywhere, F070-AC-10, R16), `updateAvatar`,
## `listAvatarPresets`, `saveAvatarPreset`. The server decides ownership, slots, uniqueness and the change limit; the
## client only shows the result. On `409 NICKNAME_UNAVAILABLE` new suggestions are fetched automatically
## (F070-AC-09). The nickname is never logged (R14).

signal profile_loaded(profile: GameProfileModel)
signal avatar_saved(config: AvatarConfigModel)
signal nickname_chosen(nickname: String)
signal suggestions_updated(count: int)
signal presets_updated(count: int)
## An account/tenant refusal (`403`) that ends the session (handled by the ServiceLocator → AuthService).
signal access_denied(code: String)

const PATH_PROFILE: String = "/game/profile"
const PATH_SUGGESTIONS: String = "/game/profile/nickname-suggestions"
const PATH_AVATAR: String = "/game/profile/avatar"
const PATH_PRESETS: String = "/game/avatar-presets"
## `suggestNicknames` `count` (contract default 6, maximum 10).
const SUGGESTION_COUNT: int = 6
## `saveAvatarPreset` slots (contract `slot` 1…3 = CONFIG_DEFAULTS `saved_outfits_max`; the server enforces it).
const PRESET_SLOT_MIN: int = 1
const PRESET_SLOT_MAX: int = 3
const CODE_NICKNAME_UNAVAILABLE: String = "NICKNAME_UNAVAILABLE"
const CODE_INVALID_SLOT: String = "VALIDATION_FAILED"
const ACCESS_DENIED_CODES: PackedStringArray = [
	"GAME_ACCESS_STUDENT_ONLY", "ACCESS_OUTSIDE_ALLOWED_HOURS", "ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED",
	"NO_ACTIVE_ENROLLMENT", "TENANT_ACCESS_DISABLED", "ACCOUNT_NOT_PROVISIONED",
]

var api: RestApiClient = null
## Store mirrored with the display fields (`AppState.set_game_profile`), or null in unit tests.
var store: Node = null
var profile: GameProfileModel = null
var suggestions: Array[NicknameOption] = []
var presets: Array[AvatarPresetModel] = []


func _init(rest: RestApiClient = null, app_state: Node = null) -> void:
	api = rest
	store = app_state


## Coroutine: `getGameProfile`. Keeps the previous profile on failure.
func fetch_profile() -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_PROFILE))
	if result.ok:
		var parsed: GameProfileModel = GameProfileModel.from_dict(result.data)
		if parsed == null:
			return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
		_set_profile(parsed)
	return result


## F070-AC-01: no profile yet, or the server says the avatar or the nickname is missing.
func needs_setup() -> bool:
	return profile == null or profile.setup_required


func avatar_config() -> AvatarConfigModel:
	return profile.avatar_config if profile != null else null


## Coroutine: `suggestNicknames?count=6` (server-generated names in the UI language, unused in the tenant).
func suggest_nicknames(count: int = SUGGESTION_COUNT) -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_SUGGESTIONS, {"count": count}))
	if result.ok:
		suggestions = NicknameOption.list_from(result.body().get("data"))
		suggestions_updated.emit(suggestions.size())
	return result


## Coroutine: `updateGameProfile` with `{"nickname_option_id": …}` only. `409 NICKNAME_UNAVAILABLE` (expired, not
## ours, or just taken by someone else in the tenant) → fetches new suggestions before returning (F070-AC-09).
func choose_nickname(option_id: String) -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_patch(PATH_PROFILE, {"nickname_option_id": option_id}))
	if result.ok:
		var parsed: GameProfileModel = GameProfileModel.from_dict(result.data)
		if parsed != null:
			_set_profile(parsed)
			nickname_chosen.emit(parsed.nickname)
		return result
	if result.status == 409 and result.code == CODE_NICKNAME_UNAVAILABLE:
		await suggest_nicknames()
	return result


## Coroutine: `updateAvatar` (PUT, idempotent). 422 `ITEM_NOT_OWNED` / `EQUIP_SLOT_INVALID` /
## `VALIDATION_FAILED` are returned as they are; the UI shows `ui.error.<CODE>`.
func save_avatar(config: AvatarConfigModel) -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_put(PATH_AVATAR, config.to_dict()))
	if result.ok:
		var saved: AvatarConfigModel = AvatarConfigModel.from_dict(result.data)
		if saved == null:
			saved = config.copy()
		if profile != null:
			profile.avatar_config = saved
		avatar_saved.emit(saved)
	return result


## Coroutine: `listAvatarPresets` (saved outfits of this student).
func list_presets() -> ApiResult:
	var result: ApiResult = await _send(ApiRequest.make_get(PATH_PRESETS))
	if result.ok:
		presets = AvatarPresetModel.list_from(result.body().get("data"))
		presets_updated.emit(presets.size())
	return result


## Coroutine: `saveAvatarPreset` into `slot` (1…3); the server checks ownership and slots.
func save_preset(slot: int, config: AvatarConfigModel, preset_name: String = "") -> ApiResult:
	if slot < PRESET_SLOT_MIN or slot > PRESET_SLOT_MAX:
		return ApiResult.failure(CODE_INVALID_SLOT)
	var body: Dictionary = {"avatar_config": config.to_dict()}
	if not preset_name.is_empty():
		body["name"] = preset_name
	var result: ApiResult = await _send(ApiRequest.make_put("%s/%d" % [PATH_PRESETS, slot], body))
	if result.ok:
		var saved: Array[AvatarPresetModel] = AvatarPresetModel.list_from([result.data])
		var updated: Array[AvatarPresetModel] = []
		for p: AvatarPresetModel in presets:
			if p.slot != slot:
				updated.append(p)
		if not saved.is_empty():
			updated.append(saved[0])
		updated.sort_custom(func(a: AvatarPresetModel, b: AvatarPresetModel) -> bool: return a.slot < b.slot)
		presets = updated
		presets_updated.emit(presets.size())
	return result


## Session ended: drops everything of this student.
func clear() -> void:
	profile = null
	suggestions.clear()
	presets.clear()


func _set_profile(parsed: GameProfileModel) -> void:
	profile = parsed
	if store != null and store.has_method("set_game_profile"):
		store.call("set_game_profile", parsed.to_display_dict())
	profile_loaded.emit(parsed)


func _send(req: ApiRequest) -> ApiResult:
	if api == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var result: ApiResult = await api.send(req)
	if result.status == 403 and result.code in ACCESS_DENIED_CODES:
		access_denied.emit(result.code)
	return result
