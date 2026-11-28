class_name LiveEventStore
extends RefCounted
## RAM store of the Event center (F111 US3, data-model §3; ADR-0064 — never written to disk, cleared on logout or
## `session.session.replaced`): the events of `listGameEvents` in the server order, the detail of the event on screen and
## the "something new" flag of the static HUD dot (F111-AC-29: on when an event id appears that the student has not seen
## since the Event center was last opened, off once it is opened). Every value is the server's: realtime updates
## REPLACE points, the daily points left and the community goal (a repeated message changes nothing, ADR-0016).

signal events_changed()
signal detail_changed(event_id: String)
signal goal_changed(goal_id: String)
signal new_content_changed(has_new: bool)

var events: Array[LiveEventModel] = []
var loaded: bool = false
## Event ids the student has seen in the Event center (this session only).
var seen_ids: Dictionary = {}
var has_new: bool = false


func set_events(list: Array[LiveEventModel]) -> void:
	# Keep the detail already read of an event still listed.
	var kept: Array[LiveEventModel] = []
	for e: LiveEventModel in list:
		var old: LiveEventModel = event(e.event_id)
		if old != null and old.has_detail:
			old.apply(_summary_of(e))
			kept.append(old)
		else:
			kept.append(e)
	events = kept
	loaded = true
	_update_new()
	events_changed.emit()


func event(event_id: String) -> LiveEventModel:
	for e: LiveEventModel in events:
		if e.event_id == event_id:
			return e
	return null


## `getGameEvent` body: merged into the listed event (or added when the list does not have it yet).
func set_detail(detail: LiveEventModel) -> void:
	var e: LiveEventModel = event(detail.event_id)
	if e == null:
		events.append(detail)
	else:
		events[events.find(e)] = detail
	detail_changed.emit(detail.event_id)
	events_changed.emit()


## Removes an event that is gone (`404`, cancelled).
func remove(event_id: String) -> void:
	var e: LiveEventModel = event(event_id)
	if e != null:
		events.erase(e)
		events_changed.emit()


## `event.status.changed`: the status changes at once (the list is read again by the service). `cancelled` removes it.
func apply_status(event_id: String, status: String) -> void:
	var e: LiveEventModel = event(event_id)
	if e == null:
		return
	if status == LiveEventModel.CANCELLED:
		remove(event_id)
		return
	if status in LiveEventModel.STATUSES:
		e.status = status
		detail_changed.emit(event_id)
		events_changed.emit()


## `event.progress.updated`: server totals replace the shown values.
func apply_progress(payload: Dictionary) -> void:
	var e: LiveEventModel = event(DataRead.str_of(payload, "event_id"))
	if e == null:
		return
	e.apply_progress(payload)
	detail_changed.emit(e.event_id)


## `event.community_goal.updated` (or a `getCommunityGoal` body as `goal`): the bar of every event with that goal.
func apply_goal(payload: Dictionary) -> void:
	var goal_id: String = DataRead.str_of(payload, "goal_id")
	for e: LiveEventModel in events:
		if e.community_goal != null and e.community_goal.goal_id == goal_id:
			e.community_goal.apply_update(payload)
			goal_changed.emit(goal_id)
			detail_changed.emit(e.event_id)


func set_goal(goal: CommunityGoalModel) -> void:
	for e: LiveEventModel in events:
		if e.community_goal != null and e.community_goal.goal_id == goal.goal_id:
			e.community_goal = goal
			goal_changed.emit(goal.goal_id)
			detail_changed.emit(e.event_id)


## The server confirmed the claim (`RewardClaimResult.claimed`): only now the milestone reads "Đã nhận".
func mark_claimed(event_id: String, milestone_id: String) -> void:
	var e: LiveEventModel = event(event_id)
	var m: EventMilestoneModel = e.milestone(milestone_id) if e != null else null
	if m != null:
		m.claimed = true
		m.reached = true
		m.just_reached = false
		detail_changed.emit(event_id)


## The Event center was opened: the HUD dot goes away (F111-AC-29).
func mark_seen() -> void:
	for e: LiveEventModel in events:
		seen_ids[e.event_id] = true
	_set_new(false)


func clear() -> void:
	events = []
	loaded = false
	seen_ids = {}
	_set_new(false)
	events_changed.emit()


func _update_new() -> void:
	for e: LiveEventModel in events:
		if not seen_ids.has(e.event_id) and (e.is_live() or e.is_scheduled()):
			_set_new(true)
			return


func _set_new(value: bool) -> void:
	if value == has_new:
		return
	has_new = value
	new_content_changed.emit(value)


static func _summary_of(e: LiveEventModel) -> Dictionary:
	return {"event_run_id": e.event_run_id, "name": e.name, "short_tagline": e.short_tagline, "starts_at": e.starts_at,
		"ends_at": e.ends_at, "status": e.status, "joined": e.joined, "event_points": e.event_points}
