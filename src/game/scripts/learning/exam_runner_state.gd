class_name ExamRunnerState
extends RefCounted
## Navigation of the exam runner (data-model F064 `ExamRunnerState`; EXAM_ENGINE §3, F064-AC-06, F064-AC-07): the
## current question, the flags and which questions have an answer. `sequential_no_return` only moves forward (no
## "Câu trước", the grid only goes forward); `free` moves anywhere. Flags stay on this device (never sent). Pure logic.

const STATE_ANSWERED: String = "answered"
const STATE_UNANSWERED: String = "unanswered"

var positions: Array[int] = []
var index: int = 0
var forward_only: bool = false
var _flagged: Dictionary = {}
var _answered: Dictionary = {}


## `positions` of the attempt items in order; `answered` positions already saved. Resumes at the first question
## without an answer — after the last answered one when the exam only goes forward.
static func create(item_positions: Array[int], only_forward: bool, answered: Array[int]) -> ExamRunnerState:
	var s: ExamRunnerState = ExamRunnerState.new()
	s.positions = item_positions.duplicate()
	s.forward_only = only_forward
	for p: int in answered:
		s._answered[p] = true
	s.index = s._resume_index()
	return s


func count() -> int:
	return positions.size()


func current_position() -> int:
	return positions[index] if index >= 0 and index < positions.size() else 0


func can_go_prev() -> bool:
	return not forward_only and index > 0


func can_go_next() -> bool:
	return index < positions.size() - 1


func can_go_to(i: int) -> bool:
	if i < 0 or i >= positions.size():
		return false
	return not forward_only or i >= index


func go_to(i: int) -> bool:
	if not can_go_to(i):
		return false
	index = i
	return true


func go_next() -> bool:
	return go_to(index + 1) if can_go_next() else false


func go_prev() -> bool:
	return go_to(index - 1) if can_go_prev() else false


## Flags/unflags the current question; returns the new flag.
func toggle_flag() -> bool:
	var p: int = current_position()
	if _flagged.has(p):
		_flagged.erase(p)
		return false
	_flagged[p] = true
	return true


func is_flagged(position: int) -> bool:
	return _flagged.has(position)


func mark_answered(position: int) -> void:
	_answered[position] = true


func is_answered(position: int) -> bool:
	return _answered.has(position)


func unanswered_positions() -> Array[int]:
	var out: Array[int] = []
	for p: int in positions:
		if not _answered.has(p):
			out.append(p)
	return out


func flagged_positions() -> Array[int]:
	var out: Array[int] = []
	for p: int in positions:
		if _flagged.has(p):
			out.append(p)
	return out


func _resume_index() -> int:
	if positions.is_empty():
		return 0
	if forward_only:
		var last: int = -1
		for i: int in positions.size():
			if _answered.has(positions[i]):
				last = i
		return mini(last + 1, positions.size() - 1)
	for i: int in positions.size():
		if not _answered.has(positions[i]):
			return i
	return positions.size() - 1
