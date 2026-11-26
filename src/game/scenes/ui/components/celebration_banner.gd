class_name CelebrationBanner
extends HBoxContainer
## Celebration at the top of the result screen (F096-AC-14, STUDENT_GAME_UX_LEARNING §10.3):
## Mây beside a speech bubble "Hoàn thành! — Em làm xong rồi, giỏi quá!" and, for `g1_3`, one star icon per right
## answer. A single pop-in of the stars and a small hop of Mây, ≤ ANIMATION_MAX_SEC, none when "Giảm chuyển động" is on.
## Always positive: no countdown, no streak, no "come back tomorrow", no comparison, no loss message (AGENTS R19,
## NFR-SAFE-003). Shows what the server granted only; decides nothing (AGENTS R1, R5).

## STUDENT_GAME_UX_LEARNING §10.3: the celebration of the result screen lasts at most 1.5 s.
const ANIMATION_MAX_SEC: float = 1.5
## Stars drawn at most (a long attempt still fits one row).
const MAX_STARS: int = 10
## One star pops in this long; the stagger spreads the rest inside ANIMATION_MAX_SEC.
const STAR_POP_SEC: float = 0.3
const HOP_SEC: float = 0.3
const STAR_ICON: Texture2D = preload("res://resources/ui/icons/star.svg")
## Star size in icon units (bigger than a button icon: they are the reward).
const STAR_SCALE: float = 1.25

var _tween: Tween = null

@onready var _stars: HBoxContainer = %Stars
@onready var _mascot: Control = %CelebrationMascot
@onready var _party: TextureRect = %PartyIcon


func _ready() -> void:
	_party.custom_minimum_size = Vector2.ONE * UiTokens.COMPONENT_ICON_SIZE


## Shows the banner with `stars` star icons (0 = none) and plays the effect once unless `reduce_motion`.
func celebrate(stars: int, reduce_motion: bool) -> void:
	stop()
	for child: Node in _stars.get_children():
		_stars.remove_child(child)
		child.queue_free()
	var count: int = clampi(stars, 0, MAX_STARS)
	var side: float = float(UiTokens.COMPONENT_ICON_SIZE) * STAR_SCALE
	for i: int in count:
		var star: TextureRect = TextureRect.new()
		star.name = "Star%d" % i
		star.texture = STAR_ICON
		star.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
		star.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
		star.custom_minimum_size = Vector2(side, side)
		star.pivot_offset = Vector2(side, side) / 2.0
		_stars.add_child(star)
	_stars.visible = count > 0
	_stars.tooltip_text = tr("ui.result.stars_tooltip") if count > 0 else ""
	if reduce_motion or not is_inside_tree():
		return
	_tween = create_tween().set_parallel(true)
	var gap: float = (ANIMATION_MAX_SEC - STAR_POP_SEC - HOP_SEC) / float(maxi(count, 1))
	for i: int in count:
		var star: Control = _stars.get_child(i) as Control
		star.scale = Vector2.ZERO
		_tween.tween_property(star, "scale", Vector2.ONE, STAR_POP_SEC).set_delay(gap * float(i)) \
				.set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	_mascot.pivot_offset = _mascot.size / 2.0
	_tween.tween_property(_mascot, "scale", Vector2.ONE * 1.1, HOP_SEC / 2.0).set_trans(Tween.TRANS_SINE)
	_tween.chain().tween_property(_mascot, "scale", Vector2.ONE, HOP_SEC / 2.0).set_trans(Tween.TRANS_SINE)


## Seconds the effect lasts for `stars` (0 when reduced motion skips it) — tested against ANIMATION_MAX_SEC.
static func effect_sec(stars: int) -> float:
	var count: int = clampi(stars, 0, MAX_STARS)
	var gap: float = (ANIMATION_MAX_SEC - STAR_POP_SEC - HOP_SEC) / float(maxi(count, 1))
	return maxf(gap * float(maxi(count - 1, 0)) + STAR_POP_SEC, HOP_SEC)


func animating() -> bool:
	return _tween != null and _tween.is_valid() and _tween.is_running()


func star_count() -> int:
	return _stars.get_child_count()


func stop() -> void:
	if _tween != null:
		_tween.kill()
		_tween = null
	for child: Node in _stars.get_children():
		(child as Control).scale = Vector2.ONE
	if _mascot != null:
		_mascot.scale = Vector2.ONE
