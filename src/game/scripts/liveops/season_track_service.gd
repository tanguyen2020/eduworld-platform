class_name SeasonTrackService
extends KeyedWriteService
## The free season track of the "Mùa" tab (F111 US4; EVENT_SEASON_SYSTEM §7; backend F077): `/game/season-track*` only
## (AGENTS R1). `load` (`404` → no season: "Mùa mới sắp bắt đầu"), `claim_tier` (Idempotency-Key kept when retried, the
## tier reads claimed and the rewards show only after the server's `RewardClaimResult`; `409 SEASON_TIER_NOT_REACHED` →
## the track is read again), `load_history` ("Mùa trước"), realtime `season.track.updated` (points / tier / the points
## left today replaced by the server values). There is no premium track and nothing to buy (ADR-0049). RAM only.

## The server granted a tier: its rewards (RewardModel).
signal reward_claimed(rewards: Array)

const PREFIX: String = "season"
const PATH_TRACK: String = "/game/season-track"
const PATH_CLAIM: String = "/game/season-track/tiers/%d:claim"
const PATH_HISTORY: String = "/game/season-track/history"
const MSG_UPDATED: String = "season.track.updated"

var store: SeasonTrackStore = SeasonTrackStore.new()


func _init(rest: RestApiClient = null, realtime: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


## Coroutine: `getSeasonTrack`.
func load_track() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_TRACK), "getSeasonTrack")
	if result.ok:
		var t: SeasonTrackModel = SeasonTrackModel.from_dict(result.body())
		if t != null:
			store.set_track(t)
		else:
			store.set_missing()
	elif result.status == 404:
		store.set_missing()
	return result


## Coroutine: "Nhận" of a reached tier — `claimSeasonTier`.
func claim_tier(tier: int) -> ApiResult:
	if tier < 1:
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_CLAIM % tier), "claimSeasonTier", "tier:%d" % tier)
	if result.ok:
		if DataRead.bool_of(result.body(), "claimed"):
			store.mark_claimed(tier)
			reward_claimed.emit(RewardModel.list_from(result.body().get("rewards")))
		return result
	notice_for(result)
	if result.status == 409 or result.status == 404:
		await load_track()
	return result


## Coroutine: `listSeasonTrackHistory` (every page).
func load_history() -> ApiResult:
	var pages: Dictionary = await send_all_pages(PATH_HISTORY, {}, "listSeasonTrackHistory")
	if bool(pages["ok"]):
		store.set_history(SeasonHistoryItemModel.list_from(pages["items"]))
	return pages["result"] as ApiResult if pages["result"] != null else ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if type == MSG_UPDATED:
		store.apply_update(payload)


func clear() -> void:
	store.clear()
	forget_keys()
