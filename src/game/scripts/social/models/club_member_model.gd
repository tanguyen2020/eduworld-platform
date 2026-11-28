class_name ClubMemberModel
extends RefCounted
## A member of a club page (OpenAPI `ClubMember`; F111-AC-20, AGENTS R16): nickname, avatar thumbnail, club role and the
## join date — never a score, a mastery, a contribution of another member, a real name or a school.

var user_id: String = ""
var nickname: String = ""
var avatar_thumbnail_url: String = ""
var role: String = ClubModel.ROLE_MEMBER
var joined_at: String = ""


static func from_dict(data: Variant) -> ClubMemberModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "user_id")
	var nick: String = DataRead.str_of(d, "nickname")
	if not Uuid.is_valid(id) or nick.is_empty() or nick.length() > FriendModel.NICKNAME_MAX_LENGTH:
		return null
	var m: ClubMemberModel = ClubMemberModel.new()
	m.user_id = id
	m.nickname = nick
	m.avatar_thumbnail_url = DataRead.str_of(d, "avatar_thumbnail_url").left(2048)
	var r: String = DataRead.str_of(d, "role", ClubModel.ROLE_MEMBER)
	m.role = r if r in ClubModel.ROLES else ClubModel.ROLE_MEMBER
	m.joined_at = DataRead.str_of(d, "joined_at").left(32)
	return m


static func list_from(data: Variant) -> Array[ClubMemberModel]:
	var out: Array[ClubMemberModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var m: ClubMemberModel = from_dict(raw)
			if m != null:
				out.append(m)
	return out


func role_key() -> String:
	return "ui.club.role.%s" % role


func role_icon() -> String:
	return "crown" if role == ClubModel.ROLE_OWNER else ("star" if role == ClubModel.ROLE_OFFICER else "group")
