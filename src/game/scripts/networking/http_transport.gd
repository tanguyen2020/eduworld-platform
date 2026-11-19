class_name HttpTransport
extends RefCounted
## One HTTP exchange, below `ApiClient` (research R2). The production transport is `HttpRequestPool`
## (pool of `HTTPRequest`, also mapped to `fetch` on Web); tests use `FakeHttp` (tests/fakes, TEST-ONLY).
## A transport never logs headers or bodies (AGENTS R14, `game_login_body_logging` off).


## Raw response. `result` is an `HTTPRequest.Result`; anything but RESULT_SUCCESS is a network error.
class Response:
	extends RefCounted
	var result: int = HTTPRequest.RESULT_CANT_CONNECT
	var status: int = 0
	var headers: PackedStringArray = []
	var body: PackedByteArray = PackedByteArray()


## Coroutine: sends one request. The base class has no network and always reports a connection failure.
func send(_method: HTTPClient.Method, _url: String, _headers: PackedStringArray, _body: String) -> Response:
	return Response.new()
