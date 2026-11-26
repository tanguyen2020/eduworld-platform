class_name RichContentView
extends VBoxContainer
## Draws a rich content of the render payload (stem, worked example text…): paragraphs, headings, lists, callouts,
## formulas (MathInline) and images (MediaLoader), in the attempt's content language (F063-AC-04, F063-AC-13).
## Vietnamese diacritics come from the Theme fonts (Be Vietnam Pro). Audio is played by the host's VoiceOverPlayer.

signal math_failed()

const MATH_SCENE: PackedScene = preload("res://scenes/learning/math/MathInline.tscn")

## Theme variation of paragraph labels (F097: `StemLabel` for a question stem); empty = default Label.
var label_variation: StringName = &""
var formulas: Array[MathInline] = []
var images: Array[TextureRect] = []


func show_content(rich: Dictionary, ctx: RendererContext, item: QuestionRenderModel = null) -> void:
	for child: Node in get_children():
		child.queue_free()
	formulas.clear()
	images.clear()
	for block: Dictionary in RichText.blocks_of(rich):
		_add_block(block, ctx, item)


func _add_block(block: Dictionary, ctx: RendererContext, item: QuestionRenderModel) -> void:
	match String(block.get("type", "")):
		"paragraph", "callout":
			_add_label(ctx.text(block.get("text")), label_variation)
		"heading":
			_add_label(ctx.text(block.get("text")), &"HeadingLabel")
		"list":
			if block.get("items") is Array:
				for entry: Variant in (block["items"] as Array):
					_add_label("• " + ctx.text(entry), label_variation)
		"table":
			if block.get("rows") is Array:
				for row: Variant in (block["rows"] as Array):
					var cells: PackedStringArray = []
					if row is Array:
						for cell: Variant in (row as Array):
							cells.append(ctx.text(cell))
					_add_label(" | ".join(cells), &"")
		"math":
			var formula: MathAssetRefModel = MathAssetRefModel.from_dict(block)
			if formula != null:
				var view: MathInline = MATH_SCENE.instantiate() as MathInline
				add_child(view)
				formulas.append(view)
				view.math_failed.connect(_on_math_failed)
				view.show_formula(formula, ctx.math)
		"image":
			var url: String = item.media_url(String(block.get("media_ref", ""))) if item != null else ""
			var rect: TextureRect = TextureRect.new()
			rect.expand_mode = TextureRect.EXPAND_FIT_WIDTH_PROPORTIONAL
			rect.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
			rect.tooltip_text = ctx.text(block.get("alt"))
			add_child(rect)
			images.append(rect)
			if not url.is_empty() and ctx.media != null:
				_load_image(rect, url, ctx.media)


func _add_label(text: String, variation: StringName) -> void:
	if text.is_empty():
		return
	var label: Label = Label.new()
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.text = text
	label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	if not variation.is_empty():
		label.theme_type_variation = variation
	add_child(label)


func _load_image(rect: TextureRect, url: String, media: MediaLoader) -> void:
	var tex: Texture2D = await media.texture(url)
	if is_instance_valid(rect) and tex != null:
		rect.texture = tex


func _on_math_failed() -> void:
	math_failed.emit()
