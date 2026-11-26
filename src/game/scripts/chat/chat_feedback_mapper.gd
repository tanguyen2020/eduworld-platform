class_name ChatFeedbackMapper
extends RefCounted
## Server decisions → the friendly sentences of CHAT_SYSTEM §4.3 (F119-AC-10, AC-11; AC-UX-106). Pure and static: the
## input is only an error code, a `category_code`, a moderation state or a sanction — never the text the student wrote,
## so a sentence can never repeat a matched word, its position or a catalog entry (AGENTS R15, ADR-0030). The client
## decides nothing: the server already blocked, masked or held the message (CHILD_SAFETY §6).
## Output: `{key, args, tone}` — `tone` picks the look of the notice (`info`, `warning`, `care`).

const TONE_INFO: String = "info"
const TONE_WARNING: String = "warning"
## Care message of `block_and_alert` (self-harm, grooming): stays until the student closes it, suggests a trusted adult.
const TONE_CARE: String = "care"

const KEY_MASKED: String = "ui.chat.feedback.masked"
const KEY_PII: String = "ui.chat.feedback.pii_blocked"
const KEY_TERM: String = "ui.chat.feedback.term_blocked"
const KEY_HELD: String = "ui.chat.feedback.held"
const KEY_CARE: String = "ui.chat.feedback.care"
const KEY_RATE: String = "ui.chat.feedback.rate_limited"
const KEY_MUTED: String = "ui.chat.feedback.muted"
const KEY_MUTED_UNTIL: String = "ui.chat.muted_until"
const KEY_RESTRICT_TEXT: String = "ui.chat.feedback.restrict_text"
const KEY_RECONNECTING: String = "ui.chat.reconnecting"
const KEY_GENERIC: String = "ui.chat.feedback.generic"
## `category_code` of the PII step (MODERATION §3.3) — always the PII sentence.
const PII_CODES: PackedStringArray = ["pii", "pii_pattern"]
## Categories decided `block_and_alert` (MODERATION §11.2 critical) — the care sentence.
const CARE_CODES: PackedStringArray = ["self_harm", "grooming_scam"]
const REASON_BLOCK_AND_ALERT: String = "block_and_alert_hold"
## Command / problem code → sentence key (codes not listed show the generic sentence, never the raw code).
const CODE_KEYS: Dictionary = {
	"CHAT_RATE_LIMITED": KEY_RATE,
	"RATE_LIMITED": KEY_RATE,
	"CHAT_MUTED": KEY_MUTED,
	"CHAT_CHANNEL_CLOSED": "ui.chat.feedback.channel_closed",
	"CHAT_KIND_NOT_ALLOWED": "ui.chat.feedback.kind_not_allowed",
	"CHAT_NOT_ALLOWED": "ui.chat.feedback.not_allowed",
	"CURFEW_ACTIVE": "ui.chat.feedback.curfew",
	"CHAT_EDIT_WINDOW_EXPIRED": "ui.chat.feedback.edit_expired",
	"CHAT_RECALL_WINDOW_EXPIRED": "ui.chat.feedback.recall_expired",
	"CHAT_FORWARD_NOT_ALLOWED": "ui.chat.feedback.forward_not_allowed",
	"CHAT_PIN_LIMIT_REACHED": "ui.chat.feedback.pin_limit",
	"CHAT_MENTION_LIMIT": "ui.chat.feedback.mention_limit",
	"CHAT_REACTION_LIMIT": "ui.chat.feedback.reaction_limit",
	"CHAT_FEATURE_DISABLED": "ui.chat.feedback.feature_disabled",
	"CHAT_ASSET_NOT_AVAILABLE": "ui.chat.feedback.asset_locked",
	"PERMISSION_DENIED": "ui.chat.feedback.not_allowed",
	"VERSION_MISMATCH": "ui.chat.feedback.edit_conflict",
	"CONFLICT": "ui.chat.feedback.edit_conflict",
	"CHAT_TEMPORARILY_UNAVAILABLE": KEY_RECONNECTING,
	"UNAVAILABLE": KEY_RECONNECTING,
	"OFFLINE": KEY_RECONNECTING,
	"TIMEOUT": KEY_RECONNECTING,
	"NETWORK_ERROR": KEY_RECONNECTING,
}
const WARNING_CODES: PackedStringArray = ["CHAT_RATE_LIMITED", "RATE_LIMITED", "CHAT_MUTED", "CURFEW_ACTIVE"]


## A refused command / REST problem. `category_code` only matters for `CHAT_CONTENT_BLOCKED` (451).
static func for_code(code: String, category_code: String = "") -> Dictionary:
	if code == "CHAT_CONTENT_BLOCKED":
		if category_code in PII_CODES:
			return _out(KEY_PII, {}, TONE_WARNING)
		if category_code in CARE_CODES:
			return _out(KEY_CARE, {}, TONE_CARE)
		return _out(KEY_TERM, {}, TONE_WARNING)
	if code == "CHAT_CONTENT_HELD":
		return for_held()
	var key: String = String(CODE_KEYS.get(code, KEY_GENERIC))
	return _out(key, {}, TONE_WARNING if code in WARNING_CODES else TONE_INFO)


## Acknowledged with `moderation_state = masked`: the student sees the stars and a kind hint.
static func for_masked() -> Dictionary:
	return _out(KEY_MASKED, {}, TONE_INFO)


## `chat.message.held` (hold_for_review): the message waits for the Safety Team.
static func for_held() -> Dictionary:
	return _out(KEY_HELD, {}, TONE_INFO)


## `chat.sanction.applied`: `block_and_alert_hold` gets the care sentence; a mute shows when writing opens again
## (`time_text` already formatted in the viewer's locale, "" when the end is unknown).
static func for_sanction(sanction_type: String, reason_code: String, time_text: String) -> Dictionary:
	if reason_code == REASON_BLOCK_AND_ALERT:
		return _out(KEY_CARE, {}, TONE_CARE)
	if sanction_type == ChatStore.SANCTION_RESTRICT_TEXT:
		return _out(KEY_RESTRICT_TEXT, {}, TONE_WARNING)
	if not time_text.is_empty():
		return _out(KEY_MUTED_UNTIL, {"time": time_text}, TONE_WARNING)
	return _out(KEY_MUTED, {}, TONE_WARNING)


## Every key this mapper can return (catalog completeness test).
static func all_keys() -> PackedStringArray:
	var out: PackedStringArray = [KEY_MASKED, KEY_PII, KEY_TERM, KEY_HELD, KEY_CARE, KEY_RATE, KEY_MUTED, KEY_MUTED_UNTIL,
		KEY_RESTRICT_TEXT, KEY_RECONNECTING, KEY_GENERIC]
	for v: Variant in CODE_KEYS.values():
		if not String(v) in out:
			out.append(String(v))
	return out


static func _out(key: String, args: Dictionary, tone: String) -> Dictionary:
	return {"key": key, "args": args, "tone": tone}
