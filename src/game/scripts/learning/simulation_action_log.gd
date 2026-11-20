class_name SimulationActionLog
extends RefCounted
## Action log of one simulation run (F066-AC-22, SIMULATION_ENGINE §3, §5): `{t_ms, action, args}` with `t_ms` relative
## to the start of the run, only actions of the template, within the limits of the server (≤ 500 actions, ≤ 20
## arguments, ≤ 64 KB). An action past a limit is dropped and `full` turns true (the run must be sent or restarted).
## The log is a record of intents: the result is computed by the server checker (ADR-0052, AGENTS R5).

const MAX_ACTIONS: int = 500
const MAX_ARGS: int = 20
const MAX_BYTES: int = 65536
const MAX_ACTION_LEN: int = 64

var allowed: PackedStringArray = []
var entries: Array[Dictionary] = []
var full: bool = false
var _start_msec: int = -1
var _bytes: int = 2
## Monotonic ms source (tests inject a fixed clock).
var now_msec: Callable = func() -> int: return Time.get_ticks_msec()


func _init(actions: PackedStringArray = []) -> void:
	allowed = actions


func start() -> void:
	entries.clear()
	full = false
	_bytes = 2
	_start_msec = int(now_msec.call())


func started() -> bool:
	return _start_msec >= 0


## Records an action; false when it is not an action of the template or a limit is reached.
func record(action: String, args: Dictionary = {}) -> bool:
	if full or action.length() > MAX_ACTION_LEN or (not allowed.is_empty() and not action in allowed) or args.size() > MAX_ARGS:
		return false
	if _start_msec < 0:
		start()
	var t: int = maxi(0, int(now_msec.call()) - _start_msec)
	if not entries.is_empty():
		t = maxi(t, int(entries.back()["t_ms"]))
	var entry: Dictionary = {"t_ms": t, "action": action, "args": args.duplicate(true)}
	var size: int = JSON.stringify(entry).to_utf8_buffer().size() + 1
	if entries.size() >= MAX_ACTIONS or _bytes + size > MAX_BYTES:
		full = true
		return false
	entries.append(entry)
	_bytes += size
	return true


func to_payload() -> Array:
	return entries.duplicate(true)


## The `SimulationRunInput` of this run with the pinned template, seed and parameters of the render.
func run_input(template_code: String, template_version: int, seed: int, parameters: Dictionary, client_outcome: Dictionary = {}) -> Dictionary:
	var out: Dictionary = {"template_code": template_code.left(64), "template_version": template_version, "seed": seed,
		"parameters": parameters.duplicate(true), "action_log": to_payload()}
	if not client_outcome.is_empty():
		out["client_outcome"] = client_outcome
	return out
