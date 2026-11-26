class_name SocialService
extends GameApiService
## Block and mute of another player from the game (F119-AC-15; SOCIAL_SYSTEM §5, CHAT_SYSTEM §9; backend F095). Both are
## idempotent `PUT`s (`blockUser`, `muteUser`) — the server is the one that enforces them (fan-out, history, DMs,
## invitations); once it answered `204` the client hides the player's messages at once (`relation_changed`). The player
## is never told. Ids only — never a nickname in a log (AGENTS R14, R16). Shared with the friends UI (F110).

## The server confirmed: `kind` is "block" or "mute".
signal relation_changed(user_id: String, kind: String)

const KIND_BLOCK: String = "block"
const KIND_MUTE: String = "mute"
const PATH_BLOCK: String = "/social/blocks/%s"
const PATH_MUTE: String = "/social/mutes/%s"


## Coroutine: `blockUser`.
func block_user(user_id: String) -> ApiResult:
	return await _put(user_id, PATH_BLOCK, "blockUser", KIND_BLOCK)


## Coroutine: `muteUser` (hides the player's messages for this student only).
func mute_user(user_id: String) -> ApiResult:
	return await _put(user_id, PATH_MUTE, "muteUser", KIND_MUTE)


func _put(user_id: String, path: String, operation_id: String, kind: String) -> ApiResult:
	if not Uuid.is_valid(user_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_request(ApiRequest.make_put(path % user_id), operation_id)
	if result.ok:
		relation_changed.emit(user_id, kind)
	return result
