class_name JoinAvailability
extends RefCounted
## Whether "Tham gia bạn" may be pressed (F111-AC-02; STUDENT_GAME_UX_SOCIAL §6, MULTIPLAYER_GAMEPLAY §5). Pure: the
## friend's presence (`online` / `away` with an `area_id`, not in an exam) and the student's own state (offline, a
## blocking activity or an exam, a join already on its way). Returns &"" when the button is on, else the reason the grey
## button says in words (`ui.friends.join_unavailable.<reason>`). Only a hint — the server decides (F091).

const SELF_OFFLINE: StringName = &"self_offline"
const BUSY: StringName = &"busy"
const PENDING: StringName = &"pending"
const OFFLINE: StringName = &"offline"
const IN_MATCH: StringName = &"in_match"
const IN_ACTIVITY: StringName = &"in_activity"
const EXAM: StringName = &"exam"
const NO_AREA: StringName = &"no_area"
const REASONS: Array[StringName] = [SELF_OFFLINE, BUSY, PENDING, OFFLINE, IN_MATCH, IN_ACTIVITY, EXAM, NO_AREA]
const ACTIVITY_EXAM: String = "exam"


## `self_state`: {offline: bool, blocking: StringName (AppState.blocking_activity), exam: bool, pending: bool}.
static func evaluate(friend: FriendModel, self_state: Dictionary) -> StringName:
	if bool(self_state.get("offline", false)):
		return SELF_OFFLINE
	if not String(self_state.get("blocking", "")).is_empty() or bool(self_state.get("exam", false)):
		return BUSY
	if bool(self_state.get("pending", false)):
		return PENDING
	if friend == null or friend.presence == FriendModel.OFFLINE:
		return OFFLINE
	if friend.presence == FriendModel.IN_MATCH:
		return IN_MATCH
	if friend.activity == ACTIVITY_EXAM:
		return EXAM
	if friend.presence == FriendModel.IN_ACTIVITY:
		return IN_ACTIVITY
	if friend.area_id.is_empty():
		return NO_AREA
	return &""


static func reason_key(reason: StringName) -> String:
	if reason == PENDING:
		return "ui.friends.join_pending"
	return "ui.friends.join_unavailable.%s" % String(reason)
