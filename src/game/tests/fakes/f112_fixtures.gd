class_name F112Fixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): the F112 test data, built ONLY from the generated contract fixtures
## (`tests/fixtures/contract/`, tools/gen_fixtures.mjs): `listMyChannels` with its party / match / team / direct channels
## plus a `world`, an `area`, the team channel of a second club and a `direct` channel whose friend the client does not
## know (copies of the example channels with another id / type — the contract has one example per type); the clubs of
## F111Fixtures each with its own `chat_channel_id`; `openDirectChannel`, `searchMessages`, `updateClub`,
## `updateClubMemberRole`, `createClubGoal`, `createClubEvent` and their problems. Routes for FakeHttp.

const OPENAPI: String = "res://tests/fixtures/contract/openapi"
const TEAM_SCHOOL: String = ChatFixtures.TEAM_CHANNEL
const DM_KNOWN: String = "0193b0a0-0000-7000-8000-000000000503"
const DM_NEW: String = "0193b0a0-0000-7000-8000-000000000504"
const WORLD: String = "0193b0a0-0000-7000-8000-000000000505"
const AREA: String = "0193b0a0-0000-7000-8000-000000000506"
const TEAM_OWN: String = "0193b0a0-0000-7000-8000-000000000507"
const DM_UNKNOWN: String = "0193b0a0-0000-7000-8000-000000000508"
const TEAM_LOCKED: String = "0193b0a0-0000-7000-8000-000000000509"
const AREA_NEXT: String = "0193b0a0-0000-7000-8000-00000000050a"
## Friends of `listFriends.200` (F110): CaHeoVui07 (the peer of DM_KNOWN), SaoBang12, ThoTrang03.
const FRIEND_DM: String = F110Fixtures.FRIEND_ONLINE
const FRIEND_NEW: String = "0193b0a0-0000-7000-8000-0000000000a4"
const FRIEND_OTHER: String = F110Fixtures.FRIEND_OFFLINE
const ROUTE_CHANNELS: String = ChatFixtures.ROUTE_CHANNELS
const ROUTE_MESSAGES: String = ChatFixtures.ROUTE_MESSAGES
const ROUTE_DIRECT: String = "POST /chat/direct-channels"
const ROUTE_SEARCH: String = "GET /chat/search"
const ROUTE_CLUB_PATCH: String = "PATCH /social/clubs/*"
const ROUTE_MEMBER_PATCH: String = "PATCH /social/clubs/*/members/*"
const ROUTE_GOAL_CREATE: String = "POST /social/clubs/*/goals"
const ROUTE_EVENT_CREATE: String = "POST /social/clubs/*/events"
const ROUTE_CLUBS_MINE: String = F111Fixtures.ROUTE_CLUBS_MINE
const ROUTE_CLUB: String = F111Fixtures.ROUTE_CLUB
const ROUTE_CLUB_MEMBERS: String = F111Fixtures.ROUTE_CLUB_MEMBERS
const ROUTE_CLUB_GOALS: String = F111Fixtures.ROUTE_CLUB_GOALS


static func openapi(name: String) -> Dictionary:
	return JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [OPENAPI, name])) as Dictionary


## The example channel of `channel_type` in `listMyChannels.200`.
static func example_channel(channel_type: String) -> Dictionary:
	for c: Variant in openapi("listMyChannels.200")["data"] as Array:
		if String((c as Dictionary)["channel_type"]) == channel_type:
			return (c as Dictionary).duplicate(true)
	return {}


## A persistent channel copied from an example: `id`, `channel_type`, `status`, unread / mentions.
static func channel(id: String, channel_type: String, status: String = "open", unread: int = 0, mentions: int = 0,
		from_type: String = "direct") -> Dictionary:
	var c: Dictionary = example_channel(from_type)
	c["channel_id"] = id
	c["channel_type"] = channel_type
	c["status"] = status
	c["unread_count"] = unread
	c["unread_mentions"] = mentions
	c["last_seq"] = 5
	c["my_read_seq"] = 5
	if channel_type != "direct":
		c.erase("peer")
	# The features of the example party channel (every feature on); World / Area narrow them (CHAT_SYSTEM §7.1).
	c["features"] = (example_channel("party").get("features", {}) as Dictionary).duplicate()
	if channel_type in ["world", "area"]:
		c.erase("muted")
		c.erase("notification_level")
		var f: Dictionary = c["features"] as Dictionary
		for k: String in ["forward", "pin", "typing", "search"]:
			f[k] = false
		f["receipts"] = "none"
		c["features"] = f
	return c


## `listMyChannels`: the examples (party, match, team of CLB Toán, DM with CaHeoVui07) + World, Area, the team channel
## of the own club (search on), a DM whose friend is unknown (2 unread) and the locked club (`read_only`).
static func channels(extra: Array = []) -> Dictionary:
	var body: Dictionary = openapi("listMyChannels.200")
	var list: Array = body["data"] as Array
	for c: Variant in list:
		var d: Dictionary = c as Dictionary
		if String(d["channel_type"]) == "team":
			d["unread_count"] = 3
			var f: Dictionary = example_channel("party").get("features", {}) as Dictionary
			d["features"] = f.duplicate()
		if String(d["channel_type"]) == "direct":
			d["unread_count"] = 1
			d["features"] = (example_channel("party").get("features", {}) as Dictionary).duplicate()
	list.append(channel(WORLD, "world", "open", 7, 1, "direct"))
	list.append(channel(AREA, "area", "open", 4, 0, "direct"))
	list.append(channel(TEAM_OWN, "team", "open", 0, 0, "direct"))
	list.append(channel(DM_UNKNOWN, "direct", "open", 2, 0, "direct"))
	(list[list.size() - 1] as Dictionary).erase("peer")
	list.append(channel(TEAM_LOCKED, "team", "read_only", 0, 0, "direct"))
	for e: Variant in extra:
		list.append(e)
	return body


## Club list `scope = mine` (F111) with one chat channel per club.
static func mine() -> Dictionary:
	var body: Dictionary = F111Fixtures.mine()
	var ids: Dictionary = {F111Fixtures.CLUB_SCHOOL: TEAM_SCHOOL, F111Fixtures.CLUB_OWN: TEAM_OWN,
		F111Fixtures.CLUB_LOCKED: TEAM_LOCKED}
	for c: Variant in body["data"] as Array:
		var d: Dictionary = c as Dictionary
		d["chat_channel_id"] = ids.get(String(d["club_id"]), "")
		if String(d["chat_channel_id"]).is_empty():
			d.erase("chat_channel_id")
	return body


## `openDirectChannel` 201 for `user_id` (the example with that friend as peer).
static func direct_created(user_id: String, nickname: String, status: String = "201") -> Dictionary:
	var c: Dictionary = openapi("openDirectChannel.%s" % status)
	c["channel_id"] = DM_NEW
	var peer: Dictionary = (c.get("peer", {}) as Dictionary).duplicate(true)
	peer["user_id"] = user_id
	peer["nickname"] = nickname
	c["peer"] = peer
	return c


## `searchMessages`: the example hit + one with markup-looking text in the body (no `highlight`) + one in a DM.
static func search() -> Dictionary:
	var body: Dictionary = openapi("searchMessages.200")
	var list: Array = body["data"] as Array
	var first: Dictionary = list[0] as Dictionary
	var tagged: Dictionary = first.duplicate(true)
	var m: Dictionary = tagged["message"] as Dictionary
	m["message_id"] = "0193b0a0-0000-7000-8000-000000000622"
	m["seq"] = 3
	m["body"] = "Hẹn [b]đi boss[/b] lúc 19h <em>nhé</em>"
	tagged.erase("highlight")
	tagged.erase("highlight_ranges")
	list.append(tagged)
	var dm: Dictionary = first.duplicate(true)
	(dm["message"] as Dictionary)["message_id"] = "0193b0a0-0000-7000-8000-000000000623"
	(dm["message"] as Dictionary)["channel_id"] = DM_KNOWN
	dm["channel_id"] = DM_KNOWN
	dm["channel_type"] = "direct"
	list.append(dm)
	return body


static func goals_with_metrics() -> Dictionary:
	return openapi("listClubGoals.200")


## Every F112 route answered with its contract example (call after F111Fixtures.script).
static func script(http: FakeHttp) -> void:
	http.clear(ROUTE_CHANNELS).reply(ROUTE_CHANNELS, 200, channels())
	http.clear(ROUTE_CLUBS_MINE).reply(ROUTE_CLUBS_MINE, 200, mine())
	http.reply(ROUTE_DIRECT, 201, direct_created(FRIEND_NEW, "SaoBang12"))
	http.reply(ROUTE_SEARCH, 200, search())
	http.reply(ROUTE_MEMBER_PATCH, 200, openapi("updateClubMemberRole.200"))
	http.reply(ROUTE_CLUB_PATCH, 200, openapi("updateClub.200"))
	http.reply(ROUTE_GOAL_CREATE, 201, openapi("createClubGoal.201"))
	http.reply(ROUTE_EVENT_CREATE, 201, openapi("createClubEvent.201"))
