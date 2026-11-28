class_name LiveEventService
extends GameApiService
## Live-ops events of the Event center (F111 US3; EVENT_SEASON_SYSTEM §3–§6, §9; backend F077). The only place of the
## client that calls `/game/events*` and `/game/community-goals/*` (AGENTS R1).

const PREFIX: String = "event"
const PATH_EVENTS: String = "/game/events"
const MSG_STATUS: String = "event.status.changed"
const MSG_PROGRESS: String = "event.progress.updated"
const MSG_GOAL: String = "event.community_goal.updated"

var store: LiveEventStore = LiveEventStore.new()
## `() -> int` server unix seconds (default the ApiClient clock).
var now_source: Callable = Callable()


func _init(rest: RestApiClient = null, realtime: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


## Coroutine: `listGameEvents` (`status = live,upcoming`, every page).
func load_list() -> ApiResult:
	var pages: Dictionary = await send_all_pages(PATH_EVENTS, {"status": ["live", "upcoming"]}, "listGameEvents")
	if bool(pages["ok"]):
		store.set_events(LiveEventModel.list_from(pages["items"]))
	return pages["result"] as ApiResult if pages["result"] != null else ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	match type:
		MSG_PROGRESS:
			store.apply_progress(payload)


func clear() -> void:
	store.clear()
