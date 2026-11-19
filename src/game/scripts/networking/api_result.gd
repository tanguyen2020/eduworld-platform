class_name ApiResult
extends RefCounted
## Typed outcome of `ApiClient.send()` (GAME_NETWORKING §2.2, F061-AC-20): `ok` for 2xx with the parsed JSON in
## `data`; otherwise `code` is the Problem `code`, `NETWORK_ERROR` (no response / timeout) or `HTTP_<status>`
## (error without a Problem body). UI maps `code` to `ui.error.<CODE>`, never `title` or `detail`.

const CODE_NETWORK_ERROR: String = "NETWORK_ERROR"
const CODE_INVALID_RESPONSE: String = "INVALID_RESPONSE"

var ok: bool = false
var status: int = 0
var data: Variant = null
var problem: ApiProblem = null
## Response headers, names in lower case.
var headers: Dictionary = {}
var network_error: bool = false
var code: String = ""
## `traceparent` trace id sent with the request (log correlation).
var trace_id: String = ""


static func from_response(response: HttpTransport.Response) -> ApiResult:
	var r: ApiResult = ApiResult.new()
	if response.result != HTTPRequest.RESULT_SUCCESS:
		r.network_error = true
		r.code = CODE_NETWORK_ERROR
		return r
	r.status = response.status
	for line: String in response.headers:
		var sep: int = line.find(":")
		if sep > 0:
			r.headers[line.substr(0, sep).strip_edges().to_lower()] = line.substr(sep + 1).strip_edges()
	var text: String = response.body.get_string_from_utf8()
	if not text.strip_edges().is_empty():
		r.data = JSON.parse_string(text)
	r.ok = r.status >= 200 and r.status < 300
	if not r.ok:
		if r.data is Dictionary:
			r.problem = ApiProblem.from_dict(r.data as Dictionary)
		r.code = r.problem.code if r.problem != null else "HTTP_%d" % r.status
	return r


static func failure(error_code: String) -> ApiResult:
	var r: ApiResult = ApiResult.new()
	r.code = error_code
	return r


## Body as a Dictionary (empty when the body is not a JSON object).
func body() -> Dictionary:
	return data as Dictionary if data is Dictionary else {}


func is_server_unavailable() -> bool:
	return status == 502 or status == 503 or status == 504


## Transient failure: no response, 5xx, or 429 (resume keeps the stored session and retries, F061-AC-31).
func is_transient() -> bool:
	return network_error or status >= 500 or status == 429


## `Retry-After` in seconds (delta-seconds form); `fallback` when missing or not a number.
func retry_after_sec(fallback: float = Backoff.DEFAULT_RETRY_AFTER_SEC) -> float:
	var raw: String = String(headers.get("retry-after", "")).strip_edges()
	if raw.is_valid_float():
		return maxf(0.0, raw.to_float())
	return fallback
