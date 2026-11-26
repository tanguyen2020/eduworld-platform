class_name ProgressionStore
extends RefCounted
## RAM store of the student's progression (ADR-0064: never written to `user://`; dropped on logout or takeover,
## F069-AC-28). The HUD and S07/"Hành trình" read it and follow its signals (no `_process` polling, F069-AC-27).

signal progression_changed(progression: ProgressionModel)
## XP just granted by the server (`delta` of `progression.xp.updated`) — only for the flying number of the XP bar.
signal xp_gained(delta: int)

var progression: ProgressionModel = null


func set_status(value: ProgressionModel) -> void:
	if value == null:
		return
	# Without `daily_xp_remaining` the cap flag of the latest message stays.
	if value.daily_xp_remaining < 0 and progression != null:
		value.capped = progression.capped
	progression = value
	progression_changed.emit(progression)


## Applies a `progression.xp.updated` payload; true when the owner must reload `getProgression` (level span changed).
func apply_xp(payload: Dictionary) -> bool:
	if progression == null:
		return true
	var reload: bool = progression.apply_xp_update(payload)
	var delta: int = DataRead.int_of(payload, "delta")
	if delta > 0:
		xp_gained.emit(delta)
	progression_changed.emit(progression)
	return reload


func clear() -> void:
	progression = null
	progression_changed.emit(null)
