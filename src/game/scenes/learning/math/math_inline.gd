class_name MathInline
extends HBoxContainer
## One formula (ADR-0055, F063-AC-19/20): shows `speech_text` at once, then the SVG rasterized by MathSvgCache at
## the DPI × font scale and tinted with the Theme text colour (`RichTextLabel.default_color`). `display = block`
## centres it on its own line. When the SVG fails (download, sha256, `math_svg_max`) the speech text stays and
## `math_failed` is emitted (telemetry by the host). Display only: the game has no formula input.
## `append_to()` places a formula inline in a RichTextLabel instead.

signal math_failed()

var asset: MathAssetRefModel = null
var loaded: bool = false

@onready var _fallback: Label = %Fallback
@onready var _image: TextureRect = %Image


## Coroutine: shows `formula` (fallback text first).
func show_formula(formula: MathAssetRefModel, cache: MathSvgCache) -> void:
	asset = formula
	loaded = false
	alignment = BoxContainer.ALIGNMENT_CENTER if formula.is_block() else BoxContainer.ALIGNMENT_BEGIN
	_fallback.text = formula.fallback_text()
	_fallback.visible = true
	_image.visible = false
	tooltip_text = formula.speech_text
	var tex: Texture2D = null
	if cache != null:
		tex = await cache.texture(formula, MathSvgCache.scale_for(font_px(self), screen_scale()), text_color(self))
	if not is_instance_valid(self) or asset != formula:
		return
	if tex == null:
		math_failed.emit()
		return
	_image.texture = tex
	_image.visible = true
	_fallback.visible = false
	loaded = true


## Coroutine: appends `formula` inline into `label` (SVG image, or its speech text on failure). Returns false on failure.
static func append_to(label: RichTextLabel, formula: MathAssetRefModel, cache: MathSvgCache) -> bool:
	var tex: Texture2D = null
	if cache != null:
		tex = await cache.texture(formula, MathSvgCache.scale_for(font_px(label), screen_scale()), text_color(label))
	if not is_instance_valid(label):
		return false
	if tex == null:
		label.add_text(" %s " % formula.fallback_text())
		return false
	label.add_image(tex, 0, 0, Color.WHITE, INLINE_ALIGNMENT_CENTER, Rect2(), null, false, formula.speech_text)
	return true


## Theme text colour used to tint formulas.
static func text_color(control: Control) -> Color:
	return control.get_theme_color("default_color", "RichTextLabel")


static func font_px(control: Control) -> int:
	return control.get_theme_font_size("normal_font_size", "RichTextLabel")


## Screen DPI factor (content scale of the window).
static func screen_scale() -> float:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	return tree.root.content_scale_factor if tree != null else 1.0
