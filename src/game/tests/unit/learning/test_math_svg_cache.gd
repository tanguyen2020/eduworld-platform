extends GdUnitTestSuite
## F063-AC-18, F063-AC-20 (TC-MATH-003; T026): MathSvgCache downloads `math/<sha256>.svg` from the CDN, checks the
## sha256 and the `math_svg_max` size, keeps an immutable `user://cache/math/<sha256>.svg`, rasterizes once per
## `(sha256, scale, colour)`; a failed download, a wrong sha256 or an oversized SVG returns null (the view shows
## `speech_text`) and sends `math_svg_failed` telemetry — never a crash.

const CACHE_DIR: String = "user://test_f063_math"
const BLUE: Color = Color(0.1176, 0.1647, 0.2667, 1)

var http: FakeHttp = null
var cache: MathSvgCache = null
var failures: Array[String] = []


func before_test() -> void:
	_wipe()
	http = FakeHttp.new()
	cache = MathSvgCache.new(http, "https://cdn.example.invalid")
	cache.cache_dir = CACHE_DIR
	failures.clear()
	cache.failure_handler = func(reason: String) -> void: failures.append(reason)


func after_test() -> void:
	_wipe()


func _wipe() -> void:
	var dir: DirAccess = DirAccess.open(CACHE_DIR)
	if dir != null:
		for f: String in dir.get_files():
			DirAccess.remove_absolute(CACHE_DIR.path_join(f))
		DirAccess.remove_absolute(CACHE_DIR)


func _asset() -> MathAssetRefModel:
	var attempt: AttemptModel = AttemptModel.from_dict(LearningFixtures.attempt())
	return attempt.render.math_assets[0]


func _serve() -> void:
	http.reply_bytes(LearningFixtures.ROUTE_MATH, 200, LearningFixtures.math_svg())


func test_svg_is_checked_and_cached_by_sha256() -> void:
	_serve()
	var svg: String = await cache.svg_text(_asset())
	assert_str(svg).starts_with("<svg")
	var path: String = CACHE_DIR.path_join("%s.svg" % LearningFixtures.MATH_SHA)
	assert_bool(FileAccess.file_exists(path)).is_true()
	assert_str(MathSvgCache.sha256_hex(FileAccess.get_file_as_bytes(path))).is_equal(LearningFixtures.MATH_SHA)
	assert_str(await cache.svg_text(_asset())).is_equal(svg)
	assert_int(http.count(LearningFixtures.ROUTE_MATH)).is_equal(1)
	assert_bool(http.requests[0].headers.has("authorization")).is_false()
	assert_array(failures).is_empty()


func test_render_payload_block_is_fetched_from_the_cdn_by_its_key() -> void:
	_serve()
	var q: QuestionRenderModel = LearningFixtures.question("numeric")
	var block: MathAssetRefModel = RichText.math_of(q.stem())[0]
	assert_str(block.svg_url).is_empty()
	assert_object(await cache.texture(block, 1.0, BLUE)).is_not_null()
	assert_str(http.requests[0].url).is_equal("https://cdn.example.invalid/math/%s.svg" % LearningFixtures.MATH_SHA)


func test_texture_is_cached_per_sha_scale_and_colour() -> void:
	_serve()
	var a: Texture2D = await cache.texture(_asset(), 1.0, BLUE)
	var b: Texture2D = await cache.texture(_asset(), 1.0, BLUE)
	assert_object(a).is_not_null()
	assert_bool(a == b).is_true()
	assert_int(cache.cached_texture_count()).is_equal(1)
	var big: Texture2D = await cache.texture(_asset(), 2.0, BLUE)
	await cache.texture(_asset(), 1.0, Color.WHITE)
	assert_int(cache.cached_texture_count()).is_equal(3)
	assert_int(big.get_width()).is_between(a.get_width() * 2 - 1, a.get_width() * 2 + 1)
	assert_str(MathSvgCache.texture_key(_asset(), 1.0, BLUE)).starts_with(LearningFixtures.MATH_SHA)
	assert_int(http.count(LearningFixtures.ROUTE_MATH)).is_equal(1)


func test_wrong_sha256_is_refused_and_not_cached() -> void:
	var bytes: PackedByteArray = LearningFixtures.math_svg()
	bytes.append_array("<!-- tampered -->".to_utf8_buffer())
	http.reply_bytes(LearningFixtures.ROUTE_MATH, 200, bytes)
	assert_object(await cache.texture(_asset(), 1.0, BLUE)).is_null()
	assert_array(failures).is_equal([MathSvgCache.FAIL_SHA_MISMATCH])
	assert_bool(FileAccess.file_exists(CACHE_DIR.path_join("%s.svg" % LearningFixtures.MATH_SHA))).is_false()


func test_oversized_svg_is_refused() -> void:
	var big: PackedByteArray = PackedByteArray()
	big.resize(MathSvgCache.MAX_SVG_BYTES + 1)
	http.reply_bytes(LearningFixtures.ROUTE_MATH, 200, big)
	assert_object(await cache.texture(_asset(), 1.0, BLUE)).is_null()
	assert_array(failures).is_equal([MathSvgCache.FAIL_TOO_LARGE])


func test_download_failure_returns_null() -> void:
	http.fail(LearningFixtures.ROUTE_MATH)
	assert_object(await cache.texture(_asset(), 1.0, BLUE)).is_null()
	http.reply(LearningFixtures.ROUTE_MATH, 404, {})
	assert_object(await cache.texture(_asset(), 1.0, BLUE)).is_null()
	assert_array(failures).is_equal([MathSvgCache.FAIL_DOWNLOAD, MathSvgCache.FAIL_DOWNLOAD])


func test_corrupted_cache_file_is_downloaded_again() -> void:
	DirAccess.make_dir_recursive_absolute(CACHE_DIR)
	var f: FileAccess = FileAccess.open(CACHE_DIR.path_join("%s.svg" % LearningFixtures.MATH_SHA), FileAccess.WRITE)
	f.store_string("<svg>broken</svg>")
	f.close()
	_serve()
	assert_str(await cache.svg_text(_asset())).is_equal(LearningFixtures.math_svg().get_string_from_utf8())
	assert_int(http.count(LearningFixtures.ROUTE_MATH)).is_equal(1)


func test_failure_shows_speech_text_and_queues_telemetry() -> void:
	var h: AuthHarness = AuthHarness.create()
	await h.sign_in()
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	services.math_svg.cache_dir = CACHE_DIR
	h.http.fail(LearningFixtures.ROUTE_MATH)
	var view: MathInline = auto_free((load("res://scenes/learning/math/MathInline.tscn") as PackedScene).instantiate()) as MathInline
	add_child(view)
	var failed: Array[bool] = []
	view.math_failed.connect(func() -> void: failed.append(true))
	await view.show_formula(_asset(), services.math_svg)
	assert_bool(view.loaded).is_false()
	assert_str((view.get_node("%Fallback") as Label).text).is_equal("ba phần tư")
	assert_bool((view.get_node("%Fallback") as Label).visible).is_true()
	assert_array(failed).has_size(1)
	var codes: Array = []
	for event: Dictionary in services.telemetry.queue:
		codes.append(String(event.get("error_code", "")))
	assert_array(codes).contains([LearningService.TELEMETRY_MATH_FAILED])
	h.dispose()


func test_prepare_svg_tints_and_sizes() -> void:
	var out: String = MathSvgCache.prepare_svg(LearningFixtures.math_svg().get_string_from_utf8(), Color(1, 0, 0))
	assert_str(out).not_contains("currentColor")
	assert_str(out).contains("#ff0000")
	assert_str(out).contains("width=\"19.20\"")
	assert_str(out).contains("height=\"41.60\"")
