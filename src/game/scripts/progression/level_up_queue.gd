class_name LevelUpQueue
extends RefCounted
## Holds the level-up announcements until they can be shown (STUDENT_GAME_UX_CORE S07 §3, F069-AC-03): while a timed
## activity runs (`AppState.blocking_activity`: challenge `in_progress`, timed exam, boss phase) or while the S07
## modal is already open, messages wait; then every pending one is released as ONE merged announcement (AC-UX-0701,
## AC-UX-0704). Pure logic: the modal reports `mark_shown()` when it closes.

signal batch_released(batch: LevelUpModel)

var pending: Array[LevelUpModel] = []
## The S07 modal is open (a new message waits for it to close).
var showing: bool = false
## The batch released and not yet reported shown (a world created later presents it).
var current: LevelUpModel = null
## Store with `blocking_activity: StringName` and `blocking_activity_changed(activity)` (AppState, or a test fake).
var store: Object = null


func _init(app_state: Object = null) -> void:
	store = app_state
	if store != null and store.has_signal("blocking_activity_changed"):
		store.connect("blocking_activity_changed", _on_blocking_changed)


func push(level_up: LevelUpModel) -> void:
	if level_up == null:
		return
	# The modal is already showing an announcement nobody closed yet: it becomes "Lên {count} cấp!" at once.
	if showing and current != null:
		var both: Array[LevelUpModel] = [current, level_up]
		current = LevelUpModel.merge(both)
		batch_released.emit(current)
		return
	pending.append(level_up)
	flush_when_free()


func is_blocked() -> bool:
	if store == null:
		return false
	var activity: Variant = store.get("blocking_activity")
	return activity != null and not String(activity).is_empty()


## Releases the merged pending announcements when nothing holds them; returns true when a batch was released.
func flush_when_free() -> bool:
	if pending.is_empty() or showing or is_blocked():
		return false
	var merged: LevelUpModel = LevelUpModel.merge(pending)
	pending.clear()
	showing = true
	current = merged
	batch_released.emit(merged)
	return true


## The modal closed: the next pending batch (arrived meanwhile) may be shown.
func mark_shown() -> void:
	showing = false
	current = null
	flush_when_free()


func clear() -> void:
	pending.clear()
	showing = false
	current = null


func _on_blocking_changed(_activity: StringName) -> void:
	flush_when_free()
