class_name RichText
extends RefCounted
## Helpers over a rich content of the render payload (`{blocks: [...]}`, specs/schemas/question.schema.json
## `richContent`; formulas are already `{type: math, math_ref, speech_text}` blocks, ADR-0055). Text is taken in the
## attempt's content language through RendererContext (F063-AC-13).

const TEXT_BLOCKS: PackedStringArray = ["paragraph", "heading", "callout"]


static func blocks_of(rich: Variant) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if rich is Dictionary and (rich as Dictionary).get("blocks") is Array:
		for b: Variant in ((rich as Dictionary)["blocks"] as Array):
			if b is Dictionary:
				out.append(b as Dictionary)
	return out


## Text of a rich content on one line: paragraphs, lists and formulas (their `speech_text`), for buttons, captions and
## read-aloud.
static func plain_text(rich: Variant, ctx: RendererContext) -> String:
	var parts: PackedStringArray = []
	for b: Dictionary in blocks_of(rich):
		var kind: String = String(b.get("type", ""))
		if kind in TEXT_BLOCKS:
			parts.append(_text(b.get("text"), ctx))
		elif kind == "list" and b.get("items") is Array:
			for entry: Variant in (b["items"] as Array):
				parts.append(_text(entry, ctx))
		elif kind == "math":
			var m: MathAssetRefModel = MathAssetRefModel.from_dict(b)
			if m != null:
				parts.append(m.fallback_text())
	return " ".join(parts).strip_edges()


## Formula blocks of a rich content.
static func math_of(rich: Variant) -> Array[MathAssetRefModel]:
	var out: Array[MathAssetRefModel] = []
	for b: Dictionary in blocks_of(rich):
		if String(b.get("type", "")) == "math":
			var m: MathAssetRefModel = MathAssetRefModel.from_dict(b)
			if m != null:
				out.append(m)
	return out


## Text of the content without its formulas (a button shows the formula as its icon).
static func text_without_math(rich: Variant, ctx: RendererContext) -> String:
	var parts: PackedStringArray = []
	for b: Dictionary in blocks_of(rich):
		if String(b.get("type", "")) in TEXT_BLOCKS:
			parts.append(_text(b.get("text"), ctx))
	return " ".join(parts).strip_edges()


static func _text(value: Variant, ctx: RendererContext) -> String:
	if ctx != null:
		return ctx.text(value)
	return value as String if value is String else ""
