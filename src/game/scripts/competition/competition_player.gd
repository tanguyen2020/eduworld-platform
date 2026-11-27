class_name CompetitionPlayer
extends RefCounted
## The signed-in student as the arena services need it, read from the bootstrap in AppState (F061): the student id
## (to find "me" in server messages) and the grade level (only to ask `listSubjects` for the student's subjects —
## never to decide a mode, a bonus or a board, which the server does; F109 Clarifications 2026-11-16).


static func self_id(app_state: Node) -> String:
	var player: Dictionary = _player(app_state)
	for path: Array in [["student", "student_id"], ["game_profile", "student_id"], ["user", "user_id"]]:
		var id: String = DataRead.str_of(DataRead.dict_of(player, String(path[0])), String(path[1]))
		if Uuid.is_valid(id):
			return id
	return ""


static func grade_level(app_state: Node) -> int:
	return clampi(DataRead.int_of(DataRead.dict_of(_player(app_state), "student"), "grade_level"), 0, 9)


static func nickname(app_state: Node) -> String:
	return DataRead.str_of(DataRead.dict_of(_player(app_state), "game_profile"), "nickname")


static func _player(app_state: Node) -> Dictionary:
	if app_state == null or not app_state.get("player") is Dictionary:
		return {}
	return app_state.get("player") as Dictionary
