class_name ApiRequest
extends RefCounted
## One logical REST call for `ApiClient.send()` (GAME_NETWORKING §2). Defaults fit a tenant-scoped student route:
## Bearer token, `X-Tenant-Id`, refresh on 401. Public routes (`getClientConfig`, `gameLogin`,
## `selectGameLoginTenant`) turn `authenticated` and `send_tenant` off (CONFIG_DEFAULTS `tenant_header_required`).

var method: String = "GET"
var path: String = ""
var query: Dictionary = {}
## JSON body (Dictionary) or null. Never logged.
var body: Variant = null
## Media type of `body` (`application/merge-patch+json` for `updateGameProfile`, F070).
var content_type: String = "application/json"
## Extra request headers (e.g. `If-None-Match` of an ETag cache, F070). Never logged.
var headers: Dictionary = {}
## Caller-provided UUIDv7 for important writes; kept unchanged on every automatic retry (AGENTS R10).
var idempotency_key: String = ""
var authenticated: bool = true
var send_tenant: bool = true
## Refresh the access token before sending (expiring) and once after a 401.
var allow_refresh: bool = true
## Automatic waits on `429` (`Retry-After`); 0 disables them (login queue is handled by AuthService).
var max_rate_limit_retries: int = 2


static func make_get(request_path: String, request_query: Dictionary = {}) -> ApiRequest:
	var r: ApiRequest = ApiRequest.new()
	r.method = "GET"
	r.path = request_path
	r.query = request_query
	return r


static func make_post(request_path: String, request_body: Variant = null) -> ApiRequest:
	var r: ApiRequest = ApiRequest.new()
	r.method = "POST"
	r.path = request_path
	r.body = request_body
	return r


static func make_put(request_path: String, request_body: Variant = null) -> ApiRequest:
	var r: ApiRequest = make_post(request_path, request_body)
	r.method = "PUT"
	return r


## JSON Merge Patch write (RFC 7396), e.g. `updateGameProfile`.
static func make_patch(request_path: String, request_body: Variant = null) -> ApiRequest:
	var r: ApiRequest = make_post(request_path, request_body)
	r.method = "PATCH"
	r.content_type = "application/merge-patch+json"
	return r


## Public route: no `Authorization`, no `X-Tenant-Id`, no refresh.
func as_public() -> ApiRequest:
	authenticated = false
	send_tenant = false
	allow_refresh = false
	return self


## Only GET and writes with an `Idempotency-Key` may be retried automatically (GAME_NETWORKING §2.3).
func is_retry_safe() -> bool:
	return method == "GET" or not idempotency_key.is_empty()


func http_method() -> HTTPClient.Method:
	match method:
		"POST":
			return HTTPClient.METHOD_POST
		"PUT":
			return HTTPClient.METHOD_PUT
		"PATCH":
			return HTTPClient.METHOD_PATCH
		"DELETE":
			return HTTPClient.METHOD_DELETE
	return HTTPClient.METHOD_GET
