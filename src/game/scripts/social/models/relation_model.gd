class_name RelationModel
extends RefCounted
## A player this student blocked or muted (OpenAPI `PlayerRelation` of `listBlocks` / `listMutes`; F110 data-model §1,
## R16): nickname + avatar + when. There is no list of who blocked this student, anywhere (F110-AC-25).

const KIND_BLOCK: String = "block"
const KIND_MUTE: String = "mute"

var kind: String = KIND_BLOCK
var user_id: String = ""
var nickname: String = ""
var avatar_config: Dictionary = {}
var created_at: String = ""


static func from_dict(data: Variant, relation_kind: String) -> RelationModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "user_id")
	var nick: String = DataRead.str_of(d, "nickname")
	if not Uuid.is_valid(id) or nick.is_empty() or nick.length() > 32:
		return null
	var m: RelationModel = RelationModel.new()
	m.kind = relation_kind
	m.user_id = id
	m.nickname = nick
	m.avatar_config = DataRead.dict_of(d, "avatar_config").duplicate(true)
	m.created_at = DataRead.str_of(d, "created_at")
	return m


static func list_from(data: Variant, relation_kind: String) -> Array[RelationModel]:
	var out: Array[RelationModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var m: RelationModel = from_dict(raw, relation_kind)
			if m != null:
				out.append(m)
	return out


func avatar() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(avatar_config) if not avatar_config.is_empty() else null
