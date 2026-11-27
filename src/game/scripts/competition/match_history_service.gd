class_name MatchHistoryService
extends GameApiService
## The student's matches (F109-AC-17: `listMyMatches` with a `mode` filter and a cursor, `getMatch` for the detail and
## the review after the match) and the safety actions on an opponent from the result / detail (F109-AC-16): a report
## goes through ReportService (`createReport`, `target_type = user`, a reason of the enum, one Idempotency-Key per
## report intent; no free text is sent — the contract `ReportCreate` has no note field) and a block through
## SocialService (`blockUser`). Other players are nicknames / avatars only (R16).

signal history_changed()
signal detail_changed(match_id: String)
signal notice_requested(key: String, args: Dictionary)

const PATH_MATCHES: String = "/competition/matches"
const PATH_MATCH: String = "/competition/matches/%s"
const PAGE_SIZE: int = 20

var reports: ReportService = null
var social: SocialService = null
var matches: Array[MatchRecord] = []
var page_cursor: String = ""
var mode_filter: String = ""
var loaded: bool = false
## match_id → MatchRecord (detail with the review).
var details: Dictionary = {}


func _init(rest: RestApiClient = null, report_service: ReportService = null, social_service: SocialService = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	reports = report_service
	social = social_service


## Coroutine: `listMyMatches` (first page for `mode` — "" every mode — or the next page with `more`).
func load_history(mode: String = "", more: bool = false) -> ApiResult:
	if not more:
		mode_filter = mode
	var query: Dictionary = {"limit": PAGE_SIZE}
	if not mode_filter.is_empty():
		query["mode"] = mode_filter
	if more and not page_cursor.is_empty():
		query["cursor"] = page_cursor
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_MATCHES, query), "listMyMatches")
	if result.ok:
		var list: Array[MatchRecord] = MatchRecord.list_from(result.body().get("data"))
		if more:
			matches.append_array(list)
		else:
			matches = list
		page_cursor = GameApiService.next_cursor(result.body())
		loaded = true
		history_changed.emit()
	return result


## Coroutine: `getMatch`. 404 (another student's / tenant's match) → "Không tìm thấy", nothing shown (F109-AC-29).
func load_detail(match_id: String) -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_MATCH % match_id), "getMatch")
	if result.ok:
		var m: MatchRecord = MatchRecord.from_dict(result.body())
		if m != null:
			details[match_id] = m
			detail_changed.emit(match_id)
	else:
		details.erase(match_id)
		if result.status == 404:
			notice_requested.emit(CompetitionText.error_key("NOT_FOUND"), {})
	return result


## Coroutine: report an opponent of `match_id` (reason of `ReportService.REASONS`).
func report_opponent(user_id: String, reason: String, match_id: String = "") -> ApiResult:
	if reports == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var result: ApiResult = await reports.create_report(ReportService.TARGET_USER, user_id, reason, {"match_id": match_id})
	notice_requested.emit("ui.arena.safety.reported" if result.ok else CompetitionText.error_key(result.code), {})
	return result


## Coroutine: block an opponent (`blockUser`); their chat messages leave at once (ServiceLocator wiring of F119).
func block_opponent(user_id: String) -> ApiResult:
	if social == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var result: ApiResult = await social.block_user(user_id)
	notice_requested.emit("ui.arena.safety.blocked" if result.ok else CompetitionText.error_key(result.code), {})
	return result


func has_more() -> bool:
	return not page_cursor.is_empty()


func clear() -> void:
	matches.clear()
	page_cursor = ""
	mode_filter = ""
	loaded = false
	details.clear()
