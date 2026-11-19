class_name LiveBackend
extends RefCounted
## TEST-ONLY: wiring for the F061 integration tests against the backend compose (quickstart §1: `make up`,
## `make migrate`, `make seed`, `make up-app`). The tests are skipped unless EDUWORLD_IT_API_BASE_URL is set
## (e.g. http://localhost:9080/api/v1). Seed passwords come from the environment (EDUWORLD_IT_STUDENT_PASSWORD,
## EDUWORLD_IT_STAFF_PASSWORD) — never from the repository (AGENTS R3/R4).


const ENV_BASE: String = "EDUWORLD_IT_API_BASE_URL"
const ENV_STUDENT_PASSWORD: String = "EDUWORLD_IT_STUDENT_PASSWORD"
const ENV_STAFF_PASSWORD: String = "EDUWORLD_IT_STAFF_PASSWORD"

var api: RestApiClient = null
var auth: GameAuthService = null
var transport: RecordingTransport = RecordingTransport.new()
var storage: SecureStorage = null
var states: Array[int] = []


## Records every request (method, path, header names/values) and forwards it to the real HTTP pool.
class RecordingTransport:
	extends HttpTransport
	var inner: HttpTransport = null
	var base_url: String = ""
	var sent: Array[Dictionary] = []

	func send(method: HTTPClient.Method, url: String, headers: PackedStringArray, body: String) -> HttpTransport.Response:
		var names: Dictionary = {}
		for line: String in headers:
			var sep: int = line.find(":")
			names[line.substr(0, sep).strip_edges().to_lower()] = line.substr(sep + 1).strip_edges()
		var path: String = url.trim_prefix(base_url).split("?")[0]
		sent.append({"method": method, "path": path, "headers": names})
		return await inner.send(method, url, headers, body)


static func skip() -> bool:
	return OS.get_environment(ENV_BASE).is_empty()


static func student_password() -> String:
	return OS.get_environment(ENV_STUDENT_PASSWORD)


static func staff_password() -> String:
	return OS.get_environment(ENV_STAFF_PASSWORD)


## Builds an AuthService + ApiClient pair talking to the real backend; `host` keeps the HTTPRequest nodes.
static func create(host: Node, device_id: String, secure_storage: SecureStorage = null) -> LiveBackend:
	var live: LiveBackend = LiveBackend.new()
	var base: String = OS.get_environment(ENV_BASE)
	live.api = RestApiClient.new()
	host.add_child(live.api)
	live.transport.inner = HttpRequestPool.new(live.api)
	live.transport.base_url = base
	live.api.configure(live.transport, ClientClock.new(), TokenStore.new(), device_id, base, Config.client_version,
			RestApiClient.client_platform())
	live.storage = secure_storage if secure_storage != null else FakeSecureStorage.new()
	live.auth = GameAuthService.new()
	host.add_child(live.auth)
	live.auth.configure(live.api, live.api.clock, live.storage, null, device_id, Config.client_version,
			RestApiClient.client_platform(), false)
	live.auth.navigator = func(s: int) -> void: live.states.append(s)
	live.auth.quitter = func() -> void: pass
	return live


func dispose() -> void:
	auth.sessions.stop_heartbeat()
	auth.queue_free()
	api.queue_free()


func last_state() -> int:
	return states[states.size() - 1] if not states.is_empty() else -1
