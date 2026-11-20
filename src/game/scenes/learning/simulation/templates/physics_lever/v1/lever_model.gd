class_name LeverModel
extends RefCounted
## The simplified model of `physics.lever` v1 (F066 research R4, AC-24): the same arithmetic as the server checker
## (modules/learning/internal/domain/activity/simulation/lever.go) — torque Σ m·g·d per side with
## d = arm_length_m · slot / slots_per_side, metrics rounded half away from zero to 4 decimals, `balanced` when both
## sides hold a mass and |diff| < 1e-6. specs/examples/simulation/physics.lever/v1/vectors.json binds both. Display only:
## the goal is reached when the server says so.

const G: float = 9.8
const PARAMS: PackedStringArray = ["arm_length_m", "slots_per_side", "mass_1_kg", "mass_2_kg", "mass_3_kg", "mass_4_kg"]

var arm: float = 1.0
var slots: int = 1
var masses: Array[float] = []
## mass index → {side: -1 left | 1 right, slot}; absent = not placed.
var state: Dictionary = {}
var measured: Dictionary = {}


## Reads the parameters; "" when valid, else the reason (the server refuses such a version).
func configure(params: Dictionary) -> String:
	for k: Variant in params.keys():
		if not String(k) in PARAMS:
			return "unknown parameter " + String(k)
	arm = float(params.get("arm_length_m", 0))
	slots = int(params.get("slots_per_side", 0))
	if arm < 0.2 or arm > 5 or slots < 1 or slots > 10:
		return "arm or slots out of range"
	masses.clear()
	for i: int in range(1, 5):
		var key: String = "mass_%d_kg" % i
		if not params.has(key):
			if i <= 2:
				return key + " is required"
			continue
		if masses.size() != i - 1:
			return key + " needs the previous mass"
		var m: float = float(params[key])
		if m < 0.1 or m > 50:
			return key + " out of range"
		masses.append(m)
	state.clear()
	measured.clear()
	return ""


static func _whole(v: Variant) -> int:
	if v is int:
		return v as int
	if v is float and float(v) == floorf(float(v)):
		return int(v)
	return -1


## Applies one action; "" when valid, else the reject code of the checker (`argument_out_of_range`, `unknown_action`).
func apply(action: String, args: Dictionary) -> String:
	match action:
		"place_mass":
			var idx: int = _whole(args.get("mass_index"))
			var slot: int = _whole(args.get("slot"))
			var side: String = String(args.get("side", ""))
			if args.size() != 3 or idx < 0 or idx >= masses.size() or slot < 1 or slot > slots or not side in ["left", "right"]:
				return "argument_out_of_range"
			state[idx] = {"side": -1 if side == "left" else 1, "slot": slot}
		"remove_mass":
			var idx: int = _whole(args.get("mass_index"))
			if args.size() != 1 or idx < 0 or idx >= masses.size():
				return "argument_out_of_range"
			state.erase(idx)
		"release":
			if not args.is_empty():
				return "argument_out_of_range"
			measured = state.duplicate(true)
		_:
			return "unknown_action"
	return ""


static func round4(x: float) -> float:
	var y: float = roundf(absf(x) * 10000.0) / 10000.0
	return -y if x < 0.0 else y


## Torque of `st` per side (live display, or the measured state at release).
func outcome(st: Dictionary = measured) -> Dictionary:
	var left: float = 0.0
	var right: float = 0.0
	var nl: int = 0
	var nr: int = 0
	for i: int in masses.size():
		if not st.has(i):
			continue
		var p: Dictionary = st[i]
		var d: float = arm * float(int(p["slot"])) / float(slots)
		if int(p["side"]) < 0:
			left += masses[i] * G * d
			nl += 1
		else:
			right += masses[i] * G * d
			nr += 1
	var diff: float = left - right
	var balanced: float = 1.0 if nl > 0 and nr > 0 and absf(diff) < 1e-6 else 0.0
	return {"torque_left": round4(left), "torque_right": round4(right), "torque_diff": round4(diff), "balanced": balanced,
		"masses_placed": float(nl + nr)}


## Replays a whole log like the server (time order, then the model): {valid, reject_code, outcome} (vector tests).
static func replay(params: Dictionary, entries: Array) -> Dictionary:
	var m: LeverModel = LeverModel.new()
	if not m.configure(params).is_empty():
		return {"valid": false, "reject_code": "pinned_value_mismatch"}
	var last: int = 0
	for e: Variant in entries:
		var entry: Dictionary = e as Dictionary
		var t: int = int(entry.get("t_ms", 0))
		if t < 0 or t < last:
			return {"valid": false, "reject_code": "time_not_increasing"}
		last = t
		var reason: String = m.apply(String(entry.get("action", "")), entry.get("args", {}) as Dictionary)
		if not reason.is_empty():
			return {"valid": false, "reject_code": reason}
	return {"valid": true, "outcome": m.outcome()}
