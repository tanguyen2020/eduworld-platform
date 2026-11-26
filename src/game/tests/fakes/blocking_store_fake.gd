class_name BlockingStoreFake
extends Node
## TEST-ONLY (AGENTS R17): store with `blocking_activity` like AppState (F069 LevelUpQueue / ChallengeStore tests).

signal blocking_activity_changed(activity: StringName)

var blocking_activity: StringName = &""
var _blockers: Array[StringName] = []


func set_blocking(value: StringName) -> void:
	blocking_activity = value
	blocking_activity_changed.emit(value)


func begin_blocking(activity: StringName) -> void:
	if not activity in _blockers:
		_blockers.append(activity)
	set_blocking(_blockers[0])


func end_blocking(activity: StringName) -> void:
	_blockers.erase(activity)
	set_blocking(_blockers[0] if not _blockers.is_empty() else &"")
