class_name F110Fixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): the F110 contract fixtures of tools/gen_fixtures.mjs (OpenAPI `BossSummary`, `BossEncounter`,
## `BossQuestion`, `QuestStepClues`, `QuestStepRoles`, `QuestPartyStepProgress`, `FriendRequest`, `FriendCode`,
## `PlayerRelation`, the `listFriends.200` example, the AsyncAPI `boss.*` / `quest.*` / `presence.friend.updated`
## samples and the `battle.question.shown` question render) filled with invented sample values — nicknames of the
## party fixtures, a two-phase sample boss, a two-role step — plus the routes of BossClient, CoopQuestService and
## SocialService. No real student, school or content (R2, R16).

const OPENAPI: String = "res://tests/fixtures/contract/openapi"
const ASYNCAPI: String = "res://tests/fixtures/contract/asyncapi"
const BOSS_ID: String = "0193b0a0-0000-7000-8000-000000000b01"
const BOSS_PARTY_ONLY_ID: String = "0193b0a0-0000-7000-8000-000000000b02"
const ENCOUNTER_ID: String = "0193b0a0-0000-7000-8000-000000000e01"
const OTHER_ENCOUNTER_ID: String = "0193b0a0-0000-7000-8000-000000000e02"
const QUEST_ID: String = "0193b0a0-0000-7000-8000-000000000c01"
const STEP_ID: String = "map_and_measure"
const ROLE_MEASURE: String = "measurer"
const ROLE_MAP: String = "map_reader"
## Other students of the party fixtures (nicknames only).
const MATE: String = PartyFixtures.MEMBER
const MATE_NICK: String = "CaHeoVui07"
const THIRD: String = PartyFixtures.WAITING
const THIRD_NICK: String = "GauNau15"
const FRIEND_ONLINE: String = "0193b0a0-0000-7000-8000-0000000000a2"
const FRIEND_OFFLINE: String = "0193b0a0-0000-7000-8000-0000000000a5"
const STRANGER: String = "0193b0a0-0000-7000-8000-0000000000b7"
const REQUEST_IN: String = "0193b0a0-0000-7000-8000-000000000d01"
const REQUEST_OUT: String = "0193b0a0-0000-7000-8000-000000000d02"
const ROUTE_BOSSES: String = "GET /game/bosses"
const ROUTE_CREATE: String = "POST /game/bosses/*/encounters"
const ROUTE_ENCOUNTER: String = "GET /game/boss-encounters/*"
const ROUTE_LEAVE: String = "POST /game/boss-encounters/*:leave"
const ROUTE_CLUES: String = "GET /game/quests/*/steps/*/clues"
const ROUTE_SHARE: String = "POST /game/quests/*/steps/*/clues/*:share"
const ROUTE_ROLES: String = "PUT /game/quests/*/steps/*/roles"
const ROUTE_START_ACTIVITY: String = "POST /game/quests/*/steps/*/activities:start"
const ROUTE_FRIENDS: String = "GET /social/friends"
const ROUTE_REQUESTS_IN: String = "GET /social/friend-requests?direction=incoming"
const ROUTE_REQUESTS_OUT: String = "GET /social/friend-requests?direction=outgoing"
const ROUTE_SEND: String = "POST /social/friend-requests"
const ROUTE_ACCEPT: String = "POST /social/friend-requests/*:accept"
const ROUTE_REJECT: String = "POST /social/friend-requests/*:reject"
const ROUTE_CODES: String = "POST /social/friend-codes"
const ROUTE_REDEEM: String = "POST /social/friend-codes:redeem"
const ROUTE_REMOVE: String = "DELETE /social/friends/*"
const ROUTE_BLOCK: String = "PUT /social/blocks/*"
const ROUTE_UNBLOCK: String = "DELETE /social/blocks/*"
const ROUTE_MUTE: String = "PUT /social/mutes/*"
const ROUTE_UNMUTE: String = "DELETE /social/mutes/*"
const ROUTE_BLOCKS: String = "GET /social/blocks"
const ROUTE_MUTES: String = "GET /social/mutes"


static func openapi(name: String) -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [OPENAPI, name])) as Dictionary).duplicate(true)


static func asyncapi(name: String) -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [ASYNCAPI, name])) as Dictionary).duplicate(true)


## The payload of the AsyncAPI sample `<type>` with `fields` merged in.
static func ws(type: String, fields: Dictionary) -> Dictionary:
	var p: Dictionary = asyncapi("%s.sample" % type)["payload"] as Dictionary
	p.merge(fields, true)
	return p


static func iso(ms: int) -> String:
	return Time.get_datetime_string_from_unix_time(int(ms / 1000), false) + "Z"


# ---------- boss ----------

static func boss(solo: bool = true) -> Dictionary:
	var b: Dictionary = openapi("BossSummary")
	b["boss_id"] = BOSS_ID if solo else BOSS_PARTY_ONLY_ID
	b["code"] = "fraction_dragon" if solo else "storm_giant"
	b["title"] = {"vi": "Rồng Phân Số", "en": "Fraction Dragon"} if solo else {"vi": "Khổng Lồ Bão", "en": "Storm Giant"}
	b["area_id"] = "math_kingdom"
	b["phase_count"] = 2 if solo else 3
	b["modes"] = ["solo", "party"] if solo else ["party"]
	b["party_size_min"] = 2
	b["party_size_max"] = 4
	b["defeated_before"] = false
	b.erase("subject_id")
	return b


static func bosses() -> Dictionary:
	return {"data": [boss(true), boss(false)], "page": {"has_more": false, "limit": 100}}


static func member(id: String, nick: String, ready: bool = true, present: bool = true, met: bool = false) -> Dictionary:
	return {"student_id": id, "nickname": nick, "ready": ready, "present": present, "contribution_met": met}


## A REST `BossEncounter` (mode, status, version; party members MeoXanh42 = self, CaHeoVui07, GauNau15).
static func encounter(self_id: String, mode: String = "solo", status: String = "lobby", version: int = 1,
		server_ms: int = 0) -> Dictionary:
	var e: Dictionary = openapi("BossEncounter")
	e["encounter_id"] = ENCOUNTER_ID
	e["boss_id"] = BOSS_ID
	e["boss_version"] = 1
	e["mode"] = mode
	if mode == "party":
		e["party_id"] = PartyFixtures.PARTY_ID
		e["members"] = [member(self_id, "MeoXanh42"), member(MATE, MATE_NICK), member(THIRD, THIRD_NICK, false)]
	else:
		e.erase("party_id")
		e["members"] = [member(self_id, "MeoXanh42")]
	e["status"] = status
	e["current_phase"] = 0
	e["phase_id"] = "p1"
	e["phase_count"] = 2
	e["energy_remaining_pct"] = 100
	e["phase_progress"] = 0
	e["phase_retries_left"] = 1
	e["phases_cleared"] = 0
	e["state_version"] = version
	var now: int = server_ms if server_ms > 0 else 1_763_712_000_000
	e["server_time"] = iso(now)
	e["phase_deadline_at"] = iso(now + 120_000)
	e["deadline_at"] = iso(now + 600_000)
	e["started_at"] = iso(now)
	e.erase("ended_at")
	e.erase("outcome")
	e["my_questions"] = []
	return e


## The single-choice question render of the battle example ("6 × 3 = ?", four tiles), as a boss question.
static func question_render(instance: String = "bq_p1_1") -> Dictionary:
	var q: Dictionary = (asyncapi("battle.question.shown.single_choice")["payload"] as Dictionary)["question"] as Dictionary
	q["question_instance_id"] = instance
	return q


## `BossQuestion` of `getBossEncounter.my_questions` (the REST form after a reconnect).
static func rest_question(instance: String = "bq_p1_1", answered: bool = false) -> Dictionary:
	var render: Dictionary = question_render(instance)
	render.erase("question_instance_id")
	return {"question_instance_id": instance, "requirement_id": "r1", "question_type": "single_choice",
		"time_limit_seconds": 30, "answered": answered, "render": render}


static func question_assigned(instances: PackedStringArray = ["bq_p1_1"], deadline_ms: int = 1_763_712_120_000) -> Dictionary:
	var qs: Array = []
	for id: String in instances:
		qs.append(question_render(id))
	return ws("boss.question.assigned", {"encounter_id": ENCOUNTER_ID, "phase_id": "p1", "requirement_id": "r1",
		"questions": qs, "deadline_at": iso(deadline_ms)})


static func updated(version: int, energy: float, members: Array = [], status: String = "phase_active") -> Dictionary:
	var p: Dictionary = ws("boss.encounter.updated", {"encounter_id": ENCOUNTER_ID, "state_version": version,
		"status": status, "current_phase": 0, "phase_id": "p1", "energy_remaining_pct": energy, "phase_progress": 0.4,
		"phase_deadline_at": iso(1_763_712_120_000)})
	if members.is_empty():
		p.erase("members")
	else:
		p["members"] = members
	return p


static func phase_started(index: int, retry: bool = false, deadline_ms: int = 1_763_712_120_000) -> Dictionary:
	return ws("boss.phase.started", {"encounter_id": ENCOUNTER_ID, "phase_id": "p%d" % (index + 1), "phase_index": index,
		"deadline_at": iso(deadline_ms), "retry": retry, "presentation_ref": "boss/fraction_dragon/phase_%d" % (index + 1)})


static func phase_completed(index: int, energy: float) -> Dictionary:
	return ws("boss.phase.completed", {"encounter_id": ENCOUNTER_ID, "phase_id": "p%d" % (index + 1), "phase_index": index,
		"energy_remaining_pct": energy, "next_phase_index": index + 1})


static func evaluated(instance: String, correct: bool) -> Dictionary:
	var p: Dictionary = ws("boss.answer.evaluated", {"encounter_id": ENCOUNTER_ID, "question_instance_id": instance,
		"is_correct": correct, "counted": true,
		"explanation": {"vi": "6 nhóm 3 là 18.", "en": "6 groups of 3 make 18."}})
	if correct:
		p.erase("explanation")
	return p


static func ended(outcome: String, phases_cleared: int = 2, eligible: bool = true) -> Dictionary:
	var rewards: Array = [{"kind": "xp", "amount": 120}, {"kind": "currency", "amount": 30, "currency_code": "coin"}] \
			if outcome == "defeated" and eligible else []
	return ws("boss.encounter.ended", {"encounter_id": ENCOUNTER_ID, "outcome": outcome, "phases_cleared": phases_cleared,
		"phase_count": 2, "eligible": eligible, "rewards": rewards})


# ---------- co-op quest ----------

static func clue(role: String, shared: bool, by: String = "") -> Dictionary:
	var c: Dictionary = (openapi("QuestStepClues")["clues"] as Array)[0] as Dictionary
	c["role"] = role
	c["title"] = {"vi": "Người đo", "en": "Measurer"} if role == ROLE_MEASURE else {"vi": "Người đọc bản đồ", "en": "Map reader"}
	c["icon_key"] = "edit" if role == ROLE_MEASURE else "map"
	c["shared"] = shared
	if by.is_empty():
		c.erase("shared_by_nickname")
	else:
		c["shared_by_nickname"] = by
	c["content"] = {"blocks": [{"type": "paragraph", "text": "Sợi dây dài 12 m." if role == ROLE_MEASURE
			else "Cây cầu ở phía Bắc khu rừng."}]}
	return c


static func clues(my_roles: PackedStringArray, cards: Array) -> Dictionary:
	return {"quest_id": QUEST_ID, "step_id": STEP_ID, "my_roles": Array(my_roles), "clues": cards}


static func roles(self_id: String, mine: PackedStringArray = [ROLE_MEASURE], pending_leader: bool = false) -> Dictionary:
	var a: Array = []
	for role: String in [ROLE_MEASURE, ROLE_MAP]:
		var holder: String = self_id if role in mine else MATE
		a.append({"role": role, "student_id": holder, "nickname": "MeoXanh42" if holder == self_id else MATE_NICK})
	return ws("quest.roles.assigned", {"quest_id": QUEST_ID, "step_id": STEP_ID, "assignments": a,
		"my_roles": Array(mine), "pending_leader": pending_leader})


static func clue_shared(role: String, by: String) -> Dictionary:
	return ws("quest.clue.shared", {"quest_id": QUEST_ID, "step_id": STEP_ID, "role": role, "shared_by_nickname": by})


static func progress(collective: float, mine_met: bool, shared_done: bool = false, step: String = "collect_answers") -> Dictionary:
	var p: Dictionary = ws("quest.progress.updated", {"quest_id": QUEST_ID, "status": "active", "step_id": step,
		"step_status": "active"})
	for k: String in ["quest_progress_id", "quest_version_id", "count", "target", "passed", "hint", "next_step_ids",
			"progress_label", "completed_at"]:
		p.erase(k)
	p["party_progress"] = {"step_id": step, "collective_progress": collective, "my_contribution_met": mine_met,
		"shared_completed": shared_done}
	return p


## A quest delivery with a collective step and the interdependent step (both active).
static func coop_quest() -> Dictionary:
	return {"quest_id": QUEST_ID, "quest_version_id": "0193b0a0-0000-7000-8000-000000000c02", "code": "bridge_team",
		"name": {"vi": "Cùng nhau xây cầu", "en": "Build the bridge together"}, "quest_type": "side", "grade_levels": [3],
		"status": "active", "steps": [
			{"step_id": "collect_answers", "order": 1, "step_type": "activity", "optional": false,
				"title": {"vi": "Cả nhóm trả lời 20 câu", "en": "The team answers 20 questions"}},
			{"step_id": STEP_ID, "order": 2, "step_type": "interdependent", "optional": false,
				"title": {"vi": "Đo dây và đọc bản đồ", "en": "Measure the rope and read the map"}}],
		"progress": {"quest_progress_id": "0193b0a0-0000-7000-8000-000000000c03", "quest_id": QUEST_ID,
			"quest_version_id": "0193b0a0-0000-7000-8000-000000000c02", "status": "active", "started_at": "2026-11-20T08:00:00Z",
			"step_states": {"collect_answers": {"status": "active", "count": 15, "target": 20}, STEP_ID: {"status": "active"}}}}


# ---------- social ----------

static func friends() -> Dictionary:
	return openapi("listFriends.200")


static func card(id: String, nick: String, presence: String = "online") -> Dictionary:
	var c: Dictionary = openapi("PlayerCard")
	c["user_id"] = id
	c["nickname"] = nick
	c["level"] = 8
	c["presence"] = presence
	c["avatar_config"] = (friends()["data"] as Array)[0]["avatar_config"]
	c.erase("title")
	return c


static func request(id: String, incoming: bool, nick: String, status: String = "pending") -> Dictionary:
	var r: Dictionary = openapi("FriendRequest")
	r["request_id"] = id
	r["status"] = status
	r["requester"] = card(STRANGER if incoming else FRIEND_ONLINE, nick)
	r["addressee"] = card(STRANGER if not incoming else FRIEND_ONLINE, nick)
	r["created_at"] = "2026-11-20T08:00:00Z"
	r["expires_at"] = "2026-11-27T08:00:00Z"
	return r


static func code(expires_ms: int) -> Dictionary:
	return {"code": "K7M2QX", "expires_at": iso(expires_ms)}


static func relation(id: String, nick: String) -> Dictionary:
	var r: Dictionary = openapi("PlayerRelation")
	r["user_id"] = id
	r["nickname"] = nick
	r.erase("avatar_thumbnail_url")
	r["avatar_config"] = (friends()["data"] as Array)[0]["avatar_config"]
	r["created_at"] = "2026-11-20T08:00:00Z"
	return r


## Every F110 route of `self_id` on `http` (a two-boss area, a solo encounter, the friends sample, one request each
## way, empty block lists).
static func script(http: FakeHttp, self_id: String, server_ms: int = 0) -> void:
	http.reply(ROUTE_BOSSES, 200, bosses())
	http.reply(ROUTE_CREATE, 201, encounter(self_id, "solo", "lobby", 1, server_ms))
	http.reply(ROUTE_ENCOUNTER, 200, encounter(self_id, "solo", "lobby", 1, server_ms))
	http.reply(ROUTE_LEAVE, 204)
	http.reply(ROUTE_CLUES, 200, clues(PackedStringArray([ROLE_MEASURE]), [clue(ROLE_MEASURE, false)]))
	http.reply(ROUTE_SHARE, 200, clues(PackedStringArray([ROLE_MEASURE]), [clue(ROLE_MEASURE, true, "MeoXanh42")]))
	http.reply(ROUTE_ROLES, 200, {"quest_id": QUEST_ID, "step_id": STEP_ID, "assignments": [
		{"role": ROLE_MEASURE, "student_id": self_id, "nickname": "MeoXanh42"},
		{"role": ROLE_MAP, "student_id": MATE, "nickname": MATE_NICK}]})
	http.reply(ROUTE_FRIENDS, 200, friends())
	http.reply(ROUTE_REQUESTS_IN, 200, {"data": [request(REQUEST_IN, true, "SaoBang12")]})
	http.reply(ROUTE_REQUESTS_OUT, 200, {"data": [request(REQUEST_OUT, false, "BuomXanh09", "rejected")]})
	http.reply(ROUTE_SEND, 201, request(REQUEST_OUT, false, "BuomXanh09"))
	http.reply(ROUTE_ACCEPT, 204)
	http.reply(ROUTE_REJECT, 204)
	http.reply(ROUTE_CODES, 201, code((server_ms if server_ms > 0 else 1_763_712_000_000) + 600_000))
	http.reply(ROUTE_REDEEM, 201, request(REQUEST_OUT, false, "BuomXanh09"))
	http.reply(ROUTE_REMOVE, 204)
	http.reply(ROUTE_BLOCK, 204)
	http.reply(ROUTE_UNBLOCK, 204)
	http.reply(ROUTE_MUTE, 204)
	http.reply(ROUTE_UNMUTE, 204)
	http.reply(ROUTE_BLOCKS, 200, {"data": [], "page": {"has_more": false, "limit": 100}})
	http.reply(ROUTE_MUTES, 200, {"data": [], "page": {"has_more": false, "limit": 100}})
