class_name ReportService
extends GameApiService
## Reports of a message or a player (F119-AC-13, AC-14; MODERATION §5, CHAT_SYSTEM §9): `createReport` with one
## `Idempotency-Key` per report intent of the student, kept when the same report is retried after a network failure
## (AGENTS R10), and exactly the fields of `ReportCreate` — target, reason, context (`channel_id`, `match_id`,
## `party_id`). No message text, no nickname and no note is sent: the Safety Team reads the evidence on the server
## (recalled / edited messages included, MODERATION §6.1). The reporter is never shown to the reported player.

## The reason codes of `ReportCreate.reason` (MODERATION §5), in the order of the report dialog.
const REASONS: PackedStringArray = ["rude_language", "bullying", "asked_personal_info", "spam", "inappropriate_content",
	"cheating", "other"]
const TARGET_MESSAGE: String = "message"
const TARGET_USER: String = "user"
const PATH_REPORTS: String = "/chat/reports"

var _intent_keys: Dictionary = {}


## Coroutine: `createReport`. `context` keys outside `channel_id` / `match_id` / `party_id` (or not UUIDs) are dropped.
func create_report(target_type: String, target_id: String, reason: String, context: Dictionary = {}) -> ApiResult:
	if not target_type in [TARGET_MESSAGE, TARGET_USER] or not Uuid.is_valid(target_id) or not reason in REASONS:
		return ApiResult.failure("VALIDATION_FAILED")
	var ctx: Dictionary = {}
	for k: String in ["channel_id", "match_id", "party_id"]:
		var v: String = String(context.get(k, ""))
		if Uuid.is_valid(v):
			ctx[k] = v
	var body: Dictionary = {"target_type": target_type, "target_id": target_id, "reason": reason}
	if not ctx.is_empty():
		body["context"] = ctx
	var intent: String = "%s:%s:%s:%s" % [target_type, target_id, reason, JSON.stringify(ctx)]
	if not _intent_keys.has(intent):
		_intent_keys[intent] = Uuid.v7()
	var req: ApiRequest = ApiRequest.make_post(PATH_REPORTS, body)
	req.idempotency_key = String(_intent_keys[intent])
	var result: ApiResult = await send_request(req, "createReport")
	if not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	return result


func clear() -> void:
	_intent_keys.clear()
