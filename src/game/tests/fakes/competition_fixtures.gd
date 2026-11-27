class_name CompetitionFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): the F109 contract fixtures of tools/gen_fixtures.mjs — OpenAPI response examples of the
## arena (modes, subjects, topics, ticket, robot, tiers, matches, leaderboards per visibility, tournaments, bracket,
## entry, grade challenges, duels) and the AsyncAPI `matchmaking.*`, `battle.*`, `competition.*`, `tournament.*`
## examples — with the routes of the competition services. Invented nicknames only (R16). `as_self` makes MeoXanh42
## (a1, the student of the examples) the signed-in student.

const OPENAPI: String = "res://tests/fixtures/contract/openapi"
const SELF_SAMPLE: String = "0193b0a0-0000-7000-8000-0000000000a1"
const OPPONENT: String = "0193b0a0-0000-7000-8000-0000000000a2"
const MATCH_ID: String = "0193b0a0-0000-7000-8000-000000000501"
const TEAM_MATCH_ID: String = "0193b0a0-0000-7000-8000-000000000502"
const QUIZ_MATCH_ID: String = "0193b0a0-0000-7000-8000-000000000503"
const TICKET_ID: String = "0193b0a0-0000-7000-8000-000000000401"
const MATH: String = "0193b0a0-0000-7000-8000-000000000601"
const LB_GOAL: String = "0193b0a0-0000-7000-8000-000000000801"
const LB_POINTS: String = "0193b0a0-0000-7000-8000-000000000802"
const TOURNAMENT_ID: String = "0193b0a0-0000-7000-8000-000000000901"
const QUIZ_CHALLENGE: String = "0193b0a0-0000-7000-8000-000000000905"
const DUEL_ID: String = "0193b0a0-0000-7000-8000-000000000a01"
const ROUTE_MODES: String = "GET /competition/modes"
const ROUTE_SUBJECTS: String = "GET /learning/subjects*"
const ROUTE_TOPICS: String = "GET /learning/topics*"
const ROUTE_TICKET_CREATE: String = "POST /competition/matchmaking/tickets"
const ROUTE_TICKET_GET: String = "GET /competition/matchmaking/tickets/*"
const ROUTE_TICKET_DELETE: String = "DELETE /competition/matchmaking/tickets/*"
const ROUTE_BOT: String = "POST /competition/matchmaking/tickets/*:bot-match"
const ROUTE_ACCEPT: String = "POST /competition/matches/*/proposal:accept"
const ROUTE_DECLINE: String = "POST /competition/matches/*/proposal:decline"
const ROUTE_RATINGS: String = "GET /competition/ratings/me*"
const ROUTE_MATCHES: String = "GET /competition/matches"
const ROUTE_MATCH: String = "GET /competition/matches/*"
const ROUTE_BOARDS: String = "GET /leaderboards"
const ROUTE_BOARD: String = "GET /leaderboards/*"
const ROUTE_TOURNAMENTS: String = "GET /competition/tournaments"
const ROUTE_TOURNAMENT: String = "GET /competition/tournaments/*"
const ROUTE_BRACKET: String = "GET /competition/tournaments/*/bracket"
const ROUTE_REGISTER: String = "POST /competition/tournaments/*/entries"
const ROUTE_WITHDRAW: String = "DELETE /competition/tournaments/*/entries/*"
const ROUTE_CHECK_IN: String = "POST /competition/tournaments/*/entries/*:check-in"
const ROUTE_CHALLENGES: String = "GET /competition/challenges*"
const ROUTE_JOIN: String = "POST /competition/challenges/*:join"
const ROUTE_DUEL_CREATE: String = "POST /competition/duels"
const ROUTE_DUEL_ACCEPT: String = "POST /competition/duels/*:accept"
const ROUTE_DUEL_DECLINE: String = "POST /competition/duels/*:decline"
const ROUTE_REPORT: String = "POST /chat/reports"
const ROUTE_BLOCK: String = "PUT /social/blocks/*"


static func openapi(name: String, self_id: String = "") -> Dictionary:
	var raw: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [OPENAPI, name])) as Dictionary
	return as_self(raw.duplicate(true), self_id) as Dictionary


static func as_self(value: Variant, self_id: String) -> Variant:
	if self_id.is_empty() or self_id == SELF_SAMPLE:
		return value
	return JSON.parse_string(JSON.stringify(value).replace(SELF_SAMPLE, self_id))


## Payload of the AsyncAPI example `<type>.<name>` (e.g. "battle.state.snapshot", "question_1v1").
static func ws(type: String, name: String, self_id: String = "") -> Dictionary:
	var env: Dictionary = RealtimeHarness.fixture("%s.%s" % [type, name])
	return as_self(env["payload"], self_id) as Dictionary


## Every read route of the arena scripted on `http` (the signed-in student = `self_id`).
static func script(http: FakeHttp, self_id: String = "") -> void:
	for route: String in [ROUTE_MODES, ROUTE_SUBJECTS, ROUTE_TOPICS, ROUTE_RATINGS, ROUTE_MATCHES, ROUTE_MATCH, ROUTE_BOARDS,
			ROUTE_BOARD, ROUTE_TOURNAMENTS, ROUTE_BRACKET, ROUTE_TOURNAMENT, ROUTE_CHALLENGES, ROUTE_TICKET_GET]:
		http.clear(route)
	http.reply(ROUTE_MODES, 200, openapi("listCompetitionModes.200"))
	http.reply(ROUTE_SUBJECTS, 200, openapi("listSubjects.200"))
	http.reply(ROUTE_TOPICS, 200, openapi("listTopics.200"))
	http.reply(ROUTE_RATINGS, 200, openapi("getMyRatings.200"))
	http.reply(ROUTE_MATCHES, 200, openapi("listMyMatches.200", self_id))
	http.reply(ROUTE_MATCH, 200, openapi("getMatch.200", self_id))
	http.reply(ROUTE_BOARDS, 200, openapi("listLeaderboards.200"))
	http.reply(ROUTE_BOARD, 200, openapi("getLeaderboard.200.top_n_percentile", self_id))
	http.reply(ROUTE_TOURNAMENTS, 200, openapi("listTournaments.200", self_id))
	http.reply(ROUTE_BRACKET, 200, openapi("getTournamentBracket.200", self_id))
	http.reply(ROUTE_TOURNAMENT, 200, openapi("getTournament.200", self_id))
	http.reply(ROUTE_CHALLENGES, 200, openapi("listMyChallenges.200"))
	http.reply(ROUTE_TICKET_GET, 200, openapi("getMatchmakingTicket.200"))


## A leaderboard page example: "grade_goal", "top_n_percentile" or "ranked".
static func board(visibility: String, self_id: String = "") -> Dictionary:
	return openapi("getLeaderboard.200.%s" % visibility, self_id)
