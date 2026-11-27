class_name PrefetchScheduler
extends RefCounted
## Off-hours prefetch of public zone packs on shared Android devices (F133 T038, F133-AC-19, F133-AC-20,
## PERFORMANCE_BUDGET §10.1, spec Clarifications 2026-11-16). Runs only when
##   - the device is shared (`device.cfg` `shared_device: true`; never on Web — ADR-0101),
##   - the local time is inside `offhours_prefetch_window` (CONFIG_DEFAULTS, project setting
##     `eduworld/content/offhours_prefetch_window`, default 18:00–06:00, wraps midnight),
##   - low data mode is off (device.cfg or a slow BandwidthProbe — ContentDownloader.next_zone_prefetch_allowed),
##   - a public bundles CDN (`Config.bundles_base_url`, https) and the release public key are configured.
## It reads the signed bundle index `<bundles_base_url>/index/<client_version>.json` (+ `.sig`) with a plain public GET
## (no API, no token, no `X-Tenant-Id` — nothing needs a student login), checks it with ManifestVerifier, and downloads
## the packs of the device's grade bands (`device.cfg` `grade_bands`, empty = all) through ContentDownloader, which uses
## preinstalled packs of the school kit instead of downloading them again. A bad index is skipped as a whole and
## reported as `client_error` `bundle_index_invalid` (spec Edge Cases). Public media of an earlier session are not
## prefetched yet (no cached content manifest on the device — open point of F133 T038).

signal prefetch_completed(downloaded: int, failed: int)

const SKIPPED: String = "PREFETCH_SKIPPED"
const ERR_INDEX_UNAVAILABLE: String = "BUNDLE_INDEX_UNAVAILABLE"
const TELEMETRY_INDEX_INVALID: String = "bundle_index_invalid"
const DEFAULT_WINDOW: String = "18:00-06:00"
const CHECK_INTERVAL_SEC: float = 900.0

var device: DeviceConfig = null
var downloader: ContentDownloader = null
var telemetry: TelemetryService = null
var client_version: String = ""
var bundles_base_url: String = ""
var public_key: String = ""
## Minutes of the day [start, end) of the prefetch window.
var window: Vector2i = parse_window(DEFAULT_WINDOW)
## Coroutine `(url: String) -> PackedByteArray` (empty = failed): a public GET without headers. Tests: TEST-ONLY fakes.
var index_fetcher: Callable = Callable()
var host: Node = null
var _running: bool = false


func _init(device_config: DeviceConfig = null, content: ContentDownloader = null,
		telemetry_service: TelemetryService = null) -> void:
	device = device_config if device_config != null else DeviceConfig.parse("", "other")
	downloader = content
	telemetry = telemetry_service
	window = parse_window(String(ProjectSettings.get_setting("eduworld/content/offhours_prefetch_window",
			DEFAULT_WINDOW)))


## "HH:MM-HH:MM" → Vector2i(start_minute, end_minute); an invalid text gives the default window.
static func parse_window(text: String) -> Vector2i:
	var m: RegExMatch = RegEx.create_from_string("^([01][0-9]|2[0-3]):([0-5][0-9])-([01][0-9]|2[0-3]):([0-5][0-9])$") \
			.search(text.strip_edges())
	if m == null:
		return parse_window(DEFAULT_WINDOW) if text != DEFAULT_WINDOW else Vector2i(18 * 60, 6 * 60)
	return Vector2i(int(m.get_string(1)) * 60 + int(m.get_string(2)), int(m.get_string(3)) * 60 + int(m.get_string(4)))


static func in_window(minute_of_day: int, w: Vector2i) -> bool:
	if w.x == w.y:
		return false
	if w.x < w.y:
		return minute_of_day >= w.x and minute_of_day < w.y
	return minute_of_day >= w.x or minute_of_day < w.y


static func local_minute_of_day() -> int:
	var t: Dictionary = Time.get_time_dict_from_system()
	return int(t["hour"]) * 60 + int(t["minute"])


func should_run(minute_of_day: int) -> bool:
	return device.shared_device and device.platform != "web" and in_window(minute_of_day, window) \
			and downloader != null and downloader.next_zone_prefetch_allowed() \
			and bundles_base_url.begins_with("https://") and not public_key.is_empty() and not client_version.is_empty()


## Coroutine: one prefetch pass. Result.value = number of packs now on the device (downloaded or already there).
func run(minute_of_day: int = local_minute_of_day()) -> Result:
	if _running or not should_run(minute_of_day):
		return Result.failure(SKIPPED)
	_running = true
	var result: Result = await _run_once()
	_running = false
	return result


func _run_once() -> Result:
	var url: String = "%s/index/%s.json" % [bundles_base_url.trim_suffix("/"), client_version.uri_encode()]
	var body: PackedByteArray = await _fetch_public(url)
	var sig: PackedByteArray = await _fetch_public(url + ".sig")
	if body.is_empty() or sig.is_empty():
		return Result.failure(ERR_INDEX_UNAVAILABLE)
	var checked: Result = ManifestVerifier.verify_bundle_index(body, sig.get_string_from_utf8(), public_key,
			client_version)
	if not checked.ok:
		if telemetry != null:
			telemetry.queue_client_error(TELEMETRY_INDEX_INVALID)
		return checked
	var ready: int = 0
	var failed: int = 0
	for item: Variant in (checked.value as Dictionary)["bundles"] as Array:
		var entry: Dictionary = item as Dictionary
		if not _wanted(entry):
			continue
		if not downloader.next_zone_prefetch_allowed():
			break
		var got: Result = await downloader.fetch_bundle(entry)
		if got.ok:
			ready += 1
		else:
			failed += 1
	prefetch_completed.emit(ready, failed)
	return Result.success(ready)


func _wanted(entry: Dictionary) -> bool:
	if device.grade_bands.is_empty():
		return true
	for band: Variant in entry.get("grade_bands", []) as Array:
		if String(band) in device.grade_bands:
			return true
	return false


func _fetch_public(url: String) -> PackedByteArray:
	if index_fetcher.is_valid():
		return await index_fetcher.call(url)
	return await _http_get(url)


func _http_get(url: String) -> PackedByteArray:
	if host == null or not host.is_inside_tree():
		return PackedByteArray()
	var req: HTTPRequest = HTTPRequest.new()
	req.timeout = 60.0
	host.add_child(req)
	var out: PackedByteArray = PackedByteArray()
	if req.request(url) == OK:
		var res: Array = await req.request_completed
		if int(res[0]) == HTTPRequest.RESULT_SUCCESS and int(res[1]) == 200:
			out = res[3] as PackedByteArray
	req.queue_free()
	return out


## Checks every CHECK_INTERVAL_SEC on `host_node` (shared Android devices only: the school kit / MDM case).
func attach(host_node: Node) -> void:
	host = host_node
	if host == null or not device.shared_device or device.platform != "android":
		return
	var timer: Timer = Timer.new()
	timer.wait_time = CHECK_INTERVAL_SEC
	timer.autostart = true
	timer.timeout.connect(func() -> void: run())
	host.add_child(timer)
