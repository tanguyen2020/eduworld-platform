class_name FakeHttp
extends HttpTransport
## TEST-ONLY (AGENTS R17): scripted HTTP transport for ApiClient / AuthService tests (GODOT_ARCHITECTURE §12).
## Routes are "METHOD /path" globs (`*` = any characters except `?`); each route keeps a queue of replies and the
## last reply repeats until a new reply is queued for the route. Every request is recorded (method, path, query,
## headers, decoded body). Lives under tests/ (excluded from every export preset).
## F110: a route that names a query ("GET /social/friend-requests?direction=outgoing") is matched against the path AND
## the query first, before the routes of the path alone.


const METHOD_NAMES: Dictionary = {
	HTTPClient.METHOD_GET: "GET", HTTPClient.METHOD_POST: "POST", HTTPClient.METHOD_PUT: "PUT",
	HTTPClient.METHOD_PATCH: "PATCH", HTTPClient.METHOD_DELETE: "DELETE",
}

var base_url: String = "https://api.test.invalid/api/v1"
var requests: Array[Recorded] = []
## Frames each request waits before answering (lets concurrent coroutines interleave).
var delay_frames: int = 1
var _routes: Array[Dictionary] = []


class Recorded:
	extends RefCounted
	var method: String = ""
	var url: String = ""
	var path: String = ""
	var query: String = ""
	## Header names in lower case.
	var headers: Dictionary = {}
	var body: Dictionary = {}
	var raw_body: String = ""


## Queues a reply for `route` ("POST /auth/game/login", globs allowed). `body` is a Dictionary or null.
func reply(route: String, status: int, body: Variant = null, headers: Dictionary = {}) -> FakeHttp:
	_queue(route, {"status": status, "body": body, "headers": headers, "network_error": false})
	return self


## Queues a reply with a raw body (CDN files such as formula SVGs or media; F063).
func reply_bytes(route: String, status: int, bytes: PackedByteArray) -> FakeHttp:
	_queue(route, {"status": status, "body": null, "raw": bytes, "headers": {}, "network_error": false})
	return self


## Queues a network failure (no response) for `route`.
func fail(route: String) -> FakeHttp:
	_queue(route, {"status": 0, "body": null, "headers": {}, "network_error": true})
	return self


func send(method: HTTPClient.Method, url: String, headers: PackedStringArray, body: String) -> HttpTransport.Response:
	var rec: Recorded = Recorded.new()
	rec.method = String(METHOD_NAMES.get(method, "GET"))
	rec.url = url
	var rest: String = url.trim_prefix(base_url)
	var q: int = rest.find("?")
	rec.path = rest.substr(0, q) if q >= 0 else rest
	rec.query = rest.substr(q + 1) if q >= 0 else ""
	for line: String in headers:
		var sep: int = line.find(":")
		rec.headers[line.substr(0, sep).strip_edges().to_lower()] = line.substr(sep + 1).strip_edges()
	rec.raw_body = body
	var parsed: Variant = JSON.parse_string(body) if not body.is_empty() else null
	rec.body = parsed as Dictionary if parsed is Dictionary else {}
	requests.append(rec)
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	for i: int in delay_frames:
		await tree.process_frame
	var spec: Dictionary = _next("%s %s" % [rec.method, rec.path], rec.query)
	var response: HttpTransport.Response = HttpTransport.Response.new()
	if bool(spec.get("network_error", true)):
		response.result = HTTPRequest.RESULT_CANT_CONNECT
		return response
	response.result = HTTPRequest.RESULT_SUCCESS
	response.status = int(spec["status"])
	var out_headers: PackedStringArray = []
	var reply_headers: Dictionary = spec["headers"] as Dictionary
	for key: Variant in reply_headers.keys():
		out_headers.append("%s: %s" % [key, reply_headers[key]])
	var payload: Variant = spec["body"]
	if spec.get("raw") is PackedByteArray:
		response.body = spec["raw"] as PackedByteArray
	elif payload != null:
		out_headers.append("Content-Type: application/json")
		response.body = JSON.stringify(payload).to_utf8_buffer()
	response.headers = out_headers
	return response


## Recorded requests whose "METHOD /path" matches `route` (glob).
func find(route: String) -> Array[Recorded]:
	var out: Array[Recorded] = []
	var re: RegEx = _glob(route)
	for rec: Recorded in requests:
		if re.search("%s %s" % [rec.method, rec.path]) != null:
			out.append(rec)
	return out


func count(route: String) -> int:
	return find(route).size()


func last(route: String) -> Recorded:
	var found: Array[Recorded] = find(route)
	return found[found.size() - 1] if not found.is_empty() else null


## Every recorded request as text (headers + body), for "no secret leaves the device" style assertions.
func all_text() -> String:
	var parts: PackedStringArray = []
	for rec: Recorded in requests:
		parts.append("%s %s %s %s" % [rec.method, rec.url, JSON.stringify(rec.headers), rec.raw_body])
	return "\n".join(parts)


## Drops the queued replies of `route` (exact route text) so the next `reply()` is served first.
func clear(route: String) -> FakeHttp:
	_route(route).clear()
	return self


func _queue(route: String, spec: Dictionary) -> void:
	var replies: Array = _route(route)
	if replies.size() == 1 and bool((replies[0] as Dictionary).get("served", false)):
		replies.clear()
	replies.append(spec)


func _route(route: String) -> Array:
	for r: Dictionary in _routes:
		if String(r["route"]) == route:
			return r["replies"] as Array
	var replies: Array = []
	_routes.append({"route": route, "re": _glob(route), "replies": replies})
	return replies


func _next(key: String, query: String = "") -> Dictionary:
	if not query.is_empty():
		for r: Dictionary in _routes:
			if String(r["route"]).contains("?") and (r["re"] as RegEx).search("%s?%s" % [key, query]) != null and not (r["replies"] as Array).is_empty():
				return _serve(r["replies"] as Array)
	for r: Dictionary in _routes:
		if String(r["route"]).contains("?"):
			continue
		if (r["re"] as RegEx).search(key) != null:
			var replies: Array = r["replies"] as Array
			if replies.is_empty():
				break
			var spec: Dictionary = (replies.pop_front() if replies.size() > 1 else replies[0]) as Dictionary
			spec["served"] = true
			return spec
	return {"network_error": true}


func _serve(replies: Array) -> Dictionary:
	var spec: Dictionary = (replies.pop_front() if replies.size() > 1 else replies[0]) as Dictionary
	spec["served"] = true
	return spec


static func _glob(route: String) -> RegEx:
	var escaped: String = ""
	for c: String in route:
		if c == "*":
			escaped += "[^?]*"
		elif "\\^$.|?+()[]{}".contains(c):
			escaped += "\\" + c
		else:
			escaped += c
	return RegEx.create_from_string("^" + escaped + "$")
