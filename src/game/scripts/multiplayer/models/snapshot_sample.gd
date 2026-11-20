class_name SnapshotSample
extends RefCounted
## One state of an entity of the hub (AsyncAPI `HubEntity` in `hub.snapshot.updated`, F092 data-model §1): server
## time of the sample (ms on the client's estimate of the server clock), position (m), yaw (rad), animation state.

var tick: int = 0
var server_ms: int = 0
var p: Vector3 = Vector3.ZERO
var yaw: float = 0.0
var anim: String = ""
## Fields present in the delta (the snapshot is a delta: missing fields keep the previous value).
var has_p: bool = false
var has_yaw: bool = false
var has_anim: bool = false


## Parses a `HubEntity`; null without an integer `eid`. `eid` is returned through `out_eid[0]`.
static func from_entity(data: Variant, tick_no: int, at_ms: int, out_eid: Array) -> SnapshotSample:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not WsEnvelope.is_int(d.get("eid")):
		return null
	var s: SnapshotSample = SnapshotSample.new()
	s.tick = tick_no
	s.server_ms = at_ms
	if d.has("p"):
		if not WsMessageSchemas.is_vec3(d["p"]):
			return null
		s.p = WsMessageSchemas.to_vec3(d["p"])
		s.has_p = true
	if d.has("r"):
		if not (d["r"] is float or d["r"] is int) or not is_finite(float(d["r"])):
			return null
		s.yaw = float(d["r"])
		s.has_yaw = true
	if d.get("a") is String and String(d["a"]).length() <= 255:
		s.anim = String(d["a"])
		s.has_anim = true
	out_eid.clear()
	out_eid.append(int(d["eid"]))
	return s


## Fills the fields this delta does not carry from `previous`.
func complete_from(previous: SnapshotSample) -> void:
	if previous == null:
		return
	if not has_p:
		p = previous.p
	if not has_yaw:
		yaw = previous.yaw
	if not has_anim:
		anim = previous.anim
