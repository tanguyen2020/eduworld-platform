class_name AchievementService
extends GameApiService
## Achievements, collections and the badge showcase of the signed-in student (F069 US4, INVENTORY_ECONOMY §6–§7,
## STUDENT_GAME_UX_SOCIAL §4): `listAchievements` (by category, every page), `listCollections`, `getCollection`, the
## owned badges (`listInventory?kind=badge`) and saving the showcase through ProfileService `updateAvatar`
## (`badge_showcase[]`, F069-AC-24). Progress is pulled when the screen opens (no push needed). Unlocking and rewards
## are server decisions; there is no claim call (F069-AC-22).

signal achievements_changed(count: int)
signal collections_changed(count: int)
## A collection vanished on the server (`404`): the list was refreshed (F069-AC-29 "Thử thách không còn mở" family).
signal collection_gone(collection_id: String)

const PATH_ACHIEVEMENTS: String = "/game/achievements"
const PATH_COLLECTIONS: String = "/game/collections"
const PATH_INVENTORY: String = "/game/inventory"
const KIND_BADGE: String = "badge"

var achievements: Array[AchievementModel] = []
var collections: Array[CollectionModel] = []
var badges: Array[InventoryItemModel] = []
var profile: ProfileService = null


func _init(rest: RestApiClient = null, profile_service: ProfileService = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	profile = profile_service


## Coroutine: every page of `listAchievements` (optionally one `category`).
func load_achievements(category: String = "") -> ApiResult:
	var query: Dictionary = {"category": category} if not category.is_empty() else {}
	var page: Dictionary = await send_all_pages(PATH_ACHIEVEMENTS, query, "listAchievements")
	if bool(page["ok"]):
		achievements = AchievementModel.list_from(page["items"])
		achievements_changed.emit(achievements.size())
	return page["result"] as ApiResult


## Categories present in the loaded list, in the order they first appear (chips of the screen).
func categories() -> PackedStringArray:
	var out: PackedStringArray = []
	for a: AchievementModel in achievements:
		if not a.category.is_empty() and not a.category in out:
			out.append(a.category)
	return out


## Coroutine: `listCollections`.
func load_collections() -> ApiResult:
	var page: Dictionary = await send_all_pages(PATH_COLLECTIONS, {}, "listCollections")
	if bool(page["ok"]):
		collections = CollectionModel.list_from(page["items"])
		collections_changed.emit(collections.size())
	return page["result"] as ApiResult


## Coroutine: `getCollection` → the detail (null on failure; a `404` refreshes the list and emits `collection_gone`).
func load_collection(collection_id: String) -> CollectionModel:
	if not Uuid.is_valid(collection_id):
		return null
	var result: ApiResult = await send_request(ApiRequest.make_get("%s/%s" % [PATH_COLLECTIONS, collection_id]),
			"getCollection")
	if result.status == 404:
		await load_collections()
		collection_gone.emit(collection_id)
		return null
	if not result.ok:
		return null
	var detail: CollectionModel = CollectionModel.from_dict(result.data)
	if detail != null:
		for i: int in collections.size():
			if collections[i].collection_id == detail.collection_id:
				collections[i] = detail
	return detail


## Coroutine: the badges the student owns (only these can go on the showcase, F069-AC-24).
func load_badges() -> ApiResult:
	var page: Dictionary = await send_all_pages(PATH_INVENTORY, {"kind": KIND_BADGE}, "listInventory")
	if bool(page["ok"]):
		badges = InventoryItemModel.list_from(page["items"])
	return page["result"] as ApiResult


## Showcase slots: the server catalog limit of `AvatarConfig.badge_showcase` (`maxItems` = CONFIG_DEFAULTS
## `badges_displayed_max`); the server refuses more.
func badge_slots() -> int:
	return AvatarConfigModel.MAX_BADGES


## Badges on the current showcase (profile avatar config).
func showcase() -> PackedStringArray:
	var config: AvatarConfigModel = profile.avatar_config() if profile != null else null
	return config.badge_showcase.duplicate() if config != null else PackedStringArray()


## The owned subset of `codes` in order, at most `badge_slots()` (never a badge the student does not own).
func valid_showcase(codes: PackedStringArray) -> PackedStringArray:
	var owned: PackedStringArray = []
	for b: InventoryItemModel in badges:
		owned.append(b.item_id)
	var out: PackedStringArray = []
	for code: String in codes:
		if code in owned and not code in out and out.size() < badge_slots():
			out.append(code)
	return out


## Coroutine: saves the showcase with `updateAvatar` (ProfileService, F070); the server checks ownership.
func save_showcase(codes: PackedStringArray) -> ApiResult:
	if profile == null or profile.avatar_config() == null:
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	var config: AvatarConfigModel = profile.avatar_config().copy()
	config.badge_showcase = valid_showcase(codes)
	return await profile.save_avatar(config)


func clear() -> void:
	achievements.clear()
	collections.clear()
	badges.clear()
