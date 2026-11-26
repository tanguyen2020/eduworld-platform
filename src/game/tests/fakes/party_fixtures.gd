class_name PartyFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): the party contract fixtures of tools/gen_fixtures.mjs (OpenAPI response examples
## `getMyParty.200`, `listMyInvitations.200`, `listPartyEligibleActivities.200`, `listFriends.200`, `startPartySession.201`
## and the AsyncAPI `party.*` examples) with the routes of PartyService, for unit and scene tests, the grade-band matrix
## and the review captures. Invented nicknames only (AGENTS R16) — no real student. `self_id` replaces the leader of the
## examples (MeoXanh42) with the signed-in student when a test needs it.

const OPENAPI: String = "res://tests/fixtures/contract/openapi"
const PARTY_ID: String = "0193b0a0-0000-7000-8000-000000000101"
const LEADER: String = "0193b0a0-0000-7000-8000-0000000000a1"
const MEMBER: String = "0193b0a0-0000-7000-8000-0000000000a2"
const WAITING: String = "0193b0a0-0000-7000-8000-0000000000a3"
const NEWCOMER: String = "0193b0a0-0000-7000-8000-0000000000a4"
const INVITATION_ID: String = "0193b0a0-0000-7000-8000-000000000301"
const ROUTE_MY_PARTY: String = "GET /multiplayer/parties/me"
const ROUTE_GET_PARTY: String = "GET /multiplayer/parties/0193b0a0-0000-7000-8000-000000000101"
const ROUTE_CREATE: String = "POST /multiplayer/parties"
const ROUTE_LEAVE: String = "POST /multiplayer/parties/*:leave"
const ROUTE_DISBAND: String = "POST /multiplayer/parties/*:disband"
const ROUTE_TRANSFER: String = "POST /multiplayer/parties/*:transfer-leader"
const ROUTE_KICK: String = "POST /multiplayer/parties/*/members/*:kick"
const ROUTE_READY: String = "PUT /multiplayer/parties/*/members/me/ready"
const ROUTE_ACTIVITY: String = "PUT /multiplayer/parties/*/activity"
const ROUTE_START: String = "POST /multiplayer/parties/*:start"
const ROUTE_ELIGIBLE: String = "GET /multiplayer/parties/*/eligible-activities"
const ROUTE_INVITE: String = "POST /multiplayer/parties/*/invitations"
const ROUTE_SETTINGS: String = "PATCH /multiplayer/parties/0193b0a0-0000-7000-8000-000000000101"
const ROUTE_INVITATIONS: String = "GET /multiplayer/invitations"
const ROUTE_ACCEPT: String = "POST /multiplayer/invitations/*:accept"
const ROUTE_REJECT: String = "POST /multiplayer/invitations/*:reject"
const ROUTE_FRIENDS: String = "GET /social/friends"


static func openapi(name: String) -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [OPENAPI, name])) as Dictionary).duplicate(true)


## A JSON value with every `LEADER` id replaced by `self_id` (the signed-in student leads the sample party).
static func as_self(value: Variant, self_id: String) -> Variant:
	if self_id.is_empty() or self_id == LEADER:
		return value
	return JSON.parse_string(JSON.stringify(value).replace(LEADER, self_id))


## `getMyParty` example (3 members, MeoXanh42 leads, GauNau15 not ready, a quest chosen).
static func party(self_id: String = "") -> Dictionary:
	return as_self(openapi("getMyParty.200"), self_id) as Dictionary


## The party with only the leader (a party just created).
static func party_alone(self_id: String = "") -> Dictionary:
	var p: Dictionary = party(self_id)
	p["members"] = [(p["members"] as Array)[0]]
	p.erase("current_activity")
	p["state_version"] = 1
	(((p["members"] as Array)[0]) as Dictionary)["ready"] = false
	return p


## Every member Ready (the leader may start).
static func party_all_ready(self_id: String = "") -> Dictionary:
	var p: Dictionary = party(self_id)
	for m: Variant in p["members"] as Array:
		(m as Dictionary)["ready"] = true
	return p


## `listMyInvitations` example; with `now_ms` > 0 the invitation expires `seconds` later (server clock of the test).
static func invitations(now_ms: int = 0, seconds: int = 300) -> Dictionary:
	var body: Dictionary = openapi("listMyInvitations.200")
	if now_ms > 0:
		for inv: Variant in body["data"] as Array:
			(inv as Dictionary)["expires_at"] = Time.get_datetime_string_from_unix_time(
					int((now_ms + seconds * 1000) / 1000.0)) + "Z"
	return body


static func eligible() -> Dictionary:
	return openapi("listPartyEligibleActivities.200")


static func friends() -> Dictionary:
	return openapi("listFriends.200")


static func session_start() -> Dictionary:
	return openapi("startPartySession.201")


## The payload of the AsyncAPI example `party.<name>` (e.g. "member.ready_changed.ready").
static func ws(name: String, self_id: String = "") -> Dictionary:
	var env: Dictionary = RealtimeHarness.fixture("party." + name)
	return as_self(env["payload"], self_id) as Dictionary


## An invitation payload expiring `seconds` after `now_ms` (server clock of the test).
static func invitation_ws(now_ms: int, seconds: int = 300) -> Dictionary:
	var payload: Dictionary = ws("invitation.received.friend")
	payload["expires_at"] = now_ms + seconds * 1000
	return payload


## Scripts every read route of the party API on `http` (`alone` = no party: `getMyParty` → 404).
static func script(http: FakeHttp, self_id: String = "", alone: bool = false) -> void:
	for route: String in [ROUTE_MY_PARTY, ROUTE_GET_PARTY, ROUTE_INVITATIONS, ROUTE_ELIGIBLE, ROUTE_FRIENDS]:
		http.clear(route)
	if alone:
		http.reply(ROUTE_MY_PARTY, 404, AuthHarness.problem(404, "PARTY_NOT_FOUND"))
	else:
		http.reply(ROUTE_MY_PARTY, 200, party(self_id))
	http.reply(ROUTE_GET_PARTY, 200, party(self_id))
	http.reply(ROUTE_INVITATIONS, 200, {"data": []})
	http.reply(ROUTE_ELIGIBLE, 200, eligible())
	http.reply(ROUTE_FRIENDS, 200, friends())
