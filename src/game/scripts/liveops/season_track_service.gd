class_name SeasonTrackService
extends GameApiService
## The free season track of the "Mùa" tab (F111 US4; EVENT_SEASON_SYSTEM §7; backend F077): `/game/season-track*`.

const PREFIX: String = "season"
const PATH_TRACK: String = "/game/season-track"
const MSG_UPDATED: String = "season.track.updated"

var store: SeasonTrackStore = SeasonTrackStore.new()


func _init(rest: RestApiClient = null, realtime: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


## Coroutine: `getSeasonTrack`.
func load() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_TRACK), "getSeasonTrack")
	if result.ok:
		store.set_track(SeasonTrackModel.from_dict(result.body()))
	return result


func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if type == MSG_UPDATED:
		store.apply_update(payload)


func clear() -> void:
	store.clear()
