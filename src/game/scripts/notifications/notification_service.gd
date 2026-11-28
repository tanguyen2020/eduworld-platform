class_name NotificationService
extends GameApiService
## In-game notifications of the signed-in student (F123 US4; CHAT_SYSTEM §10, SOCIAL_SYSTEM §8, STUDENT_GAME_UX_CORE
## S03 `NotificationBell` / `ToastStack`). Client service layer only (GODOT_ARCHITECTURE §4, AGENTS R1): the HUD and the
## inbox read its state and send intents.
## - Entering the world or a failed resume (`resync_required`), or `system.service.degraded` naming `notification`:
##   `listNotifications?unread=true` (first page) → the bell badge is the server `unread_count` (F123-AC-15). Old
##   notifications never give a burst of toasts (spec edge case).
## - Realtime `notification.item.created` (RealtimeClient dispatcher `notification.*`): the badge goes up at once; a
##   short toast shows for friend/party invitations and school work (F123-AC-16) unless the student turned popups off
##   in Settings or is in a timed exam / a solo challenge — then only the badge changes. Level-ups, achievements and
##   report outcomes go to the inbox + badge only (SOCIAL_SYSTEM §8, no popup). No sound is ever played (notification
##   sound off by default, F123-AC-16) and nothing is sent outside the game (FR-NTF-003, F123-AC-13).
## - Inbox: `listNotifications` (newest first, cursor pages of 20); the items seen are marked read with the WS command
##   `notification.item.mark_read` (≤ 100 ids), falling back to `markNotificationRead` over REST when the socket is
##   down; "Đã xem hết" = `markAllNotificationsRead` (F123-AC-09, F123-AC-10, F123-AC-18).
## - Opening a notification: marked read, then the screen of its `action` through `EventBus.ui_requested` when this
##   client has that screen (F123-AC-18).
## F064-AC-21 is kept: the first successful refresh of "Nhà trường giao" in a session gives one grouped local toast with
## the number of `not_started` assignments (no badge: the badge is the server unread count). The per-assignment toasts
## of F064-AC-20 are now the server `assignment_new` notifications. The client never creates, scores or rewards
## anything (R5/R6); params (nicknames, titles) are never logged (R14).

signal toast_requested(item: NotificationModel)
signal badge_changed(count: int)
## The inbox list or its paging changed (UI re-renders).
signal items_changed()

const PATH: String = "/notifications"
const PATH_READ: String = "/notifications/%s:read"
const PATH_READ_ALL: String = "/notifications:read-all"
const PREFIX: String = "notification"
const MSG_CREATED: String = "notification.item.created"
const CMD_MARK_READ: String = "notification.item.mark_read"
const DEGRADED_FEATURE: String = "notification"
## Inbox page size (API_DESIGN default page, a screen of rows for every grade band).
const PAGE_SIZE: int = 20
## AsyncAPI `notification.item.mark_read.notification_ids.maxItems`.
const MARK_READ_MAX: int = 100
## The badge text caps at "99+" (HUD).
const BADGE_CAP: int = 99
## Kinds that pop a short toast (F123-AC-16); the others go to the inbox + badge only (SOCIAL_SYSTEM §8).
## F109-AC-26: `tournament_update` too (the toast opens the tournament tab of the arena).
const POPUP_KINDS: PackedStringArray = [
	"friend_request", "party_invitation", "assignment_new", "assignment_results_released", "tournament_update",
]
const STATUS_NOT_STARTED: String = "not_started"
const CODE_PENDING: String = "assignments_pending"
const KIND_LOCAL_PENDING: String = "assignment_new"
const ACTION_OPEN_ASSIGNMENT: String = "open_assignment"
## `action.kind` (NotificationAction) → [screen id, args] of UiScreenRegistry. Screens this client does not have yet
## (friend requests F095) are skipped by `can_open` — the item stays readable in the inbox. F102: a party invitation
## opens the Party screen on its pending invitations (the action id is passed as `id`).
const ACTION_SCREENS: Dictionary = {
	"open_assignment": [&"quest_journal", {"tab": "assigned"}],
	"open_assignment_result": [&"quest_journal", {"tab": "assigned"}],
	"open_achievements": [&"achievements", {}],
	"open_profile": [&"journey", {}],
	"open_friend_requests": [&"friends", {"tab": "requests"}],
	"open_party_invitation": [&"party", {"view": "invitations"}],
}
## F109-AC-26: kinds that open a screen without an `action` — the screen gets the `ref_id` as `tournament_id`
## (`ref_type = tournament`).
const KIND_SCREENS: Dictionary = {"tournament_update": [&"arena", {"tab": "tournaments"}]}
## Actions whose target screen needs the action id (F102: the invitation to show, or "Lời mời đã hết hạn").
const ACTION_WITH_ID: PackedStringArray = ["open_party_invitation"]

var realtime: GameRealtimeClient = null
## Settings autoload (or a test stub) holding the local popup switch `notification_popups` (spec Assumptions:
## device-local, never sent to the server).
var settings: Object = null
## `() -> bool`: true while the student is in a timed exam or a solo challenge (ServiceLocator) — badge only.
var quiet_check: Callable = Callable()
## Kinds whose popup another screen shows (F102: `party_invitation` → the InvitationToast with accept / decline): the
## badge and the inbox row stay, only the generic toast is skipped.
var delegated_popup_kinds: PackedStringArray = []
## Inbox items, newest first (unread entry load + realtime + inbox pages).
var items: Array[NotificationModel] = []
## Server `unread_count` (all pages), kept in step with local reads and realtime items.
var unread_count: int = 0
## Bell badge value (the HUD reads it when it binds).
var badge: int:
	get:
		return unread_count
var page_cursor: String = ""
## True once the full inbox list was loaded (the entry load holds unread items only).
var inbox_loaded: bool = false
var assignments: AssignmentService = null
var _ids: Dictionary = {}
var _first_assignments: bool = true
var _loading: bool = false


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null, settings_node: Object = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	settings = settings_node
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
		realtime.service_degraded.connect(_on_service_degraded)


## F064-AC-21: listens to the refreshes of "Nhà trường giao" (only successful ones).
func bind(service: AssignmentService) -> void:
	assignments = service
	if not service.refresh_finished.is_connected(_on_refresh_finished):
		service.refresh_finished.connect(_on_refresh_finished)


## Coroutine: `listNotifications?unread=true` — entering the world, a failed resume, a degraded service (F123-AC-15).
## Keeps the previous state on failure.
func load_unread() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH, {"unread": true, "limit": PAGE_SIZE}),
			"listNotifications")
	if not result.ok:
		return result
	var page: Array[NotificationModel] = NotificationModel.list_from(result.body().get("data"))
	if not inbox_loaded:
		_replace(page, GameApiService.next_cursor(result.body()))
	else:
		for m: NotificationModel in page:
			_insert(m)
	_set_unread(DataRead.int_of(result.body(), "unread_count", _local_unread()))
	items_changed.emit()
	return result


## Coroutine: first page of the inbox (read and unread, newest first).
func refresh_inbox() -> ApiResult:
	_loading = true
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH, {"limit": PAGE_SIZE}), "listNotifications")
	_loading = false
	if result.ok:
		_replace(NotificationModel.list_from(result.body().get("data")), GameApiService.next_cursor(result.body()))
		inbox_loaded = true
		_set_unread(DataRead.int_of(result.body(), "unread_count", _local_unread()))
		items_changed.emit()
	return result


## Coroutine: the next cursor page of the inbox ("Xem thêm").
func load_more() -> ApiResult:
	if page_cursor.is_empty():
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH, {"limit": PAGE_SIZE, "cursor": page_cursor}),
			"listNotifications")
	if result.ok:
		for m: NotificationModel in NotificationModel.list_from(result.body().get("data")):
			if not _ids.has(m.notification_id):
				_ids[m.notification_id] = m
				items.append(m)
		page_cursor = GameApiService.next_cursor(result.body())
		_set_unread(DataRead.int_of(result.body(), "unread_count", unread_count))
		items_changed.emit()
	return result


func is_loading() -> bool:
	return _loading


## Coroutine: marks `targets` read (the rows the student has seen, F123-AC-18): local state first (badge at once), then
## the WS command (≤ 100 ids per message); REST `markNotificationRead` when the socket cannot take commands. Already read
## or local items are skipped. Returns the number of items newly marked.
func mark_seen(targets: Array[NotificationModel]) -> int:
	var ids: PackedStringArray = []
	for m: NotificationModel in targets:
		if m == null or m.read or m.is_local():
			continue
		m.read = true
		ids.append(m.notification_id)
	if ids.is_empty():
		return 0
	_set_unread(unread_count - ids.size())
	items_changed.emit()
	var start: int = 0
	while start < ids.size():
		var chunk: PackedStringArray = ids.slice(start, start + MARK_READ_MAX)
		start += MARK_READ_MAX
		var sent: bool = false
		if realtime != null:
			var outcome: CommandResult = await realtime.send_command(CMD_MARK_READ, {"notification_ids": Array(chunk)})
			sent = outcome.ok
		if not sent:
			for id: String in chunk:
				await send_request(ApiRequest.make_post(PATH_READ % id), "markNotificationRead")
	return ids.size()


## Coroutine: marks every unread item of the student read (`markAllNotificationsRead`, F123-AC-10).
func mark_all_read() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_post(PATH_READ_ALL), "markAllNotificationsRead")
	if result.ok:
		for m: NotificationModel in items:
			m.read = true
		_set_unread(DataRead.int_of(result.body(), "unread_count", 0))
		items_changed.emit()
	return result


## True when this client has the screen of the item's `action` (UiScreenRegistry).
func can_open(item: NotificationModel) -> bool:
	return not target_screen(item).is_empty()


## Screen id of the item's action in this client, or &"" (no action, or a screen still to come).
func target_screen(item: NotificationModel) -> StringName:
	if item != null and not ACTION_SCREENS.has(item.action_kind) and KIND_SCREENS.has(item.kind):
		var by_kind: StringName = (KIND_SCREENS[item.kind] as Array)[0] as StringName
		return by_kind if UiScreenRegistry.has(by_kind) else &""
	if item == null or not ACTION_SCREENS.has(item.action_kind):
		return &""
	var screen: StringName = (ACTION_SCREENS[item.action_kind] as Array)[0] as StringName
	return screen if UiScreenRegistry.has(screen) else &""


## Intent of a toast or an inbox row (F123-AC-18): marks the item read (the mark itself runs in the background) and
## opens the screen of its action through `EventBus.ui_requested`. Returns false when the item has no screen in this
## client (the caller shows the inbox instead).
func open(item: NotificationModel) -> bool:
	if item == null:
		return false
	if not item.read and not item.is_local():
		mark_seen([item] as Array[NotificationModel])
	var screen: StringName = target_screen(item)
	if screen.is_empty():
		return false
	if not ACTION_SCREENS.has(item.action_kind):
		var kind_args: Dictionary = ((KIND_SCREENS[item.kind] as Array)[1] as Dictionary).duplicate()
		if item.ref_type == "tournament" and Uuid.is_valid(item.ref_id):
			kind_args["tournament_id"] = item.ref_id
		_emit_ui(screen, kind_args)
		return true
	var args: Dictionary = ((ACTION_SCREENS[item.action_kind] as Array)[1] as Dictionary).duplicate()
	if item.action_kind in ACTION_WITH_ID and not item.action_id.is_empty():
		args["id"] = item.action_id
	_emit_ui(screen, args)
	return true


## Realtime dispatcher handler for `notification.*` (GAME_NETWORKING §4.2). A duplicate id (resume replay) is ignored.
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if type != MSG_CREATED:
		return
	var m: NotificationModel = NotificationModel.from_dict(payload)
	if m == null or _ids.has(m.notification_id):
		return
	m.read = false
	_ids[m.notification_id] = m
	items.push_front(m)
	_set_unread(unread_count + 1)
	items_changed.emit()
	if should_popup(m):
		toast_requested.emit(m)


## F123-AC-16: a toast only for invitation / school-work kinds, with popups on and outside an exam or a challenge.
func should_popup(item: NotificationModel) -> bool:
	return item.kind in POPUP_KINDS and not item.kind in delegated_popup_kinds and popups_enabled() and not is_quiet()


func popups_enabled() -> bool:
	if settings == null:
		return true
	var value: Variant = settings.get("notification_popups")
	return not (value is bool) or (value as bool)


func is_quiet() -> bool:
	return quiet_check.is_valid() and bool(quiet_check.call())


## Badge text: "" at 0, the count, or "99+".
static func badge_text(count: int) -> String:
	if count <= 0:
		return ""
	return str(count) if count <= BADGE_CAP else "%d+" % BADGE_CAP


## New session (login) or session end: nothing of the previous student is kept.
func reset() -> void:
	items.clear()
	_ids.clear()
	page_cursor = ""
	inbox_loaded = false
	_first_assignments = true
	_set_unread(0)
	items_changed.emit()


# ---------- F064-AC-21: grouped "pending school work" notice of the first refresh ----------

func on_assignments_refreshed(list: Array[MyAssignmentModel]) -> void:
	if not _first_assignments:
		return
	_first_assignments = false
	var pending: int = 0
	for a: MyAssignmentModel in list:
		if a.status == STATUS_NOT_STARTED:
			pending += 1
	if pending > 0 and popups_enabled() and not is_quiet():
		toast_requested.emit(NotificationModel.local(CODE_PENDING, {"count": pending}, KIND_LOCAL_PENDING,
				ACTION_OPEN_ASSIGNMENT))


func _on_refresh_finished(ok: bool) -> void:
	if ok and assignments != null:
		on_assignments_refreshed(assignments.assignments)


func _on_service_degraded(features: PackedStringArray) -> void:
	if DEGRADED_FEATURE in features:
		load_unread()


func _replace(page: Array[NotificationModel], cursor: String) -> void:
	items.clear()
	_ids.clear()
	for m: NotificationModel in page:
		_insert(m)
	page_cursor = cursor


## Adds `m` in created_at order (newest first) unless its id is known.
func _insert(m: NotificationModel) -> void:
	if _ids.has(m.notification_id):
		return
	_ids[m.notification_id] = m
	var at: int = items.size()
	for i: int in items.size():
		if items[i].created_at_ms < m.created_at_ms:
			at = i
			break
	items.insert(at, m)


func _local_unread() -> int:
	var n: int = 0
	for m: NotificationModel in items:
		if not m.read:
			n += 1
	return n


func _set_unread(count: int) -> void:
	unread_count = maxi(0, count)
	badge_changed.emit(unread_count)


static func _emit_ui(screen: StringName, args: Dictionary) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var bus: Node = tree.root.get_node_or_null("EventBus") if tree != null else null
	if bus != null:
		bus.emit_signal("ui_requested", screen, args)
