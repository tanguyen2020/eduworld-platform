class_name OnboardingHint
extends Control
## Mây's first-time guide on the HUD (F096-AC-10, STUDENT_GAME_UX_CORE S03 "Hướng dẫn lần đầu"): a card at the bottom
## with Mây, one short sentence and an icon per step, the step count, "Bỏ qua" and "Tiếp" / "Bắt đầu chơi"; the HUD
## control of the step gets a soft focus ring (pulsing unless "Giảm chuyển động" is on). Steps come from
## OnboardingGuide; when the guide closes the device remembers it (Settings.onboarding_hub_seen). UI only (AGENTS R1).

signal finished(skipped: bool)

## Seconds of one pulse of the ring (STUDENT_GAME_UX §6: gentle, no blinking).
const PULSE_SEC: float = 0.9
const RING_ALPHA_MIN: float = 0.35

var guide: OnboardingGuide = OnboardingGuide.new()
## Where the step targets (unique names of HUD controls) are looked up; the HUD sets itself.
var target_root: Node = null
var _ring_alpha: float = 1.0
var _pulse: Tween = null

@onready var _icon: TextureRect = %StepIcon
@onready var _text: Label = %StepText
@onready var _count: Label = %StepCount
@onready var _skip: Button = %Skip
@onready var _next: Button = %Next


func _ready() -> void:
	visible = false
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_icon.custom_minimum_size = Vector2(UiTokens.COMPONENT_ICON_SIZE, UiTokens.COMPONENT_ICON_SIZE)
	_skip.pressed.connect(skip)
	_next.pressed.connect(next)
	EventBus.locale_changed.connect(_on_locale_changed)
	resized.connect(queue_redraw)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## Opens the guide at its first step.
func start() -> void:
	guide.restart()
	show()
	_render()
	_next.grab_focus.call_deferred()


func next() -> void:
	if guide.next():
		_render()
	else:
		_close(false)


func skip() -> void:
	guide.skip()
	_close(true)


## Rect of the highlighted HUD control in this control's coordinates (empty when the step has none).
func target_rect() -> Rect2:
	var target: Control = _target()
	if target == null or not target.is_visible_in_tree():
		return Rect2()
	var rect: Rect2 = target.get_global_rect()
	rect.position -= get_global_rect().position
	return rect.grow(float(UiTokens.SIZE_SPACE_8))


func _target() -> Control:
	var name_: String = String(guide.current().get("target", ""))
	if name_.is_empty() or target_root == null:
		return null
	return target_root.find_child(name_, true, false) as Control


func _render() -> void:
	if _text == null:
		return
	var step: Dictionary = guide.current()
	_icon.texture = load(String(step["icon"])) as Texture2D
	_text.text = tr(String(step["key"]))
	_count.text = tr("ui.onboarding.progress").format({"n": guide.step + 1, "total": guide.count()})
	_next.text = tr("ui.onboarding.start") if guide.is_last() else tr("ui.onboarding.next")
	_skip.visible = not guide.is_last()
	_restart_pulse()
	queue_redraw()


func _restart_pulse() -> void:
	if _pulse != null:
		_pulse.kill()
		_pulse = null
	_ring_alpha = 1.0
	if Settings.reduce_motion or _target() == null or not is_inside_tree():
		return
	_pulse = create_tween().set_loops()
	_pulse.tween_method(_set_ring_alpha, 1.0, RING_ALPHA_MIN, PULSE_SEC)
	_pulse.tween_method(_set_ring_alpha, RING_ALPHA_MIN, 1.0, PULSE_SEC)


func _set_ring_alpha(alpha: float) -> void:
	_ring_alpha = alpha
	queue_redraw()


func pulsing() -> bool:
	return _pulse != null and _pulse.is_valid()


func _draw() -> void:
	var rect: Rect2 = target_rect()
	if rect.size == Vector2.ZERO:
		return
	var ring: StyleBoxFlat = StyleBoxFlat.new()
	ring.draw_center = false
	ring.border_color = Color(UiTokens.COMPONENT_FOCUS_RING_COLOR, _ring_alpha)
	ring.set_border_width_all(UiTokens.COMPONENT_FOCUS_RING_WIDTH)
	ring.set_corner_radius_all(UiTokens.SIZE_SPACE_16)
	draw_style_box(ring, rect)


func _close(skipped: bool) -> void:
	if _pulse != null:
		_pulse.kill()
		_pulse = null
	hide()
	Settings.set_onboarding_hub_seen(true)
	finished.emit(skipped)


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()
