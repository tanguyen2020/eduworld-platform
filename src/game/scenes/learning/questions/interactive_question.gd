extends QuestionRendererBase
## `interactive`: draws the widget named by the payload. This client ships the `number_line` widget (a slider over
## `config.min…max` by `config.step`, state `{value}`, each move logged); another widget shows "update required" in
## the item (F063-AC-26) — the student can leave and keep the progress. Emits `{state, action_log}`.

var slider: HSlider = null
var value_label: Label = null
var notice: Label = null


func _make_answer() -> AnswerModel:
	return InteractiveAnswer.new()


func _build() -> void:
	_add_stem(%Stem as Control)
	var model: InteractiveAnswer = answer as InteractiveAnswer
	var box: VBoxContainer = %Answers as VBoxContainer
	if not model.is_supported():
		notice = Label.new()
		notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		notice.text = "ui.activity_player.update_required"
		box.add_child(notice)
		return
	slider = HSlider.new()
	slider.min_value = float(model.config.get("min", 0))
	slider.max_value = float(model.config.get("max", 10))
	slider.step = maxf(0.0001, float(model.config.get("step", 1)))
	slider.tick_count = clampi(int((slider.max_value - slider.min_value) / slider.step) + 1, 2, 51)
	slider.ticks_on_borders = true
	slider.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	slider.value_changed.connect(_on_value_changed)
	box.add_child(slider)
	value_label = Label.new()
	value_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(value_label)


func is_supported() -> bool:
	return (answer as InteractiveAnswer).is_supported()


func _on_value_changed(value: float) -> void:
	if locked:
		return
	var model: InteractiveAnswer = answer as InteractiveAnswer
	model.log_action(context.now_msec() if context != null else 0, "set_value", {"value": value})
	model.set_state("value", value)


func _refresh() -> void:
	if slider == null:
		return
	var model: InteractiveAnswer = answer as InteractiveAnswer
	if model.state.has("value") and not is_equal_approx(slider.value, float(model.state["value"])):
		slider.set_value_no_signal(float(model.state["value"]))
	slider.editable = not locked
	value_label.text = str(model.state.get("value", "")) if model.state.has("value") else ""
