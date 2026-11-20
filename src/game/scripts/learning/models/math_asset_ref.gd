class_name MathAssetRefModel
extends RefCounted
## A pre-rendered formula (ADR-0055, GODOT_ARCHITECTURE §6.3): either a `MathAssetRef` (`math_assets[]` of hints,
## feedback, explanations, activity render) or a math block of a render payload
## (`{"type": "math", "math_ref": "math/<sha256>.svg", "speech_text": …}`). The game only shows the SVG; it never
## receives LaTeX and never lets the student type a formula. `speech_text` is the read-aloud text and the fallback.

const SHA_PATTERN: String = "^[a-f0-9]{64}$"
const MATH_REF_PATTERN: String = "^math/([a-f0-9]{64})\\.svg$"
const DISPLAY_INLINE: String = "inline"
const DISPLAY_BLOCK: String = "block"
## `MathAssetRef.speech_text` maxLength (specs/openapi).
const SPEECH_MAX_LENGTH: int = 1000

var math_id: String = ""
## Absolute URL (`MathAssetRef.svg_url`); empty for a render-payload block (built from `math_ref` + CDN base).
var svg_url: String = ""
## Immutable object key `math/<sha256>.svg` (ADR-0044).
var math_ref: String = ""
var sha256: String = ""
var speech_text: String = ""
var display: String = DISPLAY_INLINE


## Parses a `MathAssetRef` or a render-payload math block; null when neither a URL nor a key is present.
static func from_dict(data: Dictionary) -> MathAssetRefModel:
	var m: MathAssetRefModel = MathAssetRefModel.new()
	m.math_id = _text(data, "math_id")
	m.svg_url = _text(data, "svg_url")
	m.math_ref = _text(data, "math_ref")
	m.speech_text = _text(data, "speech_text").left(SPEECH_MAX_LENGTH)
	var sha: String = _text(data, "sha256").to_lower()
	if RegEx.create_from_string(SHA_PATTERN).search(sha) != null:
		m.sha256 = sha
	var ref_match: RegExMatch = RegEx.create_from_string(MATH_REF_PATTERN).search(m.math_ref)
	if m.sha256.is_empty() and ref_match != null:
		m.sha256 = ref_match.get_string(1)
	if m.math_ref.is_empty() and not m.sha256.is_empty():
		m.math_ref = "math/%s.svg" % m.sha256
	var raw_display: String = _text(data, "display")
	if data.get("type") == "math" and raw_display.is_empty():
		raw_display = DISPLAY_BLOCK
	m.display = DISPLAY_BLOCK if raw_display == DISPLAY_BLOCK else DISPLAY_INLINE
	if m.svg_url.is_empty() and m.math_ref.is_empty():
		return null
	return m


static func list_from(items: Variant) -> Array[MathAssetRefModel]:
	var out: Array[MathAssetRefModel] = []
	if not items is Array:
		return out
	for item: Variant in (items as Array):
		var m: MathAssetRefModel = from_dict(item as Dictionary) if item is Dictionary else null
		if m != null:
			out.append(m)
	return out


func is_block() -> bool:
	return display == DISPLAY_BLOCK


## Download URL: `svg_url`, else `<cdn_base_url>/<math_ref>`.
func url(cdn_base_url: String) -> String:
	if not svg_url.is_empty():
		return svg_url
	return cdn_base_url.trim_suffix("/") + "/" + math_ref


## Text shown instead of the SVG (loading, failure, too large) and read aloud.
func fallback_text() -> String:
	return speech_text if not speech_text.is_empty() else math_id


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""
