class_name PartyStore
extends RefCounted
## Client state of the student's party (F102 US2/US3; GODOT_ARCHITECTURE §6, GAME_NETWORKING §8): the PartyModel the
## lobby and the HUD read, the pending invitations and the running party session. It applies what the server says and
## decides nothing (AGENTS R1, R5):
## - snapshots (`party.state.snapshot`, REST `Party`) replace the party unless they are older than the one held;
## - realtime changes that carry `state_version` (`party.member.joined`, `party.member.left`, `party.activity.selected`)
##   are applied in order: an old or repeated version is dropped, a gap (> held + 1) asks for `getParty`
##   (`resync_needed`) and the change is not applied (F102-AC-07, F101-AC-04);
## - Ready, presence and leader grace changes (`party.member.ready_changed`, `party.member.status_changed`) touch one
##   member and emit `member_updated` so only that card is redrawn (F102-AC-06, AC-UX-104);
## - leaving, being removed and the party being disbanded close the party (`party_closed`).
## Nothing is ever written to disk; `clear()` drops everything at logout / takeover (F102-AC-19, ADR-0064).

## Structural change (members, leader, activity, status) or a new snapshot; null = no party.
signal party_updated(party: PartyModel)
## Ready / presence of one member changed (redraw only that card) — also the leader grace banner.
signal member_updated(student_id: String)
## The party is gone for this student: &"left", &"kicked", &"disbanded", &"removed", &"cleared" (logout, takeover).
signal party_closed(reason: StringName)
## `party.leader.transferred` (reason manual / leader_left / leader_disconnected).
signal leader_changed(from_id: String, to_id: String, reason: String)
## The leader chose an activity: every Ready was reset by the server (F102 edge case "Đã đổi hoạt động").
signal activity_changed(party: PartyModel)
## A member joined or left (accessibility announce of the lobby).
signal roster_changed(student_id: String, joined: bool)
signal session_started(info: Dictionary)
signal session_ended(info: Dictionary)
signal invitations_changed()
signal invitation_received(invitation: InvitationModel)
## A version gap or an unknown party in a message: the service reloads the party over REST.
signal resync_needed()

const CLOSED_LEFT: StringName = &"left"
const CLOSED_KICKED: StringName = &"kicked"
const CLOSED_DISBANDED: StringName = &"disbanded"
const CLOSED_REMOVED: StringName = &"removed"
const CLOSED_CLEARED: StringName = &"cleared"
## `party_pending_invites_max` (PARTY_SYSTEM §10): the server keeps at most 20 pending invitations per student.
const INVITATIONS_MAX: int = 20

var party: PartyModel = null
## The signed-in student (`student_id` = realtime `user_id` of the party messages).
var self_id: String = ""
var invitations: Array[InvitationModel] = []
## Running party session (`party.session.started` / `startPartySession`): room_id, room_type, scene_ref, activity…
var session: Dictionary = {}
## Members the server found not eligible for the chosen activity (`party.activity.selected.ineligible_members`).
var ineligible: Array[Dictionary] = []
## Activity titles `{vi, en}` by activity ref, from `listPartyEligibleActivities` (the REST Party has no title).
var titles: Dictionary = {}


func has_party() -> bool:
	return party != null


func is_leader() -> bool:
	return party != null and not self_id.is_empty() and party.leader_id == self_id


func me() -> PartyMemberModel:
	return party.member(self_id) if party != null else null


## Replaces the party with `next` unless it is older than the party held (same party). REST bodies have no level:
## a known level of a member is kept. Returns true when applied.
func apply_snapshot(next: PartyModel) -> bool:
	if next == null:
		return false
	if next.status == PartyModel.STATUS_DISBANDED:
		close(CLOSED_DISBANDED)
		return true
	if party != null and next.party_id == party.party_id and next.state_version < party.state_version:
		return false
	if party != null and next.party_id == party.party_id:
		for m: PartyMemberModel in next.members:
			var old: PartyMemberModel = party.member(m.student_id)
			if old != null and m.level <= 0:
				m.level = old.level
		if next.activity_title.is_empty() and next.activity_id == party.activity_id:
			next.activity_title = party.activity_title
	if next.activity_title.is_empty() and titles.has(next.activity_id):
		next.activity_title = titles[next.activity_id] as Dictionary
	if not next.is_in_activity():
		session = {}
	party = next
	party_updated.emit(party)
	return true


## Applies one realtime `party.*` message (RealtimeClient dispatcher, F102-AC-07). Returns false when it was dropped.
func apply_message(type: String, payload: Dictionary) -> bool:
	match type:
		"party.state.snapshot":
			return apply_snapshot(PartyModel.from_snapshot(payload))
		"party.invitation.received":
			return _add_invitation(InvitationModel.from_dict(payload))
		"party.invitation.updated":
			return _update_invitation(payload)
	if party == null or DataRead.str_of(payload, "party_id") != party.party_id:
		# A change of a party this client does not hold (joined elsewhere, missed the snapshot): reload it.
		if party == null and type in ["party.member.joined", "party.activity.selected", "party.session.started"]:
			resync_needed.emit()
		return false
	match type:
		"party.member.joined":
			return _versioned(payload) and _member_joined(payload)
		"party.member.left":
			return _versioned(payload) and _member_left(payload)
		"party.member.status_changed":
			return _member_status(payload)
		"party.member.ready_changed":
			return set_member_ready(DataRead.str_of(payload, "user_id"), DataRead.bool_of(payload, "ready"))
		"party.leader.transferred":
			return _leader_transferred(payload)
		"party.activity.selected":
			return (not payload.has("state_version") or _versioned(payload)) and _activity_selected(payload)
		"party.session.started":
			return start_session(payload)
		"party.session.ended":
			return _session_ended(payload)
		"party.party.disbanded":
			close(CLOSED_DISBANDED)
			return true
	return false


## The server confirmed the Ready of `student_id` (realtime push, WS ack or REST 204).
func set_member_ready(student_id: String, ready: bool) -> bool:
	var m: PartyMemberModel = party.member(student_id) if party != null else null
	if m == null:
		return false
	if m.ready != ready:
		m.ready = ready
		member_updated.emit(student_id)
	return true


## A member left (REST kick confirmed or `party.member.left`) without being this student.
func remove_member(student_id: String) -> bool:
	var m: PartyMemberModel = party.member(student_id) if party != null else null
	if m == null:
		return false
	party.members.erase(m)
	roster_changed.emit(student_id, false)
	party_updated.emit(party)
	return true


## The party session began (`party.session.started` or the `startPartySession` response). A repeat of the same
## session (`party_activity_id` / room) is ignored.
func start_session(info: Dictionary) -> bool:
	if party == null:
		return false
	var room: String = DataRead.str_of(info, "room_id")
	if room.is_empty():
		return false
	if not session.is_empty() and String(session.get("room_id", "")) == room:
		return false
	var activity: Dictionary = DataRead.dict_of(info, "activity")
	session = {
		"room_id": room, "room_type": DataRead.str_of(info, "room_type"),
		"party_activity_id": DataRead.str_of(info, "party_activity_id"),
		"scene_ref": DataRead.str_of(info, "scene_ref"),
		"activity_kind": DataRead.str_of(activity, "kind", DataRead.str_of(info, "activity_type")),
		"activity_id": DataRead.str_of(activity, "id", DataRead.str_of(info, "activity_ref")),
	}
	party.status = PartyModel.STATUS_IN_ACTIVITY
	party.room_id = room
	for m: PartyMemberModel in party.members:
		m.ready = false
	session_started.emit(session.duplicate())
	party_updated.emit(party)
	return true


## The party is gone for this student (`reason`): nothing of it stays in memory.
func close(reason: StringName) -> void:
	var had: bool = party != null
	party = null
	session = {}
	ineligible.clear()
	if had:
		party_closed.emit(reason)
		party_updated.emit(null)


## Replaces the pending invitations (`listMyInvitations?status=pending`).
func set_invitations(list: Array[InvitationModel]) -> void:
	invitations.clear()
	for m: InvitationModel in list:
		if m.is_pending() and invitation(m.invitation_id) == null and invitations.size() < INVITATIONS_MAX:
			invitations.append(m)
	invitations_changed.emit()


func invitation(invitation_id: String) -> InvitationModel:
	for m: InvitationModel in invitations:
		if m.invitation_id == invitation_id:
			return m
	return null


func remove_invitation(invitation_id: String) -> void:
	var m: InvitationModel = invitation(invitation_id)
	if m != null:
		invitations.erase(m)
		invitations_changed.emit()


## Drops the invitations whose server `expires_at` passed at `now_ms` (F102-AC-3: an expired invitation leaves the
## list). Returns how many were dropped.
func expire_invitations(now_ms: int) -> int:
	var gone: Array[InvitationModel] = []
	for m: InvitationModel in invitations:
		if m.is_expired(now_ms):
			gone.append(m)
	for m: InvitationModel in gone:
		invitations.erase(m)
	if not gone.is_empty():
		invitations_changed.emit()
	return gone.size()


## Logout, takeover (`session.session.replaced`), a new session: nothing of the previous student is kept.
func clear() -> void:
	close(CLOSED_CLEARED)
	invitations.clear()
	titles.clear()
	invitations_changed.emit()


# ---------- message handlers ----------

## In-order rule of `state_version` (data-model §3): ≤ held → dropped, held + 1 → applied, more → reload first.
func _versioned(payload: Dictionary) -> bool:
	var v: int = DataRead.int_of(payload, "state_version", -1)
	if v < 0:
		return true
	if v <= party.state_version:
		return false
	if v > party.state_version + 1:
		resync_needed.emit()
		return false
	party.state_version = v
	return true


func _member_joined(payload: Dictionary) -> bool:
	var m: PartyMemberModel = PartyMemberModel.from_ws(payload.get("member"))
	if m == null:
		return false
	if party.member(m.student_id) == null:
		if m.student_id == party.leader_id:
			m.role = PartyMemberModel.ROLE_LEADER
		else:
			m.role = PartyMemberModel.ROLE_MEMBER
		party.members.append(m)
		roster_changed.emit(m.student_id, true)
	party_updated.emit(party)
	return true


func _member_left(payload: Dictionary) -> bool:
	var id: String = DataRead.str_of(payload, "user_id")
	if id == self_id and not self_id.is_empty():
		close(CLOSED_KICKED if DataRead.str_of(payload, "reason") == "kicked" else CLOSED_REMOVED)
		return true
	return remove_member(id)


func _member_status(payload: Dictionary) -> bool:
	var id: String = DataRead.str_of(payload, "user_id")
	var m: PartyMemberModel = party.member(id)
	if m == null:
		return false
	var status: String = DataRead.str_of(payload, "status")
	if not status in PartyMemberModel.PRESENCES:
		return false
	m.presence = status
	if m.is_disconnected():
		# The server resets the Ready of a member who drops in the lobby (PARTY_SYSTEM §9).
		m.ready = false
	if id == party.leader_id:
		party.leader_reconnecting = m.is_disconnected()
	member_updated.emit(id)
	return true


func _leader_transferred(payload: Dictionary) -> bool:
	var to: String = DataRead.str_of(payload, "to_user_id")
	if party.member(to) == null:
		resync_needed.emit()
		return false
	var from: String = DataRead.str_of(payload, "from_user_id")
	party.set_leader(to)
	party.leader_reconnecting = false
	leader_changed.emit(from, to, DataRead.str_of(payload, "reason"))
	party_updated.emit(party)
	return true


func _activity_selected(payload: Dictionary) -> bool:
	var ref: Dictionary = DataRead.dict_of(payload, "activity")
	party.set_activity_ref(ref)
	if titles.has(party.activity_id):
		party.activity_title = titles[party.activity_id] as Dictionary
	for m: PartyMemberModel in party.members:
		m.ready = false
	ineligible.clear()
	for raw: Variant in DataRead.arr_of(payload, "ineligible_members"):
		if raw is Dictionary:
			var e: Dictionary = raw as Dictionary
			ineligible.append({"student_id": DataRead.str_of(e, "user_id", DataRead.str_of(e, "student_id")),
				"reason_code": DataRead.str_of(e, "reason_code"), "requirement": DataRead.str_of(e, "requirement").left(255)})
	activity_changed.emit(party)
	party_updated.emit(party)
	return true


func _session_ended(payload: Dictionary) -> bool:
	party.status = PartyModel.STATUS_OPEN
	party.room_id = ""
	session = {}
	# Only the shared outcome: the summary never carries the score of another student (FR-PTY-013).
	session_ended.emit({"outcome": DataRead.str_of(payload, "outcome"), "reason": DataRead.str_of(payload, "reason")})
	party_updated.emit(party)
	return true


func _add_invitation(m: InvitationModel) -> bool:
	if m == null or invitation(m.invitation_id) != null:
		return false
	invitations.push_front(m)
	while invitations.size() > INVITATIONS_MAX:
		invitations.pop_back()
	invitations_changed.emit()
	invitation_received.emit(m)
	return true


func _update_invitation(payload: Dictionary) -> bool:
	var id: String = DataRead.str_of(payload, "invitation_id")
	var status: String = DataRead.str_of(payload, "status")
	if invitation(id) == null or status == InvitationModel.STATUS_PENDING:
		return false
	remove_invitation(id)
	return true
