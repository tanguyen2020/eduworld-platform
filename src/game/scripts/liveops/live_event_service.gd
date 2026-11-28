class_name LiveEventService
extends KeyedWriteService
## Live-ops events of the Event center (F111 US3; EVENT_SEASON_SYSTEM §3–§6, §9–§11; backend F077). The only place of the
## client that calls `/game/events*` and `/game/community-goals/*` (AGENTS R1); the screens read `store` and call:
## - `load_list` — `listGameEvents` with `status = live, upcoming` (every page). The server already filtered the events
##   of the student's scope and grade (`audience.grade_levels`); the client never filters (F111-AC-09);
## - `get_detail` — `getGameEvent` (milestones, community goal, daily points left); `404` closes the detail with a
##   friendly sentence and reads the list again (F111-AC-10, AC-31);
## - `join` — `joinGameEvent` (Idempotency-Key kept when retried); `404 EVENT_NOT_AVAILABLE` / `409 EVENT_ENDED` → a
##   friendly sentence and the detail / list read again;
## - `claim_milestone` — `claimEventMilestone` (key kept when retried); "Đã nhận" and the rewards only from the server's
##   `RewardClaimResult` (F111-AC-11); `409 EVENT_MILESTONE_NOT_REACHED` reads the detail again;
## - `get_goal` — `getCommunityGoal`;
## - realtime: `event.status.changed` (the card changes at once, the list is read again; F111-AC-12),
##   `event.progress.updated` (server totals replace the shown ones, F111-AC-14), `event.community_goal.updated`
##   (F111-AC-13). Nothing is ever added up or sent by the client (ADR-0016). Ids only in logs (R14); nothing on disk.

## The detail of `event_id` is gone (`404`): the screen closes it and says `key`.
signal detail_closed(event_id: String, key: String)
## The server granted a claim: the rewards it says (RewardModel), shown as they are.
signal reward_claimed(rewards: Array)

const PREFIX: String = "event"
const PATH_EVENTS: String = "/game/events"
const PATH_EVENT: String = "/game/events/%s"
const PATH_JOIN: String = "/game/events/%s:join"
const PATH_CLAIM: String = "/game/events/%s/milestones/%s:claim"
const PATH_GOAL: String = "/game/community-goals/%s"
const MSG_STATUS: String = "event.status.changed"
const MSG_PROGRESS: String = "event.progress.updated"
const MSG_GOAL: String = "event.community_goal.updated"
const CODE_NOT_REACHED: String = "EVENT_MILESTONE_NOT_REACHED"

var store: LiveEventStore = LiveEventStore.new()
## `() -> int` server unix seconds (default the ApiClient clock).
var now_source: Callable = Callable()


func _init(rest: RestApiClient = null, realtime: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


func now_unix() -> int:
	if now_source.is_valid():
		return int(now_source.call())
	if api != null and api.clock != null:
		return int(api.clock.unix_time())
	return int(Time.get_unix_time_from_system())


## Coroutine: `listGameEvents` (`status = live,upcoming`, every page).
func load_list() -> ApiResult:
	var pages: Dictionary = await send_all_pages(PATH_EVENTS, {"status": ["live", "upcoming"]}, "listGameEvents")
	if bool(pages["ok"]):
		store.set_events(LiveEventModel.list_from(pages["items"]))
	return pages["result"] as ApiResult if pages["result"] != null else ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


## Coroutine: `getGameEvent`.
func get_detail(event_id: String) -> ApiResult:
	if not Uuid.is_valid(event_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_EVENT % event_id), "getGameEvent")
	if result.ok:
		var e: LiveEventModel = LiveEventModel.from_dict(result.body())
		if e != null:
			store.set_detail(e)
	elif result.status == 404:
		await _gone(event_id, "ui.error.EVENT_NOT_AVAILABLE")
	return result


## Coroutine: "Tham gia" — `joinGameEvent`.
func join(event_id: String) -> ApiResult:
	if not Uuid.is_valid(event_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_JOIN % event_id), "joinGameEvent", "join:%s" % event_id)
	if result.ok:
		var e: LiveEventModel = store.event(event_id)
		if e != null:
			e.joined = true
			if result.body().has("event_points"):
				e.event_points = maxi(0, DataRead.int_of(result.body(), "event_points"))
			store.detail_changed.emit(event_id)
		notice_requested.emit("ui.events.join_done", {})
	elif result.status == 404:
		await _gone(event_id, KeyedWriteService.error_key(result.code if not result.code.is_empty() else "EVENT_NOT_AVAILABLE"))
	else:
		notice_for(result)
		if result.status == 409:
			await get_detail(event_id)
			await load_list()
	return result


## Coroutine: "Nhận" of a reached milestone — `claimEventMilestone`.
func claim_milestone(event_id: String, milestone_id: String) -> ApiResult:
	if not Uuid.is_valid(event_id) or milestone_id.is_empty() or milestone_id.length() > 64:
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_post(PATH_CLAIM % [event_id, milestone_id.uri_encode()])
	var result: ApiResult = await send_keyed(req, "claimEventMilestone", "claim:%s:%s" % [event_id, milestone_id])
	if result.ok:
		if DataRead.bool_of(result.body(), "claimed"):
			store.mark_claimed(event_id, milestone_id)
			reward_claimed.emit(RewardModel.list_from(result.body().get("rewards")))
		return result
	if result.status == 404:
		await _gone(event_id, "ui.error.EVENT_NOT_AVAILABLE")
		return result
	notice_for(result)
	if result.status == 409:
		await get_detail(event_id)
	return result


## Coroutine: `getCommunityGoal`.
func get_goal(goal_id: String) -> ApiResult:
	if not Uuid.is_valid(goal_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_GOAL % goal_id), "getCommunityGoal")
	if result.ok:
		var g: CommunityGoalModel = CommunityGoalModel.from_dict(result.body())
		if g != null:
			store.set_goal(g)
	return result


## Realtime dispatcher handler of `event.*` (payloads checked by WsMessageSchemas first).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	match type:
		MSG_STATUS:
			store.apply_status(DataRead.str_of(payload, "event_id"), DataRead.str_of(payload, "status"))
			load_list()
		MSG_PROGRESS:
			store.apply_progress(payload)
		MSG_GOAL:
			store.apply_goal(payload)


func clear() -> void:
	store.clear()
	forget_keys()


func _gone(event_id: String, key: String) -> void:
	store.remove(event_id)
	notice_requested.emit(key, {})
	detail_closed.emit(event_id, key)
	await load_list()
