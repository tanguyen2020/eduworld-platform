class_name InteractiveAnswer
extends AnswerModel
## `interactive` → `{state, action_log?}`: the widget state plus its action log, checked by the Go validator the
## question names (server only). Bounded by the contract (≤ 50 state keys, ≤ 500 actions, action ≤ 64 chars).

const MAX_STATE_KEYS: int = 50
const MAX_ACTIONS: int = 500
const ACTION_MAX_LENGTH: int = 64
const MAX_ARGS: int = 20
## Widgets this client can draw (`render_payload.widget`); others show "update required" in the item (F063-AC-26).
const SUPPORTED_WIDGETS: PackedStringArray = ["number_line"]

var widget: String = ""
var config: Dictionary = {}
var state: Dictionary = {}
var action_log: Array[Dictionary] = []


func _on_setup() -> void:
	widget = String(payload.get("widget", "")) if payload.get("widget") is String else ""
	config = payload.get("config", {}) as Dictionary if payload.get("config") is Dictionary else {}
	state = {}
	action_log = []


func is_supported() -> bool:
	return widget in SUPPORTED_WIDGETS


func set_state(key: String, value: Variant) -> void:
	if not state.has(key) and state.size() >= MAX_STATE_KEYS:
		return
	state[key] = value
	_notify()


func log_action(t_ms: int, action: String, args: Dictionary = {}) -> void:
	if action_log.size() >= MAX_ACTIONS:
		return
	var entry: Dictionary = {"t_ms": maxi(0, t_ms), "action": action.left(ACTION_MAX_LENGTH)}
	if not args.is_empty() and args.size() <= MAX_ARGS:
		entry["args"] = args
	action_log.append(entry)


func is_complete() -> bool:
	return is_supported() and not state.is_empty()


func to_response() -> Dictionary:
	if not is_complete():
		return {}
	var out: Dictionary = {"state": state.duplicate(true)}
	if not action_log.is_empty():
		out["action_log"] = action_log.duplicate(true)
	return out


func restore(response: Dictionary) -> void:
	var saved: Variant = response.get("state")
	if saved is Dictionary:
		for key: Variant in (saved as Dictionary).keys():
			set_state(String(key), (saved as Dictionary)[key])
