extends GdUnitTestSuite
## Client telemetry `content_download` (F133-AC-21, TC-ANL-003; GAME_ANALYTICS §3.2, MONITORING §3
## `eduworld_client_content_download_bytes_total{source}`, PERFORMANCE_BUDGET §10.1; AGENTS R14): a finished download
## of a content package / public media queues `content_download` with only `source` (`preinstalled`, `lan_cache`,
## `cdn`) and `bytes` (0–10 GiB); downloads of one source are summed into the event waiting for the next batch; an
## unknown source sends nothing; a cached file (no download) sends nothing; the event goes even when product telemetry
## is off. No URL, file name, hash or student identity is sent. TEST-ONLY fakes.

const ROUTE: String = "POST /game/telemetry"
const ALLOWED_KEYS: Array[String] = ["type", "client_event_id", "occurred_at", "source", "bytes"]
const SHA: String = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
const VIDEO_URL: String = "https://cdn.example.test/media/" + SHA + "/720p.ogv?sig=placeholder-signature"
const VIDEO_BYTES: int = 4096

var h: AuthHarness = null
var telemetry: TelemetryService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	telemetry = TelemetryService.new(h.api, h.clock)
	telemetry.product_telemetry_enabled = false
	_remove_cached()


func after_test() -> void:
	_remove_cached()
	h.dispose()


func _remove_cached() -> void:
	var path: String = VideoFileCache.local_path(VIDEO_URL)
	if FileAccess.file_exists(path):
		DirAccess.remove_absolute(path)


func _downloads() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for e: Dictionary in telemetry.queue:
		if String(e["type"]) == TelemetryService.TYPE_CONTENT_DOWNLOAD:
			out.append(e)
	return out


func test_each_source_queues_its_own_event() -> void:
	for source: String in TelemetryService.DOWNLOAD_SOURCES:
		var e: Dictionary = telemetry.queue_content_download(source, 1000)
		assert_str(String(e["source"])).is_equal(source)
		assert_int(int(e["bytes"])).is_equal(1000)
	assert_int(_downloads().size()).is_equal(3)
	assert_array(Array(TelemetryService.DOWNLOAD_SOURCES)).contains_exactly(["preinstalled", "lan_cache", "cdn"])


func test_downloads_of_one_source_are_summed_until_the_next_batch() -> void:
	var first: Dictionary = telemetry.queue_content_download("cdn", 1500)
	telemetry.queue_content_download("cdn", 500)
	telemetry.queue_content_download("lan_cache", 7)
	assert_int(_downloads().size()).is_equal(2)
	assert_int(int(first["bytes"])).is_equal(2000)
	h.http.reply(ROUTE, 202)
	assert_bool((await telemetry.flush(true)).ok).is_true()
	var next: Dictionary = telemetry.queue_content_download("cdn", 10)
	assert_str(String(next["client_event_id"])).is_not_equal(String(first["client_event_id"]))
	assert_int(int(next["bytes"])).is_equal(10)


func test_bytes_are_clamped_to_zero_through_ten_gib() -> void:
	var cap: int = TelemetryService.DOWNLOAD_BYTES_MAX
	assert_int(cap).is_equal(10737418240)
	assert_int(int(telemetry.queue_content_download("cdn", cap + 5)["bytes"])).is_equal(cap)
	telemetry.clear()
	assert_int(int(telemetry.queue_content_download("preinstalled", -3)["bytes"])).is_equal(0)
	telemetry.clear()
	# A sum past the cap fills the waiting event and opens a new one with the rest.
	telemetry.queue_content_download("cdn", cap - 10)
	telemetry.queue_content_download("cdn", 25)
	var events: Array[Dictionary] = _downloads()
	assert_int(events.size()).is_equal(2)
	assert_int(int(events[0]["bytes"])).is_equal(cap)
	assert_int(int(events[1]["bytes"])).is_equal(15)


func test_unknown_source_sends_nothing() -> void:
	for source: String in ["", "lan", "CDN", "https://cdn.example.test"]:
		assert_bool(telemetry.queue_content_download(source, 10).is_empty()).override_failure_message(source).is_true()
	assert_bool(telemetry.queue.is_empty()).is_true()


func test_video_download_from_the_cdn_reports_its_size_once() -> void:
	var cache: VideoFileCache = VideoFileCache.new()
	cache.telemetry = telemetry
	cache.downloader = func(_url: String, path: String) -> bool:
		var f: FileAccess = FileAccess.open(path, FileAccess.WRITE)
		f.store_buffer(_bytes(VIDEO_BYTES))
		f.close()
		return true
	var path: String = await cache.fetch(VIDEO_URL)
	assert_str(path).is_not_empty()
	var events: Array[Dictionary] = _downloads()
	assert_int(events.size()).is_equal(1)
	assert_str(String(events[0]["source"])).is_equal("cdn")
	assert_int(int(events[0]["bytes"])).is_equal(VIDEO_BYTES)
	# Cached: no download, no event.
	assert_str(await cache.fetch(VIDEO_URL)).is_equal(path)
	assert_int(int(_downloads()[0]["bytes"])).is_equal(VIDEO_BYTES)
	assert_int(_downloads().size()).is_equal(1)


func test_failed_video_download_reports_nothing() -> void:
	var cache: VideoFileCache = VideoFileCache.new()
	cache.telemetry = telemetry
	cache.downloader = func(_url: String, _path: String) -> bool: return false
	assert_str(await cache.fetch(VIDEO_URL)).is_empty()
	assert_array(_downloads()).is_empty()


func test_sent_event_carries_no_url_hash_or_identity() -> void:
	AppState.player = AuthHarness.bootstrap()
	var cache: VideoFileCache = VideoFileCache.new()
	cache.telemetry = telemetry
	cache.downloader = func(_url: String, path: String) -> bool:
		var f: FileAccess = FileAccess.open(path, FileAccess.WRITE)
		f.store_buffer(_bytes(16))
		f.close()
		return true
	await cache.fetch(VIDEO_URL)
	h.http.reply(ROUTE, 202)
	assert_bool((await telemetry.flush(true)).ok).is_true()
	var e: Dictionary = (h.http.last(ROUTE).body["events"] as Array)[0] as Dictionary
	for key: Variant in e.keys():
		assert_bool(String(key) in ALLOWED_KEYS).override_failure_message(String(key)).is_true()
	var raw: String = h.http.last(ROUTE).raw_body
	for forbidden: String in [SHA, "cdn.example.test", "720p", "placeholder-signature", "hs.an.3a", "MeoXanh42",
			"bundle_id", "url", "username", "student_id"]:
		assert_str(raw).override_failure_message(forbidden).not_contains(forbidden)


func _bytes(n: int) -> PackedByteArray:
	var out: PackedByteArray = PackedByteArray()
	out.resize(n)
	out.fill(7)
	return out
