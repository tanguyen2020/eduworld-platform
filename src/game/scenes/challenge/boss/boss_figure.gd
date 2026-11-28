class_name BossFigure
extends Control
## The boss on the arena stage (F110-AC-05, AC-07, AC-11; STUDENT_GAME_UX_CORE S06 §9, §13): a friendly cartoon monster
## drawn with vector shapes in design-token colours (no 3D model, no SubViewport — the panel stays 2D and light on a
## mid-range phone). A right answer of the student shoots light rays at it (CPUParticles2D, ≤ 200 particles at the High
## preset, fewer at Medium, none at Low — QualityManager); a wrong one makes it "strike back" with a playful wobble
## (never scary, never the word "Sai"); a new phase changes its colours in ≤ 2 s. The energy is never drawn from client
## guesses: the arena passes the server value. Reduce motion keeps the colours, drops the movement.

## Particles per preset (AC-11: ≤ 200 at Medium is the budget; Low has none).
const PARTICLES: Dictionary = {"low": 0, "medium": 120, "high": 200}
const TRANSITION_SEC: float = 1.2
## Body colours of the phases (generic design tokens; the pinned presentation of the boss can replace the visual).
const PHASE_COLORS: Array[Color] = [UiTokens.COLOR_SUBJECT_ENGLISH, UiTokens.COLOR_SUBJECT_VIETNAMESE,
	UiTokens.COLOR_SUBJECT_MATH, UiTokens.COLOR_RARITY_EPIC]

var phase_index: int = 0
var energy_pct: float = 100.0
var defeated: bool = false
var rays: CPUParticles2D = null
## Effects played (tests).
var hits: int = 0
var strikes: int = 0
var _tween: Tween = null


func _init() -> void:
	name = "BossFigure"
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	pivot_offset = Vector2.ZERO
	rays = CPUParticles2D.new()
	rays.name = "Rays"
	rays.emitting = false
	rays.one_shot = true
	rays.explosiveness = 0.85
	rays.lifetime = 0.6
	rays.spread = 25.0
	rays.direction = Vector2(0.0, -1.0)
	rays.initial_velocity_min = 220.0
	rays.initial_velocity_max = 380.0
	rays.gravity = Vector2.ZERO
	rays.scale_amount_min = 2.0
	rays.scale_amount_max = 4.0
	rays.color = UiTokens.COLOR_ACCENT_REWARD
	add_child(rays)


func _ready() -> void:
	custom_minimum_size = Vector2.ONE * float(UiTokens.COMPONENT_MASCOT_SIZE)
	resized.connect(_on_resized)
	_on_resized()
	apply_quality(QualityManager.preset)
	QualityManager.quality_changed.connect(apply_quality)


func _exit_tree() -> void:
	if QualityManager.quality_changed.is_connected(apply_quality):
		QualityManager.quality_changed.disconnect(apply_quality)


func apply_quality(preset: String) -> void:
	rays.amount = maxi(1, int(PARTICLES.get(preset, PARTICLES["medium"])))
	rays.visible = int(PARTICLES.get(preset, 0)) > 0


func set_phase(index: int) -> void:
	phase_index = maxi(0, index)
	queue_redraw()


## Light rays from the bottom (the student) into the boss: a right answer of THIS student.
func hit() -> void:
	hits += 1
	if rays.visible:
		rays.restart()
		rays.emitting = true
	_pulse(1.08)


## A wrong answer: a playful wobble, the energy unchanged.
func strike_back() -> void:
	strikes += 1
	if Settings.reduce_motion:
		return
	_kill()
	_tween = create_tween()
	for angle: float in [0.08, -0.08, 0.05, 0.0]:
		_tween.tween_property(self, "rotation", angle, 0.09)


## Phase change (≤ 2 s): a squash then the new colours.
func transform_phase(next_index: int) -> float:
	if Settings.reduce_motion:
		set_phase(next_index)
		return 0.0
	_kill()
	_tween = create_tween()
	_tween.tween_property(self, "scale", Vector2(1.15, 0.85), TRANSITION_SEC * 0.4)
	_tween.tween_callback(set_phase.bind(next_index))
	_tween.tween_property(self, "scale", Vector2.ONE, TRANSITION_SEC * 0.6)
	return TRANSITION_SEC


func set_defeated(value: bool) -> void:
	defeated = value
	queue_redraw()


func _pulse(to: float) -> void:
	if Settings.reduce_motion:
		return
	_kill()
	_tween = create_tween()
	_tween.tween_property(self, "scale", Vector2.ONE * to, 0.12)
	_tween.tween_property(self, "scale", Vector2.ONE, 0.18)


func _kill() -> void:
	if _tween != null and _tween.is_valid():
		_tween.kill()
	scale = Vector2.ONE
	rotation = 0.0


func _on_resized() -> void:
	pivot_offset = size / 2.0
	rays.position = Vector2(size.x / 2.0, size.y)
	queue_redraw()


func _draw() -> void:
	var side: float = minf(size.x, size.y)
	if side <= 0.0:
		return
	var o: Vector2 = Vector2((size.x - side) / 2.0, (size.y - side) / 2.0)
	var body: Color = PHASE_COLORS[phase_index % PHASE_COLORS.size()]
	if defeated:
		body = UiTokens.COLOR_CONTENT_STATUS_DRAFT
	var ink: Color = UiTokens.COLOR_OUTLINE
	var line: float = maxf(2.0, side * 0.025)
	# Horns.
	for x: float in [0.3, 0.7]:
		var tip: Vector2 = o + Vector2(x + (x - 0.5) * 0.25, 0.08) * side
		var pts: PackedVector2Array = PackedVector2Array([o + Vector2(x - 0.07, 0.3) * side, o + Vector2(x + 0.07, 0.3) * side, tip])
		draw_colored_polygon(pts, UiTokens.COLOR_ACCENT_REWARD)
		draw_polyline(PackedVector2Array([pts[0], pts[2], pts[1]]), ink, line, true)
	# Body (a round blob) with its outline.
	var center: Vector2 = o + Vector2(0.5, 0.58) * side
	draw_circle(center, side * 0.36 + line, ink)
	draw_circle(center, side * 0.36, body)
	# Belly.
	draw_circle(o + Vector2(0.5, 0.7) * side, side * 0.18, UiTokens.COLOR_SURFACE_PANEL)
	# Eyes: big and friendly; closed arcs when defeated.
	for x: float in [0.4, 0.6]:
		var eye: Vector2 = o + Vector2(x, 0.5) * side
		if defeated:
			draw_arc(eye, side * 0.04, PI * 1.1, PI * 1.9, 10, ink, line, true)
			continue
		draw_circle(eye, side * 0.065, UiTokens.COLOR_TEXT_ON_ACCENT)
		draw_circle(eye + Vector2(0.01, 0.01) * side, side * 0.035, ink)
		draw_circle(eye + Vector2(-0.005, -0.01) * side, side * 0.012, UiTokens.COLOR_TEXT_ON_ACCENT)
	# Grin with two small teeth.
	draw_arc(o + Vector2(0.5, 0.6) * side, side * 0.1, PI * 0.15, PI * 0.85, 16, ink, line, true)
	for x: float in [0.46, 0.54]:
		var t: Vector2 = o + Vector2(x, 0.69) * side
		draw_colored_polygon(PackedVector2Array([t + Vector2(-0.018, 0.0) * side, t + Vector2(0.018, 0.0) * side,
			t + Vector2(0.0, 0.035) * side]), UiTokens.COLOR_TEXT_ON_ACCENT)
