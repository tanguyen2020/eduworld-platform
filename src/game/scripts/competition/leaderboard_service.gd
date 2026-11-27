class_name LeaderboardService
extends GameApiService
## Boards and tiers of the student (F109 US4, AC-14/15; COMPETITION_SYSTEM §8–§9). The board list, the display
## (`visibility`), the rows, the ranks, the "nhóm X%" and the grade goal are the server's — filtered by grade
## (`leaderboard_visibility`), the school policy (`disabled_leaderboard_definitions`) and the platform config; the client
## never ranks, cuts or sorts (R5, F109-AC-19). Only server-given `leaderboard_id` / `scope_id` / `period_key` are sent
## (F109-AC-21, R8). There is no refresh timer and no "bạn bị vượt qua" message: a board is read when it is opened or
## pulled again (F109-AC-22, NFR-SAFE-003). Tiers come from `getMyRatings` only (no raw rating, FR-RNK-003).

signal boards_changed()
signal page_changed(leaderboard_id: String)
signal tiers_changed()
## A board the server no longer opens (404): its tab is dropped and "Bảng này hiện không mở" is shown.
signal board_closed(leaderboard_id: String)

const PREFIX: String = "competition"
const PATH_BOARDS: String = "/leaderboards"
const PATH_BOARD: String = "/leaderboards/%s"
const PATH_RATINGS: String = "/competition/ratings/me"
const MSG_TIER: String = "competition.tier.updated"
const PAGE_SIZE: int = 50

var realtime: GameRealtimeClient = null
var boards: Array[LeaderboardSummaryModel] = []
var boards_loaded: bool = false
## leaderboard_id → LeaderboardPageModel (the page on screen).
var pages: Dictionary = {}
var tiers: Array[TierView] = []
var tiers_loaded: bool = false


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


## Coroutine: `listLeaderboards` (F109-AC-18) — exactly the boards the server returns.
func load_boards() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_BOARDS), "listLeaderboards")
	if result.ok:
		boards = LeaderboardSummaryModel.list_from(result.body().get("data"))
		boards_loaded = true
		boards_changed.emit()
	return result


## Coroutine: `getLeaderboard` of `board` (its server scope; `period_key` only when the server gave one).
func load_page(board: LeaderboardSummaryModel, period_key: String = "", cursor: String = "") -> ApiResult:
	var query: Dictionary = {"limit": PAGE_SIZE}
	if Uuid.is_valid(board.scope_id):
		query["scope_id"] = board.scope_id
	if not period_key.is_empty():
		query["period_key"] = period_key
	if not cursor.is_empty():
		query["cursor"] = cursor
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_BOARD % board.leaderboard_id, query), "getLeaderboard")
	if result.ok:
		var page: LeaderboardPageModel = LeaderboardPageModel.from_dict(result.body())
		if page != null:
			var held: LeaderboardPageModel = pages.get(board.leaderboard_id) as LeaderboardPageModel
			if not cursor.is_empty() and held != null:
				held.entries.append_array(page.entries)
				held.next_cursor = page.next_cursor
			else:
				pages[board.leaderboard_id] = page
			page_changed.emit(board.leaderboard_id)
	elif result.status == 404:
		_drop_board(board.leaderboard_id)
	return result


## Coroutine: `getMyRatings` — tiers of the current season (never a raw rating).
func load_tiers() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_RATINGS), "getMyRatings")
	if result.ok:
		tiers = TierView.list_from(result.body().get("data"))
		tiers_loaded = true
		tiers_changed.emit()
	return result


func board(leaderboard_id: String) -> LeaderboardSummaryModel:
	for b: LeaderboardSummaryModel in boards:
		if b.leaderboard_id == leaderboard_id:
			return b
	return null


func page(leaderboard_id: String) -> LeaderboardPageModel:
	return pages.get(leaderboard_id) as LeaderboardPageModel


## The tier of `mode` × `subject_id` ("" subject = the first tier of the mode), or null.
func tier_for(mode: String, subject_id: String = "") -> TierView:
	for t: TierView in tiers:
		if t.mode == mode and (subject_id.is_empty() or t.subject_id == subject_id):
			return t
	return null


## Realtime handler for `competition.*`: a new tier reloads `getMyRatings` (the message carries no tier name).
func handle_message(type: String, _payload: Dictionary, _envelope: Variant = null) -> void:
	if type == MSG_TIER:
		load_tiers()


func clear() -> void:
	boards.clear()
	boards_loaded = false
	pages.clear()
	tiers.clear()
	tiers_loaded = false


func _drop_board(leaderboard_id: String) -> void:
	pages.erase(leaderboard_id)
	for i: int in range(boards.size() - 1, -1, -1):
		if boards[i].leaderboard_id == leaderboard_id:
			boards.remove_at(i)
	board_closed.emit(leaderboard_id)
	boards_changed.emit()
