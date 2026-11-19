class_name ApiProblem
extends RefCounted
## Problem Details (RFC 9457) returned by the API (`Problem` in specs/openapi/eduworld-api.v1.yaml). The client
## accepts unknown codes (shown as `ui.error.generic`, GODOT_ARCHITECTURE §9) and never displays `detail` to a
## student (support only).

var type: String = ""
var title: String = ""
var status: int = 0
var code: String = ""
var detail: String = ""
var trace_id: String = ""
var retryable: bool = false


## Parses a decoded JSON body; returns null when a required field (type, title, status, code) is missing.
static func from_dict(data: Dictionary) -> ApiProblem:
	for key: String in ["type", "title", "status", "code"]:
		if not data.has(key):
			return null
	var problem: ApiProblem = ApiProblem.new()
	problem.type = String(data["type"])
	problem.title = String(data["title"])
	problem.status = int(data["status"])
	problem.code = String(data["code"])
	problem.detail = String(data.get("detail", ""))
	problem.trace_id = String(data.get("trace_id", ""))
	problem.retryable = bool(data.get("retryable", false))
	return problem


## Translated, student-friendly message for this problem (F060-AC-12).
func message() -> String:
	return L10n.error_text(code)
