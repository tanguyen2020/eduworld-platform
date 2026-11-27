class_name ContentDownloader
extends RefCounted
## Public zone packs by hash (F133 T037, F133-AC-18/19/20/21, PERFORMANCE_BUDGET §10.1, §11, ADR-0044, AGENTS R31).
## A bundle entry of the signed bundle index ({bundle_id, version, url, sha256, size_bytes, grade_bands}) is taken from,
## in order:
##   1. the device cache `user://cache/bundles/<sha256>.pck` (no telemetry — nothing was downloaded);
##   2. the preinstalled folder of the school APK kit (`DeviceConfig.preinstalled_bundles_dir`/<sha256>.pck, used in
##      place, never copied or evicted) → telemetry `preinstalled` once per pack and process;
##   3. the school LAN cache (`DeviceConfig.lan_cache_url` + the content-addressed key of the CDN URL) → `lan_cache`;
##   4. the CDN URL of the entry → `cdn`.
## Every pack must match its SHA-256 before it is kept; a LAN pack that does not match is deleted and the CDN is tried;
## a CDN pack that does not match is an error (`CONTENT_HASH_MISMATCH`, nothing kept). Only content-addressed public
## keys (`/bundles/<sha256>.pck`, unsigned `/media/<sha256>/<file>`) ever go to the LAN — no API request, no signed
## (private) media, no token (F133-AC-18). The cache keeps at most `cache_limit_bytes` (CONFIG_DEFAULTS `build_size`
## device cache 1.5 GB, project setting `eduworld/content/cache_limit_bytes`) and drops least-recently-used packs.
## Telemetry `content_download` carries only `source` + `bytes` (TelemetryService.queue_content_download, R14).
## Low data mode (device.cfg `low_data_mode` or a slow BandwidthProbe) stops prefetching the next zone and asks for low
## media quality (F133-AC-20). Fetches are public HTTP GETs without headers; tests replace `fetcher`.

signal bundle_downloaded(bundle_id: String, source: String, path: String)

const SOURCE_CACHE: String = "cache"
const ERR_HASH_MISMATCH: String = "CONTENT_HASH_MISMATCH"
const ERR_BAD_ENTRY: String = "CONTENT_ENTRY_INVALID"
const ERR_UNAVAILABLE: String = "CONTENT_UNAVAILABLE"
const QUALITY_LOW: String = "low"
const QUALITY_NORMAL: String = "normal"
const DEFAULT_CACHE_DIR: String = "user://cache/bundles"
const LRU_FILE: String = "lru.json"
const TIMEOUT_SEC: float = 300.0
const BUNDLE_KEY: String = "^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~-]+)*(/bundles/[a-f0-9]{64}\\.pck)$"
const MEDIA_KEY: String = "^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~-]+)*(/media/[a-f0-9]{64}/[A-Za-z0-9._-]+)$"

var device: DeviceConfig = null
var telemetry: TelemetryService = null
var probe: BandwidthProbe = null
var cache_dir: String = DEFAULT_CACHE_DIR
var cache_limit_bytes: int = 1_500_000_000
## Node that hosts the HTTPRequest of the default fetcher (an autoload).
var host: Node = null
## Coroutine `(url: String, dest_path: String) -> bool` replacing the HTTP download (tests: TEST-ONLY fakes).
var fetcher: Callable = Callable()
var _counted_preinstalled: Dictionary = {}


func _init(device_config: DeviceConfig = null, telemetry_service: TelemetryService = null, host_node: Node = null) -> void:
	device = device_config if device_config != null else DeviceConfig.parse("", "other")
	telemetry = telemetry_service
	host = host_node
	cache_limit_bytes = int(ProjectSettings.get_setting("eduworld/content/cache_limit_bytes", cache_limit_bytes))


func is_low_data() -> bool:
	return device.low_data_mode or (probe != null and probe.is_low())


func media_quality() -> String:
	return QUALITY_LOW if is_low_data() else QUALITY_NORMAL


func next_zone_prefetch_allowed() -> bool:
	return not is_low_data()


## LAN URL of a content-addressed CDN URL, empty when there is no LAN cache or the URL is not a public content key.
func lan_url_for(url: String) -> String:
	if device.lan_cache_url.is_empty():
		return ""
	for pattern: String in [BUNDLE_KEY, MEDIA_KEY]:
		var m: RegExMatch = RegEx.create_from_string(pattern).search(url)
		if m != null:
			return device.lan_cache_url + m.get_string(m.get_group_count())
	return ""


## Coroutine: local path of a bundle (Result.value), `details.source` = cache/preinstalled/lan_cache/cdn.
func fetch_bundle(entry: Dictionary) -> Result:
	var sha: String = String(entry.get("sha256", ""))
	var url: String = String(entry.get("url", ""))
	if RegEx.create_from_string(ManifestVerifier.SHA256_HEX).search(sha) == null \
			or RegEx.create_from_string(BUNDLE_KEY).search(url) == null \
			or not url.ends_with("/bundles/%s.pck" % sha):
		return Result.failure(ERR_BAD_ENTRY)
	var bundle_id: String = String(entry.get("bundle_id", ""))
	var cached: String = cache_dir.path_join("%s.pck" % sha)
	if FileAccess.file_exists(cached) and ManifestVerifier.matches_sha256(cached, sha):
		_touch(sha)
		return _done(bundle_id, SOURCE_CACHE, cached)
	var pre: String = _preinstalled_path(sha)
	if not pre.is_empty():
		if not _counted_preinstalled.has(sha):
			_counted_preinstalled[sha] = true
			_count(TelemetryService.DOWNLOAD_SOURCE_PREINSTALLED, _size_of(pre))
		return _done(bundle_id, TelemetryService.DOWNLOAD_SOURCE_PREINSTALLED, pre)
	DirAccess.make_dir_recursive_absolute(cache_dir)
	var lan: String = lan_url_for(url)
	if not lan.is_empty() and await _download_verified(lan, cached, sha):
		_stored(sha, cached, TelemetryService.DOWNLOAD_SOURCE_LAN_CACHE)
		return _done(bundle_id, TelemetryService.DOWNLOAD_SOURCE_LAN_CACHE, cached)
	var started: int = Time.get_ticks_msec()
	if not await _download(url, cached):
		return Result.failure(ERR_UNAVAILABLE)
	if not ManifestVerifier.matches_sha256(cached, sha):
		DirAccess.remove_absolute(cached)
		return Result.failure(ERR_HASH_MISMATCH)
	if probe != null:
		probe.record(_size_of(cached), Time.get_ticks_msec() - started)
	_stored(sha, cached, TelemetryService.DOWNLOAD_SOURCE_CDN)
	return _done(bundle_id, TelemetryService.DOWNLOAD_SOURCE_CDN, cached)


func _preinstalled_path(sha: String) -> String:
	if device.preinstalled_bundles_dir.is_empty():
		return ""
	var path: String = device.preinstalled_bundles_dir.path_join("%s.pck" % sha)
	if FileAccess.file_exists(path) and ManifestVerifier.matches_sha256(path, sha):
		return path
	return ""


func _download_verified(url: String, dest: String, sha: String) -> bool:
	if not await _download(url, dest):
		return false
	if ManifestVerifier.matches_sha256(dest, sha):
		return true
	DirAccess.remove_absolute(dest)
	return false


func _download(url: String, dest: String) -> bool:
	var part: String = dest + ".part"
	var ok: bool = false
	if fetcher.is_valid():
		ok = bool(await fetcher.call(url, part))
	else:
		ok = await _http_get(url, part)
	if ok and FileAccess.file_exists(part):
		DirAccess.remove_absolute(dest)
		ok = DirAccess.rename_absolute(part, dest) == OK
	if not ok:
		DirAccess.remove_absolute(part)
	return ok


func _http_get(url: String, path: String) -> bool:
	if host == null or not host.is_inside_tree():
		return false
	var req: HTTPRequest = HTTPRequest.new()
	req.timeout = TIMEOUT_SEC
	req.download_file = path
	host.add_child(req)
	var ok: bool = req.request(url) == OK
	if ok:
		var out: Array = await req.request_completed
		ok = int(out[0]) == HTTPRequest.RESULT_SUCCESS and int(out[1]) == 200
	req.queue_free()
	return ok


func _stored(sha: String, path: String, source: String) -> void:
	_count(source, _size_of(path))
	_touch(sha)
	_evict(sha)


func _done(bundle_id: String, source: String, path: String) -> Result:
	bundle_downloaded.emit(bundle_id, source, path)
	var r: Result = Result.success(path)
	r.details = {"source": source}
	return r


func _count(source: String, bytes: int) -> void:
	if telemetry != null:
		telemetry.queue_content_download(source, bytes)


# ---------- LRU (spec Edge Cases: drop least-used packs above the cache limit, never preinstalled ones) ----------

func _lru() -> Dictionary:
	var path: String = cache_dir.path_join(LRU_FILE)
	if not FileAccess.file_exists(path):
		return {}
	var data: Variant = JSON.parse_string(FileAccess.get_file_as_string(path))
	return data as Dictionary if data is Dictionary else {}


func _save_lru(lru: Dictionary) -> void:
	var f: FileAccess = FileAccess.open(cache_dir.path_join(LRU_FILE), FileAccess.WRITE)
	if f != null:
		f.store_string(JSON.stringify(lru))
		f.close()


## Marks `sha` as just used. The counter is a sequence number, not a clock, so the order is exact within a second.
func _touch(sha: String) -> void:
	var lru: Dictionary = _lru()
	var top: int = 0
	for v: Variant in lru.values():
		top = maxi(top, int(v))
	lru[sha] = top + 1
	_save_lru(lru)


func _evict(keep_sha: String) -> void:
	var dir: DirAccess = DirAccess.open(cache_dir)
	if dir == null:
		return
	var lru: Dictionary = _lru()
	var sizes: Dictionary = {}
	var total: int = 0
	for file_name: String in dir.get_files():
		if file_name.ends_with(".pck"):
			var sha: String = file_name.get_basename()
			sizes[sha] = _size_of(cache_dir.path_join(file_name))
			total += int(sizes[sha])
	var order: Array = sizes.keys()
	order.sort_custom(func(a: Variant, b: Variant) -> bool: return int(lru.get(a, 0)) < int(lru.get(b, 0)))
	for sha: Variant in order:
		if total <= cache_limit_bytes:
			break
		if String(sha) == keep_sha:
			continue
		if dir.remove("%s.pck" % sha) == OK:
			total -= int(sizes[sha])
			lru.erase(sha)
	_save_lru(lru)


static func _size_of(path: String) -> int:
	var f: FileAccess = FileAccess.open(path, FileAccess.READ)
	if f == null:
		return 0
	var n: int = f.get_length()
	f.close()
	return n
