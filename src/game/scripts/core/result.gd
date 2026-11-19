class_name Result
extends RefCounted
## Shared result type for services (F060 T006): `ok` with `value`, or a stable error `code` (ProblemCode or a
## client code such as `CONFIG_KEY_MISSING`) with optional `details`. Never carries tokens or passwords.

var ok: bool = false
var value: Variant = null
var code: String = ""
var details: Dictionary = {}


static func success(result_value: Variant = null) -> Result:
	var r: Result = Result.new()
	r.ok = true
	r.value = result_value
	return r


static func failure(error_code: String, error_details: Dictionary = {}) -> Result:
	var r: Result = Result.new()
	r.ok = false
	r.code = error_code
	r.details = error_details
	return r


func _to_string() -> String:
	return "Result(ok)" if ok else "Result(%s)" % code
