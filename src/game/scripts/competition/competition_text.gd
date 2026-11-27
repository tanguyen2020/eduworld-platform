class_name CompetitionText
extends RefCounted
## Translation keys of the arena screens (F109-AC-30; `ui.arena.*`, `ui.leaderboard.*`, `ui.tournament.*`): every key
## is written out here (extract_pot finds literal keys only), the screens look them up by code. A problem code outside
## the lists shows the generic friendly sentence, never the code (F109-AC-04, AC-07, AC-24, AC-27).

const KEY_GENERIC_ERROR: String = "ui.arena.error.generic"
## REST / realtime problem code → friendly sentence. 404 everywhere = "Không tìm thấy" (F109-AC-29).
const ERROR_KEYS: Dictionary = {
	"MM_TICKET_ALREADY_ACTIVE": "ui.arena.error.MM_TICKET_ALREADY_ACTIVE",
	"POLICY_DENIED": "ui.arena.error.POLICY_DENIED",
	"PARTY_NOT_LEADER": "ui.arena.error.PARTY_NOT_LEADER",
	"PARTY_MEMBER_NOT_ONLINE": "ui.arena.error.PARTY_MEMBER_NOT_ONLINE",
	"ANSWER_LATE": "ui.arena.error.ANSWER_LATE",
	"ANSWER_ALREADY_SUBMITTED": "ui.arena.error.ANSWER_ALREADY_SUBMITTED",
	"MATCH_NOT_ACTIVE": "ui.arena.error.MATCH_NOT_ACTIVE",
	"PROPOSAL_EXPIRED": "ui.arena.error.PROPOSAL_EXPIRED",
	"DUEL_EXPIRED": "ui.arena.error.DUEL_EXPIRED",
	"HTTP_410": "ui.arena.error.DUEL_EXPIRED",
	"SOCIAL_SCOPE_VIOLATION": "ui.arena.error.SOCIAL_SCOPE_VIOLATION",
	"RATE_LIMITED": "ui.arena.error.RATE_LIMITED",
	"CONFLICT": "ui.arena.error.CONFLICT",
	"NOT_FOUND": "ui.arena.error.NOT_FOUND",
	"HTTP_404": "ui.arena.error.NOT_FOUND",
	"UNAVAILABLE": "ui.arena.error.UNAVAILABLE",
	"NETWORK_ERROR": "ui.arena.error.NETWORK_ERROR",
	"OFFLINE": "ui.arena.error.NETWORK_ERROR",
	"TIMEOUT": "ui.arena.error.NETWORK_ERROR",
}
## Tournament problems (F109-AC-24; server delta 2026-11-27): the four `TOURNAMENT_*` codes show their own
## `ui.error.<CODE>` sentence; outside the audience is still 404, an entry already made is `409 CONFLICT`, and
## `INVALID_STATE_TRANSITION` (organiser path / older server) reads as "not now".
const TOURNAMENT_ERROR_KEYS: Dictionary = {
	"TOURNAMENT_NOT_ELIGIBLE": "ui.error.TOURNAMENT_NOT_ELIGIBLE",
	"TOURNAMENT_REGISTRATION_CLOSED": "ui.error.TOURNAMENT_REGISTRATION_CLOSED",
	"TOURNAMENT_FULL": "ui.error.TOURNAMENT_FULL",
	"TOURNAMENT_CHECK_IN_CLOSED": "ui.error.TOURNAMENT_CHECK_IN_CLOSED",
	"POLICY_DENIED": "ui.tournament.error.not_eligible",
	"NOT_FOUND": "ui.tournament.error.not_eligible",
	"HTTP_404": "ui.tournament.error.not_eligible",
	"CONFLICT": "ui.tournament.error.already_registered",
}
## `battle.match.ended.encouragement.code` → the client sentence (the server `message_code`
## `competition.encouragement.<code>` is the client key `ui.competition.encouragement.<code>`, as notifications read
## `ui.notification.<code>`); `{subject}` / `{topic}` are the names of the match's subject and topic. Only listed codes
## are shown — a server string never becomes a key by itself.
const ENCOURAGEMENT_KEYS: Dictionary = {
	"loss_streak_review": "ui.competition.encouragement.loss_streak_review",
}
## The same sentence when the match had no topic (subject only).
const ENCOURAGEMENT_SUBJECT_KEYS: Dictionary = {
	"loss_streak_review": "ui.competition.encouragement.loss_streak_review.subject",
}
## An encouragement code this client does not know yet: the kind general line.
const KEY_ENCOURAGEMENT_GENERIC: String = "ui.arena.result.encouragement"
const MODE_KEYS: Dictionary = {
	"battle_1v1": "ui.arena.mode.battle_1v1", "team_battle": "ui.arena.mode.team_battle",
	"coop_challenge": "ui.arena.mode.coop_challenge", "timed_quiz": "ui.arena.mode.timed_quiz",
	"tournament": "ui.arena.mode.tournament",
}
const MODE_HINT_KEYS: Dictionary = {
	"battle_1v1": "ui.arena.mode.battle_1v1.hint", "team_battle": "ui.arena.mode.team_battle.hint",
	"coop_challenge": "ui.arena.mode.coop_challenge.hint", "timed_quiz": "ui.arena.mode.timed_quiz.hint",
	"tournament": "ui.arena.mode.tournament.hint",
}
const MODE_ICONS: Dictionary = {
	"battle_1v1": "arena", "team_battle": "group", "coop_challenge": "party", "timed_quiz": "clock",
	"tournament": "bracket",
}
## Outcome words — encouraging, never shaming (NFR-SAFE-003): a loss is "Chưa thắng lần này".
const OUTCOME_KEYS: Dictionary = {
	"win": "ui.arena.result.win", "loss": "ui.arena.result.not_won", "draw": "ui.arena.result.draw",
	"abandoned": "ui.arena.result.abandoned", "forfeit": "ui.arena.result.abandoned",
	"no_contest": "ui.arena.result.no_contest",
}
const OUTCOME_ICONS: Dictionary = {
	"win": "emoji_trophy", "loss": "emoji_muscle", "draw": "emoji_handshake", "abandoned": "info", "forfeit": "info",
	"no_contest": "info",
}
const TOURNAMENT_STATUS_KEYS: Dictionary = {
	"published": "ui.tournament.status.published", "registration_open": "ui.tournament.status.registration_open",
	"registration_closed": "ui.tournament.status.registration_closed", "seeding": "ui.tournament.status.seeding",
	"scheduled": "ui.tournament.status.scheduled", "in_progress": "ui.tournament.status.in_progress",
	"completed": "ui.tournament.status.completed", "rewarded": "ui.tournament.status.completed",
	"archived": "ui.tournament.status.completed", "cancelled": "ui.tournament.status.cancelled",
	"draft": "ui.tournament.status.published",
}
const ENTRY_STATUS_KEYS: Dictionary = {
	"registered": "ui.tournament.entry.registered", "confirmed": "ui.tournament.entry.confirmed",
	"checked_in": "ui.tournament.entry.checked_in", "withdrawn": "ui.tournament.entry.withdrawn",
	"disqualified": "ui.tournament.entry.withdrawn", "eliminated": "ui.tournament.entry.eliminated",
	"champion": "ui.tournament.entry.champion",
}
const NODE_STATUS_KEYS: Dictionary = {
	"pending": "ui.tournament.node.pending", "scheduled": "ui.tournament.node.scheduled",
	"in_match": "ui.tournament.node.in_match", "awaiting_confirm": "ui.tournament.node.awaiting_confirm",
	"confirmed": "ui.tournament.node.confirmed", "overridden": "ui.tournament.node.confirmed",
	"bye": "ui.tournament.node.bye", "void": "ui.tournament.node.void",
}


static func error_key(code: String) -> String:
	return String(ERROR_KEYS.get(code, KEY_GENERIC_ERROR))


static func tournament_error_key(code: String) -> String:
	if TOURNAMENT_ERROR_KEYS.has(code):
		return String(TOURNAMENT_ERROR_KEYS[code])
	if code == "INVALID_STATE_TRANSITION":
		return "ui.tournament.error.window_closed"
	return error_key(code)


## The sentence of `encouragement` (`{code, message_code, subject_id, topic_id}`); `with_topic` = a topic name is
## known. The `message_code` is used only when it names a listed sentence.
static func encouragement_key(encouragement: Dictionary, with_topic: bool) -> String:
	var code: String = String(encouragement.get("code", ""))
	var message_code: String = String(encouragement.get("message_code", ""))
	if not ENCOURAGEMENT_KEYS.has(code):
		for known: Variant in ENCOURAGEMENT_KEYS.keys():
			if "ui." + message_code == String(ENCOURAGEMENT_KEYS[known]):
				code = String(known)
	if not ENCOURAGEMENT_KEYS.has(code):
		return KEY_ENCOURAGEMENT_GENERIC
	return String(ENCOURAGEMENT_KEYS[code]) if with_topic else String(ENCOURAGEMENT_SUBJECT_KEYS[code])


static func mode_key(mode: String) -> String:
	return String(MODE_KEYS.get(mode, "ui.arena.mode.other"))


static func outcome_key(outcome: String) -> String:
	return String(OUTCOME_KEYS.get(outcome, "ui.arena.result.no_contest"))


## "0:24" style wait time (never a ticking pressure: shown while queueing only).
static func clock(ms: int) -> String:
	var sec: int = maxi(0, ms / 1000)
	return "%d:%02d" % [sec / 60, sec % 60]
