class_name HttpRequestPool
extends HttpTransport
## Production transport (GAME_NETWORKING §2.4): at most MAX_PARALLEL `HTTPRequest` nodes under a host node
## (the `ApiClient` autoload); extra requests wait for a free slot in arrival order.

signal slot_released()

const MAX_PARALLEL: int = 4
const DEFAULT_TIMEOUT_SEC: float = 15.0

var timeout_sec: float = DEFAULT_TIMEOUT_SEC
var _host: Node = null
var _idle: Array[HTTPRequest] = []
var _created: int = 0


func _init(host: Node) -> void:
	_host = host


func send(method: HTTPClient.Method, url: String, headers: PackedStringArray, body: String) -> HttpTransport.Response:
	var response: HttpTransport.Response = HttpTransport.Response.new()
	if _host == null or not _host.is_inside_tree():
		return response
	while _idle.is_empty() and _created >= MAX_PARALLEL:
		await slot_released
	var node: HTTPRequest = _idle.pop_back() if not _idle.is_empty() else _create()
	node.timeout = timeout_sec
	if node.request(url, headers, method, body) != OK:
		_release(node)
		return response
	var out: Array = await node.request_completed
	response.result = int(out[0])
	response.status = int(out[1])
	response.headers = out[2] as PackedStringArray
	response.body = out[3] as PackedByteArray
	_release(node)
	return response


func _create() -> HTTPRequest:
	var node: HTTPRequest = HTTPRequest.new()
	node.name = "HttpRequest%d" % _created
	node.accept_gzip = true
	_created += 1
	_host.add_child(node)
	return node


func _release(node: HTTPRequest) -> void:
	_idle.append(node)
	slot_released.emit()
