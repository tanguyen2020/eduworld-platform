class_name PartyModel
extends RefCounted
## The student's party (F102 data-model §1): the REST `Party` (`createParty`, `getMyParty`, `getParty`, writes that
## return the party) or the payload of the realtime `party.state.snapshot`. Tolerant parse (GAME_NETWORKING §9): a
## wrong field gives the fallback, unknown fields are ignored, a party without a valid id is dropped. It holds what the
## lobby shows and nothing the client decides: the leader, the Ready of each member, the activity the leader chose and
## `state_version` (the order of the realtime changes, F101) all come from the server (R1, R5).

const STATUS_OPEN: String = "open"
const STATUS_IN_ACTIVITY: String = "in_activity"
const STATUS_DISBANDED: String = "disbanded"
const TYPES: PackedStringArray = ["friend", "study_party", "quest", "competition_team", "custom"]
const VISIBILITIES: PackedStringArray = ["private", "friends"]
const POLICIES: PackedStringArray = ["leader_only", "members"]
const NAME_MAX_LENGTH: int = 255
## Platform hard cap of `max_members` (PARTY_SYSTEM §3).
const MAX_MEMBERS_CAP: int = 8

var party_id: String = ""
var name: String = ""
var leader_id: String = ""
var party_type: String = "friend"
var visibility: String = "private"
var invitation_policy: String = "leader_only"
var max_members: int = 4
var status: String = STATUS_OPEN
var state_version: int = 0
var members: Array[PartyMemberModel] = []
## Activity chosen by the leader (`PartyActivityType` + ref); "" = none yet.
var activity_kind: String = ""
var activity_id: String = ""
## Display name of the activity: `{vi, en}` from the eligible list, or `{_: text}` of a realtime ActivityRef title.
var activity_title: Dictionary = {}
var leader_reconnecting: bool = false
## Room of the running session (status in_activity), to come back to it after a reconnect (F101-AC-17).
var room_id: String = ""
var chat_channel_id: String = ""


## REST `Party`; null without a valid `party_id`.
static func from_rest(data: Variant) -> PartyModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var p: PartyModel = _base(d)
	if p == null:
		return null
	for raw: Variant in DataRead.arr_of(d, "members"):
		var m: PartyMemberModel = PartyMemberModel.from_rest(raw)
		if m != null and p.member(m.student_id) == null and p.members.size() < MAX_MEMBERS_CAP:
			p.members.append(m)
	var activity: Dictionary = DataRead.dict_of(d, "current_activity")
	p.activity_kind = DataRead.str_of(activity, "activity_type")
	p.activity_id = DataRead.str_of(activity, "activity_ref")
	p.room_id = DataRead.str_of(d, "current_room_id")
	p._normalise_roles()
	return p


## Payload of `party.state.snapshot`; null without a valid `party_id`.
static func from_snapshot(data: Variant) -> PartyModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var p: PartyModel = _base(d)
	if p == null:
		return null
	for raw: Variant in DataRead.arr_of(d, "members"):
		var m: PartyMemberModel = PartyMemberModel.from_ws(raw)
		if m != null and p.member(m.student_id) == null and p.members.size() < MAX_MEMBERS_CAP:
			p.members.append(m)
	p.set_activity_ref(DataRead.dict_of(d, "current_activity"))
	p.room_id = DataRead.str_of(d, "room_id")
	p._normalise_roles()
	return p


## A realtime `ActivityRef` `{kind, id, title}` (title in the request language).
func set_activity_ref(ref: Dictionary) -> void:
	activity_kind = DataRead.str_of(ref, "kind")
	activity_id = DataRead.str_of(ref, "id")
	activity_title = DataRead.text_of(ref, "title")


func has_activity() -> bool:
	return not activity_kind.is_empty()


func member(student_id: String) -> PartyMemberModel:
	for m: PartyMemberModel in members:
		if m.student_id == student_id:
			return m
	return null


func leader() -> PartyMemberModel:
	return member(leader_id)


func is_full() -> bool:
	return members.size() >= max_members


func is_in_activity() -> bool:
	return status == STATUS_IN_ACTIVITY


## Members that count for the start (online, not in grace): the server rule, shown only as a hint (R1).
func not_ready_count() -> int:
	var n: int = 0
	for m: PartyMemberModel in members:
		if not m.is_disconnected() and not m.ready:
			n += 1
	return n


## The leader changed (manual transfer, leader left, grace over): roles follow the server leader id.
func set_leader(student_id: String) -> void:
	leader_id = student_id
	_normalise_roles()


func copy() -> PartyModel:
	var p: PartyModel = PartyModel.new()
	for key: String in ["party_id", "name", "leader_id", "party_type", "visibility", "invitation_policy", "max_members",
			"status", "state_version", "activity_kind", "activity_id", "leader_reconnecting", "room_id", "chat_channel_id"]:
		p.set(key, get(key))
	p.activity_title = activity_title.duplicate()
	for m: PartyMemberModel in members:
		p.members.append(m.copy())
	return p


static func _base(d: Dictionary) -> PartyModel:
	var id: String = DataRead.str_of(d, "party_id")
	if not Uuid.is_valid(id):
		return null
	var p: PartyModel = PartyModel.new()
	p.party_id = id
	p.name = DataRead.str_of(d, "name").left(NAME_MAX_LENGTH)
	p.leader_id = DataRead.str_of(d, "leader_id")
	p.party_type = _one_of(DataRead.str_of(d, "party_type"), TYPES, "friend")
	p.visibility = _one_of(DataRead.str_of(d, "visibility"), VISIBILITIES, "private")
	p.invitation_policy = _one_of(DataRead.str_of(d, "invitation_policy"), POLICIES, "leader_only")
	p.max_members = clampi(DataRead.int_of(d, "max_members", 4), 2, MAX_MEMBERS_CAP)
	p.status = _one_of(DataRead.str_of(d, "status"), PackedStringArray([STATUS_OPEN, STATUS_IN_ACTIVITY, STATUS_DISBANDED]), STATUS_OPEN)
	p.state_version = maxi(0, DataRead.int_of(d, "state_version", 0))
	p.leader_reconnecting = DataRead.bool_of(d, "leader_reconnecting")
	p.chat_channel_id = DataRead.str_of(d, "chat_channel_id")
	return p


func _normalise_roles() -> void:
	if leader_id.is_empty():
		for m: PartyMemberModel in members:
			if m.is_leader():
				leader_id = m.student_id
	for m: PartyMemberModel in members:
		m.role = PartyMemberModel.ROLE_LEADER if m.student_id == leader_id else PartyMemberModel.ROLE_MEMBER


static func _one_of(value: String, allowed: PackedStringArray, fallback: String) -> String:
	return value if value in allowed else fallback
