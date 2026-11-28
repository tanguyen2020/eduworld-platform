class_name F111Fixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): the F111 contract fixtures of tools/gen_fixtures.mjs (OpenAPI `FriendSuggestion`, `Club*`,
## `GameEventSummary`, `GameEventDetail`, `EventMilestone`, `CommunityGoalProgress`, `RewardClaimResult`, `SeasonTrack*`,
## `QuestSummary`, the `listFriendSuggestions.200`, `listMyClubInvitations.200` and `listClubGoals.200` examples, the
## AsyncAPI `event.*` / `season.track.updated` samples) filled with invented sample values — nicknames of the party
## fixtures, a sample festival "Lễ hội Trăng Rằm", a 30-tier free season — plus the routes of SocialService
## (suggestions), ClubService, LiveEventService, SeasonTrackService and the daily / weekly quests. No real student,
## school or content (R2, R16).

const OPENAPI: String = "res://tests/fixtures/contract/openapi"
const ASYNCAPI: String = "res://tests/fixtures/contract/asyncapi"
## Players (nicknames only; the suggestion ids are those of the listFriendSuggestions example).
const SUGGESTED_COOP: String = "0193b0a0-0000-7000-8000-0000000000a6"
const SUGGESTED_MATCH: String = "0193b0a0-0000-7000-8000-0000000000a7"
const SUGGESTED_PARTY: String = "0193b0a0-0000-7000-8000-0000000000a8"
const FRIEND_ONLINE: String = F110Fixtures.FRIEND_ONLINE
const FRIEND_NICK: String = "CaHeoVui07"
const FRIEND_OFFLINE: String = F110Fixtures.FRIEND_OFFLINE
const MEMBER_OTHER: String = "0193b0a0-0000-7000-8000-0000000000b9"
## Clubs.
const CLUB_SCHOOL: String = "0193b0a0-0000-7000-8000-000000000c01"
const CLUB_OWN: String = "0193b0a0-0000-7000-8000-000000000c02"
const CLUB_PENDING: String = "0193b0a0-0000-7000-8000-000000000c03"
const CLUB_DISCOVER: String = "0193b0a0-0000-7000-8000-000000000c04"
const CLUB_LOCKED: String = "0193b0a0-0000-7000-8000-000000000c05"
const CLUB_STUDY: String = "0193b0a0-0000-7000-8000-000000000c06"
const CLUB_INVITED: String = "0193b0a0-0000-7000-8000-000000000c07"
const CLUB_CHANNEL: String = "0193b0a0-0000-7000-8000-000000000502"
const INVITATION_ID: String = "0193b0a0-0000-7000-8000-000000000c31"
const REQUEST_ID: String = "0193b0a0-0000-7000-8000-000000000c41"
const REQUEST_ID_2: String = "0193b0a0-0000-7000-8000-000000000c42"
## Live-ops events.
const EVENT_FESTIVAL: String = "0193b0a0-0000-7000-8000-000000000f01"
const EVENT_SCHOOL: String = "0193b0a0-0000-7000-8000-000000000f02"
const EVENT_UPCOMING: String = "0193b0a0-0000-7000-8000-000000000f03"
const RUN_FESTIVAL: String = "0193b0a0-0000-7000-8000-000000000f11"
const GOAL_ID: String = "0193b0a0-0000-7000-8000-000000000f21"
const EVENT_QUEST: String = "0193b0a0-0000-7000-8000-000000000f31"
const SEASON_ID: String = "0193b0a0-0000-7000-8000-000000000f41"
const QUEST_DAILY_1: String = "0193b0a0-0000-7000-8000-000000000d11"
const QUEST_DAILY_2: String = "0193b0a0-0000-7000-8000-000000000d12"
const QUEST_WEEKLY: String = "0193b0a0-0000-7000-8000-000000000d21"
const DAY_SEC: int = 86400

const ROUTE_SUGGESTIONS: String = "GET /social/friend-suggestions"
const ROUTE_CLUBS: String = "GET /social/clubs"
const ROUTE_CLUBS_MINE: String = "GET /social/clubs?scope=mine*"
const ROUTE_CLUBS_DISCOVER: String = "GET /social/clubs?scope=discoverable*"
const ROUTE_CLUB_CREATE: String = "POST /social/clubs"
const ROUTE_CLUB: String = "GET /social/clubs/*"
const ROUTE_CLUB_MEMBERS: String = "GET /social/clubs/*/members"
const ROUTE_CLUB_GOALS: String = "GET /social/clubs/*/goals"
const ROUTE_CLUB_EVENTS: String = "GET /social/clubs/*/events"
const ROUTE_CLUB_REQUESTS: String = "GET /social/clubs/*/join-requests"
const ROUTE_REQUEST_JOIN: String = "POST /social/clubs/*/join-requests"
const ROUTE_ACCEPT_REQUEST: String = "POST /social/club-join-requests/*:accept"
const ROUTE_REJECT_REQUEST: String = "POST /social/club-join-requests/*:reject"
const ROUTE_INVITE: String = "POST /social/clubs/*/invitations"
const ROUTE_INVITATIONS: String = "GET /social/club-invitations"
const ROUTE_ACCEPT_INVITATION: String = "POST /social/club-invitations/*:accept"
const ROUTE_DECLINE_INVITATION: String = "POST /social/club-invitations/*:decline"
const ROUTE_LEAVE: String = "POST /social/clubs/*:leave"
const ROUTE_EVENTS: String = "GET /game/events"
const ROUTE_EVENT: String = "GET /game/events/*"
const ROUTE_EVENT_JOIN: String = "POST /game/events/*:join"
const ROUTE_MILESTONE_CLAIM: String = "POST /game/events/*/milestones/*:claim"
const ROUTE_GOAL: String = "GET /game/community-goals/*"
const ROUTE_SEASON: String = "GET /game/season-track"
const ROUTE_TIER_CLAIM: String = "POST /game/season-track/tiers/*:claim"
const ROUTE_SEASON_HISTORY: String = "GET /game/season-track/history"
const ROUTE_QUESTS_DAILY: String = "GET /game/quests?quest_type=daily*"
const ROUTE_QUESTS_WEEKLY: String = "GET /game/quests?quest_type=weekly*"
const ROUTE_QUESTS: String = "GET /game/quests"


static func openapi(name: String) -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [OPENAPI, name])) as Dictionary).duplicate(true)


static func asyncapi(name: String) -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [ASYNCAPI, name])) as Dictionary).duplicate(true)


## The payload of the AsyncAPI sample `<type>` with `fields` merged in.
static func ws(type: String, fields: Dictionary) -> Dictionary:
	var p: Dictionary = asyncapi("%s.sample" % type)["payload"] as Dictionary
	p.merge(fields, true)
	return p


static func iso(unix_sec: int) -> String:
	return Time.get_datetime_string_from_unix_time(unix_sec, false) + "Z"


static func page(data: Array, more: bool = false, cursor: String = "") -> Dictionary:
	var p: Dictionary = {"has_more": more, "limit": 100}
	if more:
		p["next_cursor"] = cursor
	return {"data": data, "page": p}


# ---------- suggestions ----------

## `listFriendSuggestions` (the contract example + a party co-player): GauTruc22 (coop), CaVang15 (match), SoiNho31 (party).
static func suggestions() -> Dictionary:
	var body: Dictionary = openapi("listFriendSuggestions.200")
	var third: Dictionary = ((body["data"] as Array)[0] as Dictionary).duplicate(true)
	(third["player"] as Dictionary)["user_id"] = SUGGESTED_PARTY
	(third["player"] as Dictionary)["nickname"] = "SoiNho31"
	(third["player"] as Dictionary).erase("title")
	third["context_kind"] = "party"
	(body["data"] as Array).append(third)
	return body


static func suggestion(user_id: String, nick: String, context: String = "party") -> Dictionary:
	var s: Dictionary = (openapi("listFriendSuggestions.200")["data"] as Array)[0] as Dictionary
	(s["player"] as Dictionary)["user_id"] = user_id
	(s["player"] as Dictionary)["nickname"] = nick
	s["context_kind"] = context
	return s


# ---------- clubs ----------

static func club(id: String, name: String, club_type: String = "interest_group", status: String = "active",
		role: String = "member", members: int = 6) -> Dictionary:
	var c: Dictionary = openapi("Club")
	c["club_id"] = id
	c["club_type"] = club_type
	c["name"] = name
	c["description"] = "Cùng nhau khám phá và học vui mỗi tuần"
	c["icon_id"] = "club_icon_compass"
	c["status"] = status
	c["member_count"] = members
	c["max_members"] = 50
	c["chat_channel_id"] = CLUB_CHANNEL
	c["version"] = 1
	c.erase("subject_id")
	if role.is_empty():
		c.erase("my_role")
	else:
		c["my_role"] = role
	return c


## "Club của em": the school's CLB Toán (member), an own club (owner), a pending one and a locked one, plus a legacy
## study group the client must drop (ADR-0095).
static func mine() -> Dictionary:
	return page([club(CLUB_SCHOOL, "CLB Toán", "school_club", "active", "member", 24),
		club(CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "owner", 6),
		club(CLUB_PENDING, "Đội Robot", "competition_team", "pending_approval", "owner", 1),
		club(CLUB_LOCKED, "CLB Cờ vua", "interest_group", "locked", "member", 8),
		club(CLUB_STUDY, "Nhóm học cũ", "study_group", "active", "member", 4)])


static func discoverable() -> Dictionary:
	return page([club(CLUB_DISCOVER, "CLB Vẽ", "interest_group", "active", "", 12),
		club(CLUB_STUDY, "Nhóm học cũ", "study_group", "active", "", 4)])


static func invitations() -> Dictionary:
	var body: Dictionary = openapi("listMyClubInvitations.200")
	var inv: Dictionary = (body["data"] as Array)[0] as Dictionary
	inv["club_id"] = CLUB_INVITED
	inv["club_name"] = "CLB Âm nhạc"
	return body


static func member(user_id: String, nick: String, role: String = "member") -> Dictionary:
	var m: Dictionary = openapi("ClubMember")
	m["club_id"] = CLUB_OWN
	m["user_id"] = user_id
	m["nickname"] = nick
	m["role"] = role
	return m


static func members(self_id: String) -> Dictionary:
	return page([member(self_id, "MeoXanh42", "owner"), member(FRIEND_ONLINE, FRIEND_NICK, "officer"),
		member(MEMBER_OTHER, "GauNau15", "member")])


static func goals() -> Dictionary:
	var body: Dictionary = openapi("listClubGoals.200")
	((body["data"] as Array)[0] as Dictionary)["club_id"] = CLUB_OWN
	return body


static func club_event(title: String, offset_days: int, now_sec: int) -> Dictionary:
	var e: Dictionary = openapi("ClubEvent")
	e["club_id"] = CLUB_OWN
	e["title"] = title
	e["description"] = "Cả nhóm cùng làm bài luyện tập nhé!"
	e["starts_at"] = iso(now_sec + offset_days * DAY_SEC)
	e["ends_at"] = iso(now_sec + offset_days * DAY_SEC + 3600)
	e.erase("challenge_id")
	return e


static func club_events(now_sec: int) -> Dictionary:
	return page([club_event("Luyện tập Toán cuối tuần", 2, now_sec)])


static func join_request(id: String, nick: String, requester: String = MEMBER_OTHER) -> Dictionary:
	var r: Dictionary = openapi("ClubJoinRequest")
	r["request_id"] = id
	r["club_id"] = CLUB_OWN
	r["requester_id"] = requester
	r["requester_nickname"] = nick
	return r


static func join_requests() -> Dictionary:
	return page([join_request(REQUEST_ID, "ThoTrang03", FRIEND_OFFLINE), join_request(REQUEST_ID_2, "BuomXanh09",
			F110Fixtures.STRANGER)])


static func invited(invitee_id: String) -> Dictionary:
	var i: Dictionary = openapi("ClubInvitation")
	i["club_id"] = CLUB_OWN
	i["club_name"] = "Nhà thám hiểm nhí"
	i["inviter_nickname"] = "MeoXanh42"
	i["invitee_id"] = invitee_id
	return i


# ---------- live-ops events ----------

static func reward_xp(amount: int) -> Dictionary:
	return {"kind": "xp", "amount": amount}


static func reward_item(vi: String, en: String) -> Dictionary:
	return {"kind": "item", "item_id": "lantern_moon", "label": {"vi": vi, "en": en}}


static func summary(id: String, name: Dictionary, status: String, starts_sec: int, ends_sec: int,
		kind: String = "world_festival", scope: String = "platform") -> Dictionary:
	var e: Dictionary = openapi("GameEventSummary")
	e["event_id"] = id
	e["event_run_id"] = RUN_FESTIVAL if id == EVENT_FESTIVAL else id
	e["event_kind"] = kind
	e["owner_scope"] = scope
	e["name"] = name
	e["short_tagline"] = {"vi": "Rước đèn, giải đố cùng cả trường", "en": "Lanterns and riddles with the whole school"} \
			if id == EVENT_FESTIVAL else {"vi": "Cùng nhau học vui"}
	e["starts_at"] = iso(starts_sec)
	e["ends_at"] = iso(ends_sec)
	e["status"] = status
	e["joined"] = false
	e["event_points"] = 0
	e.erase("theme_code")
	return e


## `listGameEvents`: the festival (live, 5 days left), a school event named in Vietnamese only, an upcoming weekend.
static func events(now_sec: int) -> Dictionary:
	return page([
		summary(EVENT_FESTIVAL, {"vi": "Lễ hội Trăng Rằm", "en": "Mid-Autumn Festival"}, "live", now_sec - 2 * DAY_SEC,
				now_sec + 5 * DAY_SEC - 3600),
		summary(EVENT_SCHOOL, {"vi": "Tuần lễ đọc sách"}, "live", now_sec - DAY_SEC, now_sec + 3600, "school", "tenant"),
		summary(EVENT_UPCOMING, {"vi": "Cuối tuần khám phá", "en": "Discovery weekend"}, "scheduled", now_sec + 2 * DAY_SEC,
				now_sec + 4 * DAY_SEC, "weekend"),
	])


static func milestone(id: String, points: int, reached: bool, claimed: bool, rewards: Array) -> Dictionary:
	var m: Dictionary = openapi("EventMilestone")
	m["milestone_id"] = id
	m["points_required"] = points
	m["reached"] = reached
	m["claimed"] = claimed
	m["rewards"] = rewards
	return m


static func goal(progress: int = 6200, by_grade: Array = [], contributed: bool = true) -> Dictionary:
	var g: Dictionary = openapi("CommunityGoalProgress")
	g["goal_id"] = GOAL_ID
	g["event_run_id"] = RUN_FESTIVAL
	g["metric"] = "correct_answers"
	g["scope"] = "school"
	g["target"] = 10000
	g["progress"] = progress
	g["milestones_reached"] = [25, 50]
	g["my_contributed"] = contributed
	g["by_grade"] = by_grade
	return g


## `getGameEvent` of the festival: joined, 120 points, two milestones (the first reached, not claimed), the community goal.
static func detail(now_sec: int, joined: bool = true, by_grade: Array = []) -> Dictionary:
	var d: Dictionary = openapi("GameEventDetail")
	d.merge(summary(EVENT_FESTIVAL, {"vi": "Lễ hội Trăng Rằm", "en": "Mid-Autumn Festival"}, "live",
			now_sec - 2 * DAY_SEC, now_sec + 5 * DAY_SEC - 3600), true)
	d["joined"] = joined
	d["event_points"] = 120 if joined else 0
	d["description"] = {"vi": "Cùng rước đèn và giải câu đố để thắp sáng Quảng trường.",
		"en": "Carry lanterns and solve riddles to light up the square."}
	d["quest_ids"] = [EVENT_QUEST]
	for k: String in ["challenge_ids", "boss_ids", "shop_item_ids", "collection_id"]:
		d.erase(k)
	d["milestones"] = [milestone("m1", 100, true, false, [reward_xp(50)]),
		milestone("m2", 300, false, false, [reward_item("Đèn lồng trăng", "Moon lantern")])]
	d["community_goal"] = goal(6200, by_grade)
	d["daily_points_remaining"] = 80
	d["claim_deadline_at"] = iso(now_sec + 7 * DAY_SEC)
	return d


static func claim_result(rewards: Array) -> Dictionary:
	var r: Dictionary = openapi("RewardClaimResult")
	r["claimed"] = true
	r["rewards"] = rewards
	return r


static func event_quest() -> Dictionary:
	var q: Dictionary = openapi("QuestSummary")
	q["quest_id"] = EVENT_QUEST
	q["quest_type"] = "special_event"
	q["name"] = {"vi": "Thắp đèn lồng", "en": "Light the lanterns"}
	q["status"] = "available"
	q["can_accept"] = true
	q.erase("progress")
	q.erase("locked_reason")
	q.erase("missing_requirements")
	q.erase("retry_after")
	return q


# ---------- season track ----------

static func tier(n: int, reached: bool, claimed: bool) -> Dictionary:
	var t: Dictionary = openapi("SeasonTrackTier")
	t["tier"] = n
	t["points_required"] = (n - 1) * 100
	t["reached"] = reached
	t["claimed"] = claimed
	t["milestone"] = n % 10 == 0
	t["rewards"] = [reward_xp(20 + n)] if n % 10 != 0 else [reward_item("Huy hiệu mùa", "Season badge")]
	return t


## `getSeasonTrack`: 30 tiers, tier 3 reached (tiers 1–2 claimed? no: only tier 1 claimed), 250 points.
static func season(tiers: int = 30) -> Dictionary:
	var s: Dictionary = openapi("SeasonTrack")
	s["season_id"] = SEASON_ID
	s["season_name"] = {"vi": "Mùa Khám phá", "en": "Discovery Season"}
	s["points"] = 250
	s["tier"] = 3
	s["max_tier"] = tiers
	s["points_per_tier"] = 100
	s["daily_points_remaining"] = 60
	var list: Array = []
	for n: int in range(1, tiers + 1):
		list.append(tier(n, n <= 3, n == 1))
	s["tiers"] = list
	return s


static func history() -> Dictionary:
	var h: Dictionary = openapi("SeasonTrackHistoryItem")
	h["season_name"] = {"vi": "Mùa Hè Vui", "en": "Fun Summer Season"}
	h["final_tier"] = 12
	h["points"] = 1180
	h["auto_granted_tiers"] = [11, 12]
	return page([h])


# ---------- daily / weekly quests ----------

static func quest(id: String, quest_type: String, vi: String, en: String, status: String, until_sec: int,
		done_steps: int = 0, steps: int = 2) -> Dictionary:
	var q: Dictionary = openapi("QuestSummary")
	q["quest_id"] = id
	q["quest_type"] = quest_type
	q["name"] = {"vi": vi, "en": en}
	q["status"] = status
	q["can_accept"] = status == "available"
	q["available_until"] = iso(until_sec)
	for k: String in ["locked_reason", "missing_requirements", "retry_after", "subject_id", "story_chapter_code"]:
		q.erase(k)
	if status == "available":
		q.erase("progress")
	else:
		var p: Dictionary = q["progress"] as Dictionary
		p["quest_id"] = id
		p["status"] = status
		var states: Dictionary = {}
		for i: int in steps:
			states["s%d" % (i + 1)] = {"status": "completed" if i < done_steps else "active"}
		p["step_states"] = states
		p.erase("party_progress")
	return q


static func daily(now_sec: int, all_done: bool = false) -> Dictionary:
	var midnight: int = now_sec + DAY_SEC
	if all_done:
		return page([quest(QUEST_DAILY_1, "daily", "Trả lời 5 câu Toán", "Answer 5 maths questions", "completed", midnight, 2),
			quest(QUEST_DAILY_2, "daily", "Ghé thăm Rừng Ngôn Ngữ", "Visit the Language Forest", "completed", midnight, 2)])
	return page([quest(QUEST_DAILY_1, "daily", "Trả lời 5 câu Toán", "Answer 5 maths questions", "active", midnight, 1),
		quest(QUEST_DAILY_2, "daily", "Ghé thăm Rừng Ngôn Ngữ", "Visit the Language Forest", "available", midnight)])


static func weekly(now_sec: int) -> Dictionary:
	return page([quest(QUEST_WEEKLY, "weekly", "Học 4 ngày trong tuần", "Learn on 4 days this week", "active",
			now_sec + 4 * DAY_SEC, 4, 7)])


## Every F111 route with its default reply (`self_id` = the signed-in student, `now_sec` = server time).
static func script(http: FakeHttp, self_id: String, now_sec: int) -> void:
	http.reply(ROUTE_SUGGESTIONS, 200, suggestions())
	http.reply(ROUTE_CLUBS_MINE, 200, mine())
	http.reply(ROUTE_CLUBS_DISCOVER, 200, discoverable())
	http.reply(ROUTE_INVITATIONS, 200, invitations())
	# The sub-resources first: ROUTE_CLUB (`/social/clubs/*`) would also match `/social/clubs/<id>/members`.
	http.reply(ROUTE_CLUB_MEMBERS, 200, members(self_id))
	http.reply(ROUTE_CLUB_GOALS, 200, goals())
	http.reply(ROUTE_CLUB_EVENTS, 200, club_events(now_sec))
	http.reply(ROUTE_CLUB_REQUESTS, 200, join_requests())
	http.reply(ROUTE_CLUB, 200, club(CLUB_OWN, "Nhà thám hiểm nhí", "interest_group", "active", "owner", 6))
	http.reply(ROUTE_REQUEST_JOIN, 201, join_request(REQUEST_ID, "MeoXanh42", self_id))
	http.reply(ROUTE_ACCEPT_REQUEST, 200, member(FRIEND_OFFLINE, "ThoTrang03"))
	http.reply(ROUTE_REJECT_REQUEST, 204)
	http.reply(ROUTE_INVITE, 201, {"data": [invited(FRIEND_ONLINE)]})
	http.reply(ROUTE_ACCEPT_INVITATION, 200, member(self_id, "MeoXanh42"))
	http.reply(ROUTE_DECLINE_INVITATION, 204)
	http.reply(ROUTE_LEAVE, 204)
	http.reply(ROUTE_CLUB_CREATE, 201, club(CLUB_PENDING, "Đội Robot", "competition_team", "pending_approval", "owner", 1))
	http.reply(ROUTE_EVENTS, 200, events(now_sec))
	http.reply(ROUTE_EVENT, 200, detail(now_sec))
	http.reply(ROUTE_EVENT_JOIN, 200, {"event_run_id": RUN_FESTIVAL, "joined_at": iso(now_sec), "event_points": 0})
	http.reply(ROUTE_MILESTONE_CLAIM, 200, claim_result([reward_xp(50)]))
	http.reply(ROUTE_GOAL, 200, goal())
	http.reply(ROUTE_SEASON, 200, season())
	http.reply(ROUTE_TIER_CLAIM, 200, claim_result([reward_xp(22)]))
	http.reply(ROUTE_SEASON_HISTORY, 200, history())
	http.reply(ROUTE_QUESTS_DAILY, 200, daily(now_sec))
	http.reply(ROUTE_QUESTS_WEEKLY, 200, weekly(now_sec))

