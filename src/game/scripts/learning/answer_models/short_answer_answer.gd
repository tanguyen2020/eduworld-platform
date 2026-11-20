class_name ShortAnswerAnswer
extends AnswerModel
## `short_answer` → `{text}`, clipped to the render payload `max_length` (default 100) and the contract limit 500
## (F063-AC-12). Graded by the server against its accepted answers (never on the client).

const DEFAULT_MAX_LENGTH: int = 100
## `ShortAnswerResponse.text` maxLength.
const CONTRACT_MAX_LENGTH: int = 500

var text: String = ""
var limit: int = DEFAULT_MAX_LENGTH


func _on_setup() -> void:
	text = ""
	var raw: Variant = payload.get("max_length")
	limit = clampi(int(raw), 1, CONTRACT_MAX_LENGTH) if raw is int or raw is float else DEFAULT_MAX_LENGTH


func set_text(value: String) -> void:
	var clipped: String = AnswerModel.clip(value, limit)
	if clipped != text:
		text = clipped
		_notify()


func is_complete() -> bool:
	return not text.strip_edges().is_empty()


func to_response() -> Dictionary:
	return {"text": text.strip_edges()} if is_complete() else {}


func restore(response: Dictionary) -> void:
	if response.get("text") is String:
		set_text(response["text"] as String)
