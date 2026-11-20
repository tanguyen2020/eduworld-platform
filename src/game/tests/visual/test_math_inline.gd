extends GdUnitTestSuite
## F063-AC-19 (TC-MATH-001; T027): the sample formula renders through MathInline as a block (centred) or inline in a
## RichTextLabel, rasterized sharper at a higher DPI scale and tinted with the Theme text colour (checked on the
## rasterized pixels — the fraction bar has the Theme colour, the background stays transparent). No formula input
## exists anywhere in the player. Headless: checks the rasterized image, not a screenshot (baseline screenshots of
## the player: T035).

const CACHE_DIR: String = "user://test_f063_math_visual"

var http: FakeHttp = null
var cache: MathSvgCache = null


func before_test() -> void:
	http = FakeHttp.new()
	http.reply_bytes(LearningFixtures.ROUTE_MATH, 200, LearningFixtures.math_svg())
	cache = MathSvgCache.new(http, "https://cdn.example.invalid")
	cache.cache_dir = CACHE_DIR


func after_test() -> void:
	DirAccess.remove_absolute(CACHE_DIR.path_join("%s.svg" % LearningFixtures.MATH_SHA))
	DirAccess.remove_absolute(CACHE_DIR)


func _asset(display: String = "block") -> MathAssetRefModel:
	var data: Dictionary = LearningFixtures.hint()["hint"]["math_assets"][0] as Dictionary
	data = data.duplicate()
	data["display"] = display
	return MathAssetRefModel.from_dict(data)


func test_block_formula_is_centred_and_tinted_with_the_theme_colour() -> void:
	var view: MathInline = auto_free((load("res://scenes/learning/math/MathInline.tscn") as PackedScene).instantiate()) as MathInline
	add_child(view)
	await view.show_formula(_asset("block"), cache)
	assert_bool(view.loaded).is_true()
	assert_int(view.alignment).is_equal(BoxContainer.ALIGNMENT_CENTER)
	var image: Image = (view.get_node("%Image") as TextureRect).texture.get_image()
	var theme_color: Color = MathInline.text_color(view)
	var bar: Color = image.get_pixel(image.get_width() / 2, int(image.get_height() * 25.5 / 52.0))
	assert_float(bar.a).is_greater(0.8)
	# 8-bit rasterization: each channel within 2/255 of the Theme colour.
	for pair: Array in [[bar.r, theme_color.r], [bar.g, theme_color.g], [bar.b, theme_color.b]]:
		assert_float(absf(float(pair[0]) - float(pair[1]))).override_failure_message("bar %s theme %s" % [bar, theme_color]) \
				.is_less(0.01)
	assert_float(image.get_pixel(0, image.get_height() / 2 - 8).a).is_less(0.1)


func test_higher_dpi_scale_gives_a_larger_texture() -> void:
	var low: Texture2D = await cache.texture(_asset(), MathSvgCache.scale_for(18, 1.0), Color.BLACK)
	var high: Texture2D = await cache.texture(_asset(), MathSvgCache.scale_for(18, 2.0), Color.BLACK)
	assert_int(high.get_height()).is_greater(low.get_height() * 2 - 2)
	assert_float(MathSvgCache.scale_for(32, 1.0)).is_greater(MathSvgCache.scale_for(18, 1.0))


func test_inline_formula_goes_into_a_rich_text_label() -> void:
	var label: RichTextLabel = auto_free(RichTextLabel.new()) as RichTextLabel
	label.fit_content = true
	add_child(label)
	label.add_text("Tính ")
	assert_bool(await MathInline.append_to(label, _asset("inline"), cache)).is_true()
	label.add_text(" của 8.")
	assert_str(label.get_parsed_text()).not_contains("ba phần tư")
	assert_str(label.get_parsed_text()).starts_with("Tính ")


func test_stem_formula_is_drawn_by_math_inline_without_any_input() -> void:
	var view: RichContentView = auto_free((load("res://scenes/learning/RichContentView.tscn") as PackedScene).instantiate()) as RichContentView
	add_child(view)
	var ctx: RendererContext = RendererContext.new()
	ctx.math = cache
	var q: QuestionRenderModel = LearningFixtures.question("numeric")
	view.show_content(q.stem(), ctx, q)
	for i: int in 10:
		await await_idle_frame()
	assert_int(view.formulas.size()).is_equal(1)
	assert_bool(view.formulas[0].loaded).is_true()
	assert_int(view.find_children("*", "LineEdit", true, false).size()).is_equal(0)
	assert_int(view.find_children("*", "TextEdit", true, false).size()).is_equal(0)
