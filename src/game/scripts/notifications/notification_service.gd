class_name NotificationService
extends RefCounted
## In-game notices of new school work (FR-NTF-001, STUDENT_GAME_UX_CORE S03/S05, F064 US4). No realtime push and no
## inbox yet (F092/F115+): each successful refresh of "Nhà trường giao" (entering the Hub, closing a screen, opening
## 🔔 — AssignmentService triggers) is compared with the assignments already seen in this session (RAM only):
## - first refresh of the session → one grouped toast with the number of `not_started` assignments (F064-AC-21);
## - later → one toast per new `not_started` assignment, or one grouped toast when more than MAX_TOASTS arrive at
##   once, and the 🔔 badge goes up (F064-AC-20).
## Texts carry the assignment title the school typed — never another student or a score (AGENTS R16). Nothing here
## runs by itself: no timer, no polling, no notification outside the game (F064-AC-23, FR-NTF-003).

signal toast_requested(key: String, args: Dictionary)
signal badge_changed(count: int)

## STUDENT_GAME_UX_CORE S03 §13: at most 3 toasts at once.
const MAX_TOASTS: int = 3
const STATUS_NOT_STARTED: String = "not_started"
const KEY_PENDING: String = "ui.notification.assignments_pending"
const KEY_NEW: String = "ui.notification.assignment_new"
const KEY_NEW_COUNT: String = "ui.notification.assignments_new_count"
const SCREEN_JOURNAL: StringName = &"quest_journal"
const TAB_ASSIGNED: String = "assigned"

var badge: int = 0
var assignments: AssignmentService = null
## assignment_id → true for every assignment seen in this session.
var _known: Dictionary = {}
var _first: bool = true


## Listens to the refreshes of `service` (only successful ones: a failed refresh keeps the previous list).
func bind(service: AssignmentService) -> void:
	assignments = service
	if not service.refresh_finished.is_connected(_on_refresh_finished):
		service.refresh_finished.connect(_on_refresh_finished)


func on_assignments_refreshed(list: Array[MyAssignmentModel]) -> void:
	var fresh: Array[MyAssignmentModel] = []
	var pending: int = 0
	for a: MyAssignmentModel in list:
		if a.status == STATUS_NOT_STARTED:
			pending += 1
			if not _known.has(a.assignment_id):
				fresh.append(a)
		_known[a.assignment_id] = true
	if _first:
		_first = false
		if pending > 0:
			toast_requested.emit(KEY_PENDING, {"count": pending})
			_set_badge(pending)
		return
	if fresh.is_empty():
		return
	if fresh.size() > MAX_TOASTS:
		toast_requested.emit(KEY_NEW_COUNT, {"count": fresh.size()})
	else:
		for a: MyAssignmentModel in fresh:
			toast_requested.emit(KEY_NEW, {"title": a.title})
	_set_badge(badge + fresh.size())


## The student touched a toast or 🔔: the badge is cleared and the "Nhà trường giao" tab opens (it refreshes the
## list itself, F062-AC-07) — F064-AC-22.
func open_assigned() -> void:
	clear_badge()
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var bus: Node = tree.root.get_node_or_null("EventBus") if tree != null else null
	if bus != null:
		bus.emit_signal("ui_requested", SCREEN_JOURNAL, {"tab": TAB_ASSIGNED})


func clear_badge() -> void:
	_set_badge(0)


## New session (login) or session end: nothing of the previous student is kept.
func reset() -> void:
	_known.clear()
	_first = true
	_set_badge(0)


func _on_refresh_finished(ok: bool) -> void:
	if ok and assignments != null:
		on_assignments_refreshed(assignments.assignments)


func _set_badge(count: int) -> void:
	badge = maxi(0, count)
	badge_changed.emit(badge)
