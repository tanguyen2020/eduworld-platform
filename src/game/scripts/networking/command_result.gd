class_name CommandResult
extends RefCounted
## Outcome of `RealtimeClient.send_command` (F092-AC-06): `ok` on `system.command.acked`; otherwise the error code
## of `system.command.failed` (AsyncAPI `ErrorCode`), `TIMEOUT` after `client_command_timeout`, `OFFLINE` while the
## connection cannot take write commands (Backoff, reconnecting — F092-AC-08) or `INVALID_COMMAND`.

const CODE_TIMEOUT: String = "TIMEOUT"
const CODE_OFFLINE: String = "OFFLINE"
const CODE_INVALID: String = "INVALID_COMMAND"

var ok: bool = false
var code: String = ""
var retry_after_ms: int = 0
var result: Dictionary = {}
## `details` of `system.command.failed` (stable fields only, e.g. `category_code` of CHAT_CONTENT_BLOCKED — F119).
var details: Dictionary = {}


static func success(data: Dictionary = {}) -> CommandResult:
	var r: CommandResult = CommandResult.new()
	r.ok = true
	r.result = data
	return r


static func failure(error_code: String, retry_ms: int = 0, error_details: Dictionary = {}) -> CommandResult:
	var r: CommandResult = CommandResult.new()
	r.code = error_code
	r.retry_after_ms = retry_ms
	r.details = error_details
	return r
