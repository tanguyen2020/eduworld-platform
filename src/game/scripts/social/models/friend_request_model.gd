class_name FriendRequestModel
extends RefCounted
## A pending friend request (OpenAPI `FriendRequest` of `listFriendRequests`; F110 data-model §1). The other player is
## shown by nickname + avatar only (R16). A request this student SENT always reads "Đang chờ" until it disappears — a
## refused request is never told apart from an unanswered one (SOCIAL_SYSTEM §4, F110-AC-21, SC-005).

const DIRECTION_INCOMING: String = "incoming"
const DIRECTION_OUTGOING: String = "outgoing"

var request_id: String = ""
var direction: String = DIRECTION_INCOMING
## The other player (requester of an incoming request, addressee of an outgoing one).
var other: FriendModel = null
var created_at: String = ""
var expires_ms: int = 0


static func from_dict(data: Variant, dir: String) -> FriendRequestModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "request_id")
	var card: FriendModel = FriendModel.from_card(d.get("requester" if dir == DIRECTION_INCOMING else "addressee"))
	if not Uuid.is_valid(id) or card == null:
		return null
	# Only requests still waiting are listed; an outgoing one stays "pending" on screen whatever the server status.
	if dir == DIRECTION_INCOMING and DataRead.str_of(d, "status", "pending") != "pending":
		return null
	var r: FriendRequestModel = FriendRequestModel.new()
	r.request_id = id
	r.direction = dir
	r.other = card
	r.created_at = DataRead.str_of(d, "created_at")
	r.expires_ms = ExamCountdown.iso_to_ms(DataRead.str_of(d, "expires_at"))
	return r


static func list_from(data: Variant, dir: String) -> Array[FriendRequestModel]:
	var out: Array[FriendRequestModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var r: FriendRequestModel = from_dict(raw, dir)
			if r != null and out.size() < 100:
				out.append(r)
	return out


## The status words of the row: "Đang chờ" for every request this student sent.
func status_key() -> String:
	return "ui.friends.request_pending" if direction == DIRECTION_OUTGOING else "ui.friends.request_incoming"
