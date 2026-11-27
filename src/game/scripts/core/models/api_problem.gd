class_name ApiProblem
extends RefCounted
## Problem Details (RFC 9457) returned by the API (`Problem` in specs/openapi/eduworld-api.v1.yaml). The client
## accepts unknown codes (shown as `ui.error.generic`, GODOT_ARCHITECTURE §9) and never displays `detail` to a
## student (support only). F061 adds the typed extension members the game login flow uses (F061-AC-20):
## `errors[]`, `choices[]` + `login_ticket` (GameLoginProblem), `other_device` (ActiveSessionConflict),
## `min_client_version` + `store_url` (ClientVersionProblem), `reopen_at` (CURFEW_ACTIVE). F109: `ticket_id` of
## `409 MM_TICKET_ALREADY_ACTIVE` (only when the held ticket includes the caller).


var type: String = ""
var title: String = ""
var status: int = 0
var code: String = ""
var detail: String = ""
var trace_id: String = ""
var retryable: bool = false
## `errors[]` of 422 VALIDATION_FAILED, each `{field, code, ...}` as sent.
var errors: Array[Dictionary] = []
var login_ticket: String = ""
var min_client_version: String = ""
var store_url: String = ""
var reopen_at: String = ""
var ticket_id: String = ""
var choices: Array[TenantChoice] = []
var other_device: OtherDevice = null


## One tenant the student may pick after authentication (`GameLoginTenantChoice`).
class TenantChoice:
	extends RefCounted
	var tenant_id: String = ""
	var tenant_type: String = ""
	var name: String = ""
	var school_name: String = ""
	var logo_url: String = ""


## Device holding the active game session (`OtherDevice`).
class OtherDevice:
	extends RefCounted
	var platform: String = ""
	var device_label: String = ""
	var last_seen_at: String = ""


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
	problem.login_ticket = _string(data, "login_ticket")
	problem.min_client_version = _string(data, "min_client_version")
	problem.store_url = _string(data, "store_url")
	problem.reopen_at = _string(data, "reopen_at")
	problem.ticket_id = _string(data, "ticket_id")
	var errs: Variant = data.get("errors", [])
	if errs is Array:
		for item: Variant in (errs as Array):
			if item is Dictionary:
				problem.errors.append(item as Dictionary)
	var raw_choices: Variant = data.get("choices", [])
	if raw_choices is Array:
		for item: Variant in (raw_choices as Array):
			var choice: TenantChoice = _choice(item)
			if choice != null:
				problem.choices.append(choice)
	var device: Variant = data.get("other_device")
	if device is Dictionary:
		var d: Dictionary = device as Dictionary
		problem.other_device = OtherDevice.new()
		problem.other_device.platform = _string(d, "platform")
		problem.other_device.device_label = _string(d, "device_label")
		problem.other_device.last_seen_at = _string(d, "last_seen_at")
	return problem


## Translated, student-friendly message for this problem (F060-AC-12).
func message() -> String:
	return L10n.error_text(code)


static func _choice(item: Variant) -> TenantChoice:
	if not item is Dictionary:
		return null
	var d: Dictionary = item as Dictionary
	var tenant_id: String = _string(d, "tenant_id")
	if tenant_id.is_empty():
		return null
	var choice: TenantChoice = TenantChoice.new()
	choice.tenant_id = tenant_id
	choice.tenant_type = _string(d, "tenant_type")
	choice.name = _string(d, "name")
	choice.school_name = _string(d, "school_name")
	choice.logo_url = _string(d, "logo_url")
	return choice


static func _string(d: Dictionary, key: String) -> String:
	var value: Variant = d.get(key, "")
	return value as String if value is String else ""
