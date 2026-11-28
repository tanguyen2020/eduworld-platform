class_name ClubStore
extends RefCounted
## RAM store of the club tab (F111 US5, US6; data-model §3; ADR-0064 — never written to disk, cleared on logout or
## `session.session.replaced`): "Club của em", "Club đề xuất" (none for lớp 1–3), the student's pending invitations, the
## clubs this student asked to join ("Đang chờ duyệt"), and the page of the club on screen (club, members, goals, calendar
## and — owner / officer only — the join requests). A legacy `study_group` never reaches it (ClubModel).

signal clubs_changed()
signal page_changed()

var mine: Array[ClubModel] = []
var discoverable: Array[ClubModel] = []
var invitations: Array[ClubInvitationModel] = []
var loaded: bool = false
## Club ids with a join request of this student waiting (server answered `requestJoinClub`).
var requested: Dictionary = {}
## The club page on screen ("" = none).
var page_id: String = ""
var page: ClubModel = null
var members: Array[ClubMemberModel] = []
var goals: Array[ClubGoalModel] = []
var calendar: Array[ClubEventModel] = []
var requests: Array[ClubJoinRequestModel] = []


func set_lists(mine_list: Array[ClubModel], discover_list: Array[ClubModel], invitation_list: Array[ClubInvitationModel]) -> void:
	mine = mine_list
	# A club the student is already in is never "suggested".
	var own: Dictionary = {}
	for c: ClubModel in mine:
		own[c.club_id] = true
	var others: Array[ClubModel] = []
	for c: ClubModel in discover_list:
		if not own.has(c.club_id):
			others.append(c)
	discoverable = others
	invitations = invitation_list
	loaded = true
	clubs_changed.emit()


func club(club_id: String) -> ClubModel:
	if page != null and page.club_id == club_id:
		return page
	for list: Array[ClubModel] in [mine, discoverable]:
		for c: ClubModel in list:
			if c.club_id == club_id:
				return c
	return null


func is_requested(club_id: String) -> bool:
	return requested.has(club_id)


func mark_requested(club_id: String) -> void:
	requested[club_id] = true
	clubs_changed.emit()
	page_changed.emit()


func remove_invitation(invitation_id: String) -> void:
	for i: ClubInvitationModel in invitations.duplicate():
		if i.invitation_id == invitation_id:
			invitations.erase(i)
	clubs_changed.emit()


func open_page(value: ClubModel) -> void:
	page = value
	page_id = value.club_id if value != null else ""
	members = []
	goals = []
	calendar = []
	requests = []
	page_changed.emit()


func set_page_parts(member_list: Array[ClubMemberModel], goal_list: Array[ClubGoalModel],
		event_list: Array[ClubEventModel]) -> void:
	members = member_list
	goals = goal_list
	calendar = event_list
	page_changed.emit()


func set_requests(list: Array[ClubJoinRequestModel]) -> void:
	requests = list
	page_changed.emit()


func remove_request(request_id: String) -> void:
	for r: ClubJoinRequestModel in requests.duplicate():
		if r.request_id == request_id:
			requests.erase(r)
	page_changed.emit()


func close_page() -> void:
	open_page(null)


func clear() -> void:
	mine = []
	discoverable = []
	invitations = []
	loaded = false
	requested = {}
	open_page(null)
	clubs_changed.emit()
