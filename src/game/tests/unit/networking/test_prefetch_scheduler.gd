extends GdUnitTestSuite
## F133 T032 (F133-AC-19, F133-AC-20, PERFORMANCE_BUDGET §10.1, spec Clarifications 2026-11-16): PrefetchScheduler runs
## only on a shared device inside `offhours_prefetch_window` (18:00–06:00, wraps midnight) and not in low data mode;
## it reads the signed public bundle index of this client version from the bundles CDN (no API, no token, no header),
## refuses an index with a bad signature (nothing downloaded, `client_error`), downloads the packs of the device's grade
## bands through ContentDownloader and uses the preinstalled packs instead of downloading them again.
## TEST-ONLY fakes: index fetches and pack downloads are recorded, nothing touches the network.

const FIX: String = "res://tests/fixtures/release"
const BASE: String = "https://bundles.eduworld.example"
const CACHE: String = "user://test_prefetch_cache"
const PRE: String = "user://test_prefetch_pre"
const SHA_MATH: String = "8edf23eda98938e5a87cc31152721d01cc6b34ce951d10ff8991d74f139d5bb7"
const SHA_SCI: String = "5d63107c5862ced567a532fb8bfca97c50e2d69c1dcbdcd4eaacfdabe599857e"

var telemetry: TelemetryService = null
var index_requests: Array[String] = []
var pack_requests: Array[String] = []
var index_sig_override: String = ""


func before_test() -> void:
	telemetry = TelemetryService.new()
	index_requests.clear()
	pack_requests.clear()
	index_sig_override = ""
	_clear(CACHE)
	_clear(PRE)


func after_test() -> void:
	_clear(CACHE)
	_clear(PRE)


func _clear(dir_path: String) -> void:
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return
	for f: String in dir.get_files():
		dir.remove(f)
	DirAccess.remove_absolute(dir_path)


func _fetch_index(url: String) -> PackedByteArray:
	index_requests.append(url)
	await Engine.get_main_loop().process_frame
	if url == "%s/index/1.2.3.json" % BASE:
		return FileAccess.get_file_as_bytes(FIX.path_join("bundle_index.json"))
	if url == "%s/index/1.2.3.json.sig" % BASE:
		if not index_sig_override.is_empty():
			return index_sig_override.to_utf8_buffer()
		return FileAccess.get_file_as_bytes(FIX.path_join("bundle_index.json.sig"))
	return PackedByteArray()


func _fetch_pack(url: String, dest: String) -> bool:
	pack_requests.append(url)
	await Engine.get_main_loop().process_frame
	var sha: String = url.get_file().get_basename()
	var src: String = FIX.path_join("%s.pck" % sha)
	if not FileAccess.file_exists(src):
		return false
	var f: FileAccess = FileAccess.open(dest, FileAccess.WRITE)
	f.store_buffer(FileAccess.get_file_as_bytes(src))
	f.close()
	return true


func _scheduler(cfg_text: String) -> PrefetchScheduler:
	var device: DeviceConfig = DeviceConfig.parse(cfg_text, "android", false)
	var downloader: ContentDownloader = ContentDownloader.new(device, telemetry)
	downloader.cache_dir = CACHE
	downloader.fetcher = _fetch_pack
	var s: PrefetchScheduler = PrefetchScheduler.new(device, downloader, telemetry)
	s.client_version = "1.2.3"
	s.bundles_base_url = BASE
	s.public_key = FileAccess.get_file_as_string(FIX.path_join("release_public.pem"))
	s.index_fetcher = _fetch_index
	return s


func test_window_parsing_and_wrap_around_midnight() -> void:
	var w: Vector2i = PrefetchScheduler.parse_window("18:00-06:00")
	assert_vector(w).is_equal(Vector2i(18 * 60, 6 * 60))
	assert_bool(PrefetchScheduler.in_window(23 * 60, w)).is_true()
	assert_bool(PrefetchScheduler.in_window(2 * 60, w)).is_true()
	assert_bool(PrefetchScheduler.in_window(18 * 60, w)).is_true()
	assert_bool(PrefetchScheduler.in_window(6 * 60, w)).is_false()
	assert_bool(PrefetchScheduler.in_window(10 * 60, w)).is_false()
	assert_bool(PrefetchScheduler.in_window(13 * 60, PrefetchScheduler.parse_window("12:00-14:00"))).is_true()
	assert_vector(PrefetchScheduler.parse_window("25:00-xx")).is_equal(PrefetchScheduler.parse_window(
			PrefetchScheduler.DEFAULT_WINDOW))


func test_only_shared_devices_inside_the_window_and_not_in_low_data_mode() -> void:
	assert_bool(_scheduler("[device]\nshared_device=false\n").should_run(22 * 60)).is_false()
	assert_bool(_scheduler("[device]\nshared_device=true\n").should_run(10 * 60)).is_false()
	assert_bool(_scheduler("[device]\nshared_device=true\nlow_data_mode=true\n").should_run(22 * 60)).is_false()
	assert_bool(_scheduler("[device]\nshared_device=true\n").should_run(22 * 60)).is_true()
	var web: PrefetchScheduler = _scheduler("")
	web.device = DeviceConfig.parse("", "web")
	assert_bool(web.should_run(22 * 60)).is_false()
	var skipped: Result = await _scheduler("[device]\nshared_device=true\n").run(10 * 60)
	assert_str(skipped.code).is_equal(PrefetchScheduler.SKIPPED)
	assert_array(index_requests).is_empty()


func test_downloads_the_signed_index_packs_of_the_device_bands_without_any_api_call() -> void:
	var s: PrefetchScheduler = _scheduler("[device]\nshared_device=true\ngrade_bands=[\"g1_3\"]\n")
	var r: Result = await s.run(22 * 60)
	assert_bool(r.ok).override_failure_message(r.code).is_true()
	assert_int(int(r.value)).is_equal(1)
	assert_array(index_requests).contains_exactly(["%s/index/1.2.3.json" % BASE, "%s/index/1.2.3.json.sig" % BASE])
	assert_array(pack_requests).contains_exactly(["%s/bundles/%s.pck" % [BASE, SHA_MATH]])
	for url: String in index_requests + pack_requests:
		assert_str(url).not_contains("/api/")


func test_preinstalled_packs_are_used_instead_of_downloading_again() -> void:
	DirAccess.make_dir_recursive_absolute(PRE)
	for sha: String in [SHA_MATH, SHA_SCI]:
		var f: FileAccess = FileAccess.open(PRE.path_join("%s.pck" % sha), FileAccess.WRITE)
		f.store_buffer(FileAccess.get_file_as_bytes(FIX.path_join("%s.pck" % sha)))
		f.close()
	var s: PrefetchScheduler = _scheduler("[device]\nshared_device=true\npreinstalled_bundles_dir=\"%s\"\n" % PRE)
	var r: Result = await s.run(23 * 60)
	assert_bool(r.ok).is_true()
	assert_array(pack_requests).is_empty()


func test_bad_index_signature_downloads_nothing_and_reports_client_error() -> void:
	var s: PrefetchScheduler = _scheduler("[device]\nshared_device=true\n")
	index_sig_override = Marshalls.raw_to_base64("x".repeat(384).to_utf8_buffer())
	var r: Result = await s.run(22 * 60)
	assert_str(r.code).is_equal(ManifestVerifier.ERR_SIGNATURE)
	assert_array(pack_requests).is_empty()
	var codes: Array[String] = []
	for e: Dictionary in telemetry.queue:
		if String(e["type"]) == TelemetryService.TYPE_CLIENT_ERROR:
			codes.append(String(e.get("error_code", "")))
	assert_array(codes).contains([PrefetchScheduler.TELEMETRY_INDEX_INVALID])


func test_no_bundles_cdn_configured_means_no_prefetch() -> void:
	var s: PrefetchScheduler = _scheduler("[device]\nshared_device=true\n")
	s.bundles_base_url = ""
	assert_bool(s.should_run(22 * 60)).is_false()
	s.bundles_base_url = "http://bundles.example"
	assert_bool(s.should_run(22 * 60)).is_false()
