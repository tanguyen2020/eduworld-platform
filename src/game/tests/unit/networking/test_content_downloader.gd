extends GdUnitTestSuite
## F133 T031 (F133-AC-18, F133-AC-21, PERFORMANCE_BUDGET §10.1, spec Edge Cases, ADR-0044): DeviceConfig drops a
## `lan_cache_url` that is not a valid http(s) URL or is longer than 2048 characters (with a warning); ContentDownloader
## takes a bundle from the cache, then the preinstalled school kit folder, then the LAN cache, then the CDN; verifies
## SHA-256 of every pack (a changed pack on the LAN is refused and the CDN is used); never sends anything but a
## content-addressed key to the LAN (no API path); counts `content_download` per source (cache hits send nothing);
## evicts least-recently-used packs above the cache limit. Fetches go to a TEST-ONLY fake (no network).

const FIX: String = "res://tests/fixtures/release"
const SHA_MATH: String = "8edf23eda98938e5a87cc31152721d01cc6b34ce951d10ff8991d74f139d5bb7"
const SHA_SCI: String = "5d63107c5862ced567a532fb8bfca97c50e2d69c1dcbdcd4eaacfdabe599857e"
const CACHE: String = "user://test_content_cache"
const PREINSTALLED: String = "user://test_preinstalled"
const LAN: String = "http://10.0.0.5:8080"

var telemetry: TelemetryService = null
var fetched: Array[String] = []
## url → bytes served by the fake (missing → 404).
var served: Dictionary = {}


func before_test() -> void:
	telemetry = TelemetryService.new()
	fetched.clear()
	served.clear()
	_clear(CACHE)
	_clear(PREINSTALLED)


func after_test() -> void:
	_clear(CACHE)
	_clear(PREINSTALLED)


func _clear(dir_path: String) -> void:
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return
	for f: String in dir.get_files():
		dir.remove(f)
	DirAccess.remove_absolute(dir_path)


func _pack(sha: String) -> PackedByteArray:
	return FileAccess.get_file_as_bytes(FIX.path_join("%s.pck" % sha))


func _entry(sha: String, bundle_id: String = "zone.math_g3") -> Dictionary:
	return {"bundle_id": bundle_id, "version": 1, "sha256": sha, "size_bytes": _pack(sha).size(),
		"url": "https://bundles.eduworld.example/bundles/%s.pck" % sha, "grade_bands": ["g1_3"]}


func _fake_fetch(url: String, dest: String) -> bool:
	fetched.append(url)
	await Engine.get_main_loop().process_frame
	if not served.has(url):
		return false
	var f: FileAccess = FileAccess.open(dest, FileAccess.WRITE)
	f.store_buffer(served[url] as PackedByteArray)
	f.close()
	return true


func _downloader(cfg_text: String = "") -> ContentDownloader:
	var device: DeviceConfig = DeviceConfig.parse(cfg_text, "android")
	var d: ContentDownloader = ContentDownloader.new(device, telemetry)
	d.cache_dir = CACHE
	d.fetcher = _fake_fetch
	return d


func _sources() -> Dictionary:
	var out: Dictionary = {}
	for e: Dictionary in telemetry.queue:
		if String(e["type"]) == TelemetryService.TYPE_CONTENT_DOWNLOAD:
			out[String(e["source"])] = int(out.get(String(e["source"]), 0)) + int(e["bytes"])
			assert_array(e.keys()).contains_exactly_in_any_order(["type", "client_event_id", "occurred_at", "source", "bytes"])
	return out


# ---------- DeviceConfig (F133-AC-18 length/format rule) ----------

func test_device_config_keeps_a_valid_lan_url_and_drops_bad_ones_with_a_warning() -> void:
	var ok: DeviceConfig = DeviceConfig.parse("[device]\nshared_device=true\nlan_cache_url=\"%s/\"\n" % LAN, "android")
	assert_str(ok.lan_cache_url).is_equal(LAN)
	assert_bool(ok.shared_device).is_true()
	assert_array(ok.warnings).is_empty()
	for bad: String in ["ftp://x", "javascript:alert(1)", "http://", "https://user:pw@cache.local",
			"https://cache.local/?q=1", "http://%s" % "a".repeat(2050), "cache.local"]:
		var cfg: DeviceConfig = DeviceConfig.parse("[device]\nlan_cache_url=\"%s\"\n" % bad, "android")
		assert_str(cfg.lan_cache_url).override_failure_message(bad).is_empty()
		assert_array(cfg.warnings).contains(["lan_cache_url_invalid"])


func test_device_config_defaults_and_web() -> void:
	var cfg: DeviceConfig = DeviceConfig.parse("", "android")
	assert_bool(cfg.low_data_mode).is_false()
	assert_str(cfg.preinstalled_bundles_dir).is_equal(DeviceConfig.DEFAULT_PREINSTALLED_DIR)
	var web: DeviceConfig = DeviceConfig.parse("[device]\nshared_device=false\nlan_cache_url=\"%s\"\n" % LAN, "web")
	assert_bool(web.shared_device).is_true()
	assert_str(web.lan_cache_url).is_empty()
	var long_dir: DeviceConfig = DeviceConfig.parse("[device]\npreinstalled_bundles_dir=\"user://%s\"\n" % "d".repeat(600),
			"android")
	assert_str(long_dir.preinstalled_bundles_dir).is_equal(DeviceConfig.DEFAULT_PREINSTALLED_DIR)
	assert_array(long_dir.warnings).contains(["preinstalled_bundles_dir_invalid"])
	var bands: DeviceConfig = DeviceConfig.parse("[device]\ngrade_bands=[\"g1_3\", \"g99\"]\n", "android")
	assert_array(bands.grade_bands).contains_exactly(["g1_3"])


# ---------- source order + hash ----------

func test_order_is_cache_then_preinstalled_then_lan_then_cdn() -> void:
	DirAccess.make_dir_recursive_absolute(PREINSTALLED)
	var f: FileAccess = FileAccess.open(PREINSTALLED.path_join("%s.pck" % SHA_MATH), FileAccess.WRITE)
	f.store_buffer(_pack(SHA_MATH))
	f.close()
	var d: ContentDownloader = _downloader("[device]\nlan_cache_url=\"%s\"\npreinstalled_bundles_dir=\"%s\"\n" % [
		LAN, PREINSTALLED])
	var r: Result = await d.fetch_bundle(_entry(SHA_MATH))
	assert_bool(r.ok).is_true()
	assert_str(String(r.details["source"])).is_equal(TelemetryService.DOWNLOAD_SOURCE_PREINSTALLED)
	assert_str(String(r.value)).is_equal(PREINSTALLED.path_join("%s.pck" % SHA_MATH))
	assert_array(fetched).is_empty()
	served["%s/bundles/%s.pck" % [LAN, SHA_SCI]] = _pack(SHA_SCI)
	r = await d.fetch_bundle(_entry(SHA_SCI, "zone.science_g4"))
	assert_str(String(r.details["source"])).is_equal(TelemetryService.DOWNLOAD_SOURCE_LAN_CACHE)
	assert_array(fetched).contains_exactly(["%s/bundles/%s.pck" % [LAN, SHA_SCI]])
	fetched.clear()
	r = await d.fetch_bundle(_entry(SHA_SCI, "zone.science_g4"))
	assert_str(String(r.details["source"])).is_equal(ContentDownloader.SOURCE_CACHE)
	assert_array(fetched).is_empty()
	var sizes: Dictionary = _sources()
	assert_int(int(sizes.get("preinstalled", 0))).is_equal(_pack(SHA_MATH).size())
	assert_int(int(sizes.get("lan_cache", 0))).is_equal(_pack(SHA_SCI).size())
	assert_bool(sizes.has("cdn")).is_false()


func test_changed_pack_on_the_lan_is_refused_and_the_cdn_is_used() -> void:
	var d: ContentDownloader = _downloader("[device]\nlan_cache_url=\"%s\"\n" % LAN)
	served["%s/bundles/%s.pck" % [LAN, SHA_MATH]] = "tampered".to_utf8_buffer()
	served[String(_entry(SHA_MATH)["url"])] = _pack(SHA_MATH)
	var r: Result = await d.fetch_bundle(_entry(SHA_MATH))
	assert_bool(r.ok).is_true()
	assert_str(String(r.details["source"])).is_equal(TelemetryService.DOWNLOAD_SOURCE_CDN)
	assert_bool(ManifestVerifier.matches_sha256(String(r.value), SHA_MATH)).is_true()
	assert_int(fetched.size()).is_equal(2)
	assert_bool(_sources().has("lan_cache")).is_false()


func test_lan_down_falls_back_to_cdn_and_a_bad_cdn_pack_is_an_error_not_a_file() -> void:
	var d: ContentDownloader = _downloader("[device]\nlan_cache_url=\"%s\"\n" % LAN)
	served[String(_entry(SHA_MATH)["url"])] = "wrong".to_utf8_buffer()
	var r: Result = await d.fetch_bundle(_entry(SHA_MATH))
	assert_bool(r.ok).is_false()
	assert_str(r.code).is_equal(ContentDownloader.ERR_HASH_MISMATCH)
	assert_bool(FileAccess.file_exists(CACHE.path_join("%s.pck" % SHA_MATH))).is_false()
	assert_dict(_sources()).is_empty()


func test_only_content_addressed_keys_go_to_the_lan_never_api_paths() -> void:
	var d: ContentDownloader = _downloader("[device]\nlan_cache_url=\"%s\"\n" % LAN)
	assert_str(d.lan_url_for("https://api.eduworld.example/api/v1/game/content-manifest")).is_empty()
	assert_str(d.lan_url_for("https://cdn.eduworld.example/media/%s/720p.ogv?sig=x" % SHA_MATH)).is_empty()
	assert_str(d.lan_url_for("https://bundles.eduworld.example/bundles/%s.pck" % SHA_MATH)) \
			.is_equal("%s/bundles/%s.pck" % [LAN, SHA_MATH])
	var bad: Dictionary = _entry(SHA_MATH)
	bad["url"] = "https://api.eduworld.example/api/v1/game/world/manifest"
	var r: Result = await d.fetch_bundle(bad)
	assert_str(r.code).is_equal(ContentDownloader.ERR_BAD_ENTRY)
	assert_array(fetched).is_empty()


func test_cache_above_the_limit_drops_least_recently_used_packs() -> void:
	var d: ContentDownloader = _downloader()
	d.cache_limit_bytes = _pack(SHA_MATH).size() + 4
	served[String(_entry(SHA_MATH)["url"])] = _pack(SHA_MATH)
	served[String(_entry(SHA_SCI)["url"])] = _pack(SHA_SCI)
	await d.fetch_bundle(_entry(SHA_MATH))
	await d.fetch_bundle(_entry(SHA_SCI, "zone.science_g4"))
	assert_bool(FileAccess.file_exists(CACHE.path_join("%s.pck" % SHA_MATH))).is_false()
	assert_bool(FileAccess.file_exists(CACHE.path_join("%s.pck" % SHA_SCI))).is_true()
	assert_int(int(_sources().get("cdn", 0))).is_equal(_pack(SHA_MATH).size() + _pack(SHA_SCI).size())


func test_low_data_mode_from_device_or_probe() -> void:
	var d: ContentDownloader = _downloader("[device]\nlow_data_mode=true\n")
	assert_bool(d.is_low_data()).is_true()
	assert_str(d.media_quality()).is_equal(ContentDownloader.QUALITY_LOW)
	assert_bool(d.next_zone_prefetch_allowed()).is_false()
	var probe: BandwidthProbe = BandwidthProbe.new(1_000_000)
	var normal: ContentDownloader = _downloader()
	normal.probe = probe
	assert_bool(normal.is_low_data()).is_false()
	probe.record(50_000, 2000)
	assert_bool(normal.is_low_data()).is_true()
	assert_str(normal.media_quality()).is_equal(ContentDownloader.QUALITY_LOW)
	probe.record(10_000_000, 1000)
	assert_bool(normal.is_low_data()).is_false()
