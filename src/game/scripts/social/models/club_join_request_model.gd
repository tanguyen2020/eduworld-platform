class_name ClubJoinRequestModel
extends RefCounted
## A request to join a club, seen by its owner / officers in the "Yêu cầu" tab (OpenAPI `ClubJoinRequest`;
## F111-AC-25): the nickname of the student who asked — nothing else about them (R16).

var request_id: String = ""
var club_id: String = ""
var requester_id: String = ""
var requester_nickname: String = ""
var status: String = "pending"
var created_at: String = ""


static func from_dict(data: Variant) -> ClubJoinRequestModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "request_id")
	if not Uuid.is_valid(id):
		return null
	var r: ClubJoinRequestModel = ClubJoinRequestModel.new()
	r.request_id = id
	r.club_id = DataRead.str_of(d, "club_id").left(36)
	r.requester_id = DataRead.str_of(d, "requester_id").left(36)
	r.requester_nickname = DataRead.str_of(d, "requester_nickname").left(FriendModel.NICKNAME_MAX_LENGTH)
	r.status = DataRead.str_of(d, "status", "pending").left(16)
	r.created_at = DataRead.str_of(d, "created_at").left(32)
	return r


## Only the requests still waiting.
static func list_from(data: Variant) -> Array[ClubJoinRequestModel]:
	var out: Array[ClubJoinRequestModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var r: ClubJoinRequestModel = from_dict(raw)
			if r != null and r.status == "pending":
				out.append(r)
	return out
