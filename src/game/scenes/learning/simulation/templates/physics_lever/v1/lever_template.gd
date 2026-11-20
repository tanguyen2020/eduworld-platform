extends SimulationTemplateBase
## Template `physics.lever` v1 (F066 US3, AC-24): a lever with `slots_per_side` slots on each arm and up to four
## masses. The student picks a mass, then a slot (`place_mass`); picking a placed mass again takes it off
## (`remove_mass`). The beam tilts with the torque difference of LeverModel — the same arithmetic as the server checker —
## for display only. Lite mode (Low preset, Web) skips the tilt animation.

const MAX_TILT_DEG: float = 12.0

var model: LeverModel = LeverModel.new()
var selected: int = -1
var lite: bool = false
var _slot_buttons: Dictionary = {}
var _mass_buttons: Array[Button] = []


func setup_template(params: Dictionary, run_seed: int) -> String:
	super.setup_template(params, run_seed)
	var reason: String = model.configure(params)
	if not reason.is_empty():
		return reason
	_build()
	return ""


func actions() -> PackedStringArray:
	return PackedStringArray(["place_mass", "remove_mass", "release"])


func set_lite(value: bool) -> void:
	lite = value


func _build() -> void:
	var masses: Control = %Masses as Control
	for c: Node in masses.get_children():
		c.queue_free()
	_mass_buttons.clear()
	for i: int in model.masses.size():
		var b: Button = Button.new()
		b.toggle_mode = true
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.text = tr("ui.simulation.mass").format({"mass": String.num(model.masses[i])})
		b.pressed.connect(pick_mass.bind(i))
		masses.add_child(b)
		_mass_buttons.append(b)
	_slot_buttons.clear()
	for side: String in ["left", "right"]:
		var box: Control = (%LeftSlots if side == "left" else %RightSlots) as Control
		for c: Node in box.get_children():
			c.queue_free()
		for k: int in model.slots:
			var slot: int = model.slots - k if side == "left" else k + 1
			var b: Button = Button.new()
			b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
			b.text = str(slot)
			b.pressed.connect(pick_slot.bind(side, slot))
			box.add_child(b)
			_slot_buttons["%s:%d" % [side, slot]] = b
	_refresh()


## Selects a mass; selecting a placed mass that is already selected takes it off the lever.
func pick_mass(index: int) -> void:
	if index == selected and model.state.has(index):
		_act("remove_mass", {"mass_index": index})
		selected = -1
	else:
		selected = index
	_refresh()


func pick_slot(side: String, slot: int) -> void:
	if selected < 0:
		return
	_act("place_mass", {"mass_index": selected, "side": side, "slot": slot})
	selected = -1
	_refresh()


func _act(action: String, args: Dictionary) -> void:
	if model.apply(action, args).is_empty():
		action_performed.emit(action, args)


func reset_run() -> void:
	model.state.clear()
	model.measured.clear()
	selected = -1
	_refresh()


func measurements() -> Dictionary:
	return model.outcome(model.state)


func _refresh() -> void:
	for i: int in _mass_buttons.size():
		_mass_buttons[i].button_pressed = i == selected
	for key: Variant in _slot_buttons.keys():
		(_slot_buttons[key] as Button).text = String(key).split(":")[1]
	for i: Variant in model.state.keys():
		var p: Dictionary = model.state[i]
		var key: String = "%s:%d" % ["left" if int(p["side"]) < 0 else "right", int(p["slot"])]
		if _slot_buttons.has(key):
			var b: Button = _slot_buttons[key] as Button
			b.text += " · " + tr("ui.simulation.mass").format({"mass": String.num(model.masses[int(i)])})
	var m: Dictionary = measurements()
	var diff: float = float(m.get("torque_diff", 0.0))
	var beam: Control = %Beam as Control
	beam.pivot_offset = beam.size / 2.0
	beam.rotation_degrees = 0.0 if lite else clampf(-diff, -MAX_TILT_DEG, MAX_TILT_DEG)
