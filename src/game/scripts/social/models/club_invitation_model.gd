class_name ClubInvitationModel
extends RefCounted
## A club invitation to the signed-in student (OpenAPI `ClubInvitation` of `listMyClubInvitations`, F120 delta;
## F111-AC-19, AC-21): the club name (moderated by the server) and the nickname of who invited — no real name (R16).

var invitation_id: String = ""
var club_id: String = ""
var club_name: String = ""
var inviter_nickname: String = ""
var invitee_id: String = ""
var status: String = "pending"
var expires_at: String = ""


static func from_dict(data: Variant) -> ClubInvitationModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "invitation_id")
	var club: String = DataRead.str_of(d, "club_id")
	if not Uuid.is_valid(id) or not Uuid.is_valid(club):
		return null
	var i: ClubInvitationModel = ClubInvitationModel.new()
	i.invitation_id = id
	i.club_id = club
	i.club_name = DataRead.str_of(d, "club_name").left(ClubModel.NAME_MAX)
	i.inviter_nickname = DataRead.str_of(d, "inviter_nickname").left(FriendModel.NICKNAME_MAX_LENGTH)
	var invitee: String = DataRead.str_of(d, "invitee_id")
	i.invitee_id = invitee if Uuid.is_valid(invitee) else ""
	i.status = DataRead.str_of(d, "status", "pending").left(16)
	i.expires_at = DataRead.str_of(d, "expires_at").left(32)
	return i


static func list_from(data: Variant) -> Array[ClubInvitationModel]:
	var out: Array[ClubInvitationModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var i: ClubInvitationModel = from_dict(raw)
			if i != null:
				out.append(i)
	return out
