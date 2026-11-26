class_name RestApiClient
extends Node
## Autoload #6 `ApiClient` (GODOT_ARCHITECTURE §4, GAME_NETWORKING §2, F061 US3). Every client service calls REST
## through `send()`:
## - standard headers (F061-AC-16): `Authorization: Bearer`, `X-Tenant-Id` (session tenant, except the public
##   routes — CONFIG_DEFAULTS `tenant_header_required`), `X-Device-Id`, `X-Client-Version`, `X-Client-Platform`,
##   `Accept-Language`, a fresh `traceparent` per request, and the caller's `Idempotency-Key` (kept on retry);
## - safe retries (F061-AC-17): network/timeout/502/503/504 only for GET or keyed writes, `Backoff` ≤ 3 times;
##   `429` waits `Retry-After` ≤ 2 times; an unkeyed write is never retried;
## - 401 → one single-flight refresh through `refresh_handler` (AuthService.refresh) then one replay (F061-AC-18);
## - `400 TENANT_HEADER_REQUIRED` (telemetry `client_bug`) / `403 TENANT_MISMATCH` → `auth_error_handler`
##   (F061-AC-19);
## - Problem Details parsed into `ApiResult` (F061-AC-20).
## Bodies and headers are never logged (`game_login_body_logging` off, AGENTS R14); logs keep method, path,
## status, code and trace id only. Business logic does not live here.

const TELEMETRY_CLIENT_BUG: String = "client_bug"
const CODE_TENANT_HEADER_REQUIRED: String = "TENANT_HEADER_REQUIRED"
const CODE_TENANT_MISMATCH: String = "TENANT_MISMATCH"
const CODE_INSECURE_TRANSPORT: String = "INSECURE_TRANSPORT"

var transport: HttpTransport = null
var clock: ClientClock = null
var rng: RandomNumberGenerator = null
var tokens: TokenStore = null
var device_id: String = ""
var base_url: String = ""
var client_version: String = ""
var platform: String = ""
## Coroutine `(used_access_token: String) -> bool`: refreshes the access token (single-flight in AuthService).
var refresh_handler: Callable = Callable()
## `(code: String) -> void`: the session can no longer be used (tenant header/mismatch after login).
var auth_error_handler: Callable = Callable()
## Allow `http://` (debug builds, local compose and the Prism mock). Release builds require HTTPS (GAME_NETWORKING §9).
var allow_insecure: bool = OS.is_debug_build()


func _ready() -> void:
	# F113-AC-04: requests finish behind the rotate prompt (OrientationGuard pauses the tree, not the network).
	process_mode = Node.PROCESS_MODE_ALWAYS
	if transport == null:
		transport = HttpRequestPool.new(self)
	if clock == null:
		clock = ClientClock.new()
	if rng == null:
		rng = RandomNumberGenerator.new()
	if tokens == null:
		tokens = TokenStore.new()


## Test seam: injects the transport and clock (FakeHttp / FakeClock) and the client identity.
func configure(http: HttpTransport, time_source: ClientClock, token_store: TokenStore, install_id: String,
		api_base_url: String, version: String, client_platform: String) -> void:
	transport = http
	clock = time_source
	tokens = token_store
	device_id = install_id
	base_url = api_base_url
	client_version = version
	platform = client_platform
	if rng == null:
		rng = RandomNumberGenerator.new()


## `X-Client-Platform` / `GameLoginRequest.platform` value of the running OS. Released builds send android, ios or web;
## windows, macos and linux only come from editor or debug runs on a developer machine (ADR-0101).
static func client_platform(os_name: String = OS.get_name(), is_web: bool = OS.has_feature("web")) -> String:
	if is_web:
		return "web"
	match os_name:
		"Windows":
			return "windows"
		"Android":
			return "android"
		"iOS":
			return "ios"
		"macOS":
			return "macos"
	return "linux"


## Accept-Language value of the current UI locale (`vi-VN`, `en-US`).
static func accept_language() -> String:
	return TranslationServer.get_locale().replace("_", "-")


## Coroutine: sends `req` with headers, retries and refresh. Always returns an ApiResult.
func send(req: ApiRequest) -> ApiResult:
	var url: String = _url(req)
	if not allow_insecure and not url.begins_with("https://"):
		return ApiResult.failure(CODE_INSECURE_TRANSPORT)
	if req.authenticated and req.allow_refresh and tokens.access_expiring(clock.now_msec()) and refresh_handler.is_valid():
		await refresh_handler.call(tokens.access_token)
	var body_text: String = JSON.stringify(req.body) if req.body != null else ""
	var network_retries: int = 0
	var rate_limit_retries: int = 0
	var refreshed: bool = false
	while true:
		var used_token: String = tokens.access_token
		var traceparent: String = TraceContext.new_traceparent()
		var started: int = clock.now_msec()
		var response: HttpTransport.Response = await transport.send(req.http_method(), url, build_headers(req, traceparent), body_text)
		var result: ApiResult = ApiResult.from_response(response)
		result.trace_id = TraceContext.trace_id_of(traceparent)
		_log_result(req, result, clock.now_msec() - started)
		if (result.network_error or result.is_server_unavailable()) and req.is_retry_safe() and network_retries < Backoff.MAX_RETRIES:
			network_retries += 1
			await clock.wait(Backoff.delay(network_retries, rng))
			continue
		if result.status == 429 and rate_limit_retries < mini(req.max_rate_limit_retries, Backoff.MAX_RATE_LIMIT_RETRIES):
			rate_limit_retries += 1
			await clock.wait(result.retry_after_sec())
			continue
		if result.status == 401 and req.authenticated and req.allow_refresh and not refreshed and refresh_handler.is_valid():
			refreshed = true
			var renewed: bool = bool(await refresh_handler.call(used_token))
			if renewed:
				continue
		if req.authenticated:
			_check_tenant_error(req, result)
		return result
	return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


func build_headers(req: ApiRequest, traceparent: String) -> PackedStringArray:
	var headers: PackedStringArray = [
		"Accept: application/json, application/problem+json",
		"X-Client-Version: %s" % client_version,
		"X-Client-Platform: %s" % platform,
		"Accept-Language: %s" % accept_language(),
		"traceparent: %s" % traceparent,
	]
	if not device_id.is_empty():
		headers.append("X-Device-Id: %s" % device_id)
	if req.body != null:
		headers.append("Content-Type: %s" % req.content_type)
	for key: Variant in req.headers.keys():
		headers.append("%s: %s" % [key, req.headers[key]])
	if req.authenticated and tokens.has_access():
		headers.append("Authorization: Bearer %s" % tokens.access_token)
	if req.send_tenant and not tokens.tenant_id.is_empty():
		headers.append("X-Tenant-Id: %s" % tokens.tenant_id)
	if not req.idempotency_key.is_empty():
		headers.append("Idempotency-Key: %s" % req.idempotency_key)
	return headers


func _url(req: ApiRequest) -> String:
	var root: String = base_url
	if root.is_empty():
		var config: Node = _autoload("Config")
		root = String(config.get("api_base_url")) if config != null else ""
	var url: String = root.trim_suffix("/") + req.path
	if not req.query.is_empty():
		var parts: PackedStringArray = []
		for key: Variant in req.query.keys():
			parts.append("%s=%s" % [String(key).uri_encode(), str(req.query[key]).uri_encode()])
		url += "?" + "&".join(parts)
	return url


func _check_tenant_error(req: ApiRequest, result: ApiResult) -> void:
	var tenant_error: bool = (result.status == 400 and result.code == CODE_TENANT_HEADER_REQUIRED) \
			or (result.status == 403 and result.code == CODE_TENANT_MISMATCH)
	if not tenant_error:
		return
	if result.code == CODE_TENANT_HEADER_REQUIRED:
		_queue_telemetry(TELEMETRY_CLIENT_BUG, {"code": result.code, "path": req.path, "trace_id": result.trace_id})
	if auth_error_handler.is_valid():
		auth_error_handler.call(result.code)


func _log_result(req: ApiRequest, result: ApiResult, elapsed_msec: int) -> void:
	var log_node: Node = _autoload("Log")
	if log_node == null:
		return
	var fields: Dictionary = {
		"method": req.method, "path": req.path, "status": result.status, "code": result.code,
		"trace_id": result.trace_id, "elapsed_ms": elapsed_msec,
	}
	log_node.call("debug" if result.ok else "info", "api_request_completed", fields)


func _queue_telemetry(event_name: String, fields: Dictionary) -> void:
	var app_state: Node = _autoload("AppState")
	if app_state != null:
		app_state.call("queue_telemetry", event_name, fields)


static func _autoload(autoload_name: String) -> Node:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.get_node_or_null(autoload_name) if tree != null else null
