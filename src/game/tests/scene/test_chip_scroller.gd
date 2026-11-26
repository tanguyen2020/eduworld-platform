extends GdUnitTestSuite
## F099-AC-18 (F098 open point: Shop filter chips cut at 1024×600 in English): a chip row that does not fit shows an
## arrow on each side (the one at an end of the row disabled), an arrow press scrolls the row, and a row that fits
## shows no arrow. The shop filter row uses it. TEST-ONLY fixtures.


func before_test() -> void:
	AppState.start_ui(get_tree().root)
	AppState.set_grade_band("g4_5")


func _row(count: int) -> FilterChips:
	var chips: FilterChips = FilterChips.new()
	var options: Array[Dictionary] = []
	for i: int in count:
		options.append({"code": "c%d" % i, "key": "ui.inventory.kind.outfit"})
	chips.set_options(options)
	return chips


func _mount(width: float, count: int) -> ChipScroller:
	var vp: SubViewport = auto_free(SubViewport.new()) as SubViewport
	vp.size = Vector2i(1280, 720)
	add_child(vp)
	var box: VBoxContainer = VBoxContainer.new()
	box.theme = AppState.theme_service.current_theme
	box.size = Vector2(width, 200)
	vp.add_child(box)
	var scroller: ChipScroller = UiBuild.chip_scroller(_row(count)) as ChipScroller
	box.add_child(scroller)
	return scroller


func test_long_row_shows_arrows_and_scrolls() -> void:
	var s: ChipScroller = _mount(500.0, 8)
	for i: int in 4:
		await await_idle_frame()
	assert_bool(s.overflows()).is_true()
	assert_bool(s.prev.visible and s.next.visible).is_true()
	assert_bool(s.prev.disabled).is_true()
	assert_bool(s.next.disabled).is_false()
	s.next.pressed.emit()
	for i: int in 2:
		await await_idle_frame()
	assert_int(s.scroll.scroll_horizontal).is_greater(0)
	assert_bool(s.prev.disabled).is_false()
	# Touch target of the band, arrow icon.
	assert_float(s.next.size.y + 0.5).is_greater_equal(float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT["g4_5"]))
	assert_object(s.next.icon).is_not_null()


func test_short_row_shows_no_arrow() -> void:
	var s: ChipScroller = _mount(1200.0, 2)
	for i: int in 4:
		await await_idle_frame()
	assert_bool(s.overflows()).is_false()
	assert_bool(s.prev.visible or s.next.visible).is_false()


func test_shop_filter_row_uses_the_scroller() -> void:
	var source: String = FileAccess.get_file_as_string("res://scenes/inventory/inventory_shop_screen.gd")
	assert_str(source).contains("UiBuild.chip_scroller(_filters)")
	var built: Control = UiBuild.chip_scroller(_row(1))
	assert_object(built).is_instanceof(ChipScroller)
	built.free()
