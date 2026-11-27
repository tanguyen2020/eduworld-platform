class_name VideoFileCache
extends RefCounted
## Theora files of video lessons (F065-AC-15, ADR-0044): `VideoStreamTheora` plays a file, so the variant is downloaded
## once to `user://cache/media/<sha256>/<variant>.ogv` (immutable key `media/<sha256>/…` of the CDN URL; the signed
## query string is never stored or logged) and reused. Video is public lesson media, never an answer or a student's
## data. An `HTTPRequest` streams the body to the file under a host node (the ApiClient autoload).

const CACHE_ROOT: String = "user://cache/media"
const TIMEOUT_SEC: float = 120.0

var host: Node = null
## Coroutine `(url: String, path: String) -> bool` replacing the download (tests, TEST-ONLY fakes).
var downloader: Callable = Callable()
## Each finished download is counted as `content_download` (source `cdn`, file size only — F133-AC-21, R14).
var telemetry: TelemetryService = null


func _init(host_node: Node = null) -> void:
	host = host_node


## Local path of a CDN URL (`…/media/<sha256>/<file>?sig…` → `user://cache/media/<sha256>/<file>`), empty when the URL
## has no immutable media key.
static func local_path(url: String) -> String:
	var clean: String = url.split("?")[0]
	var at: int = clean.find("/media/")
	if at < 0:
		return ""
	var key: String = clean.substr(at + "/media/".length())
	if key.contains("..") or key.split("/").size() != 2:
		return ""
	return CACHE_ROOT.path_join(key)


## Coroutine: the local file of `url` (downloaded when missing), empty on failure.
func fetch(url: String) -> String:
	var path: String = local_path(url)
	if path.is_empty():
		return ""
	if FileAccess.file_exists(path):
		return path
	DirAccess.make_dir_recursive_absolute(path.get_base_dir())
	var ok: bool = false
	if downloader.is_valid():
		ok = bool(await downloader.call(url, path))
	else:
		ok = await _download(url, path)
	if not ok or not FileAccess.file_exists(path):
		return ""
	if telemetry != null:
		telemetry.queue_content_download(TelemetryService.DOWNLOAD_SOURCE_CDN, _file_size(path))
	return path


static func _file_size(path: String) -> int:
	var f: FileAccess = FileAccess.open(path, FileAccess.READ)
	if f == null:
		return 0
	var size: int = f.get_length()
	f.close()
	return size


func _download(url: String, path: String) -> bool:
	if host == null or not host.is_inside_tree():
		return false
	var req: HTTPRequest = HTTPRequest.new()
	req.timeout = TIMEOUT_SEC
	req.download_file = path + ".part"
	host.add_child(req)
	var ok: bool = req.request(url) == OK
	if ok:
		var out: Array = await req.request_completed
		ok = int(out[0]) == HTTPRequest.RESULT_SUCCESS and int(out[1]) == 200
	req.queue_free()
	if ok:
		ok = DirAccess.rename_absolute(path + ".part", path) == OK
	else:
		DirAccess.remove_absolute(path + ".part")
	return ok
