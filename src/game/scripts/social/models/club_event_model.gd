class_name ClubEventModel
extends RefCounted
## One entry of a club's calendar (OpenAPI `ClubEvent`; F111-AC-20): title, description, kind and when (ICT date).

var event_id: String = ""
var title: String = ""
var description: String = ""
var event_kind: String = "other"
var starts_at: String = ""
var ends_at: String = ""


static func from_dict(data: Variant) -> ClubEventModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "event_id")
	var t: String = DataRead.str_of(d, "title")
	if not Uuid.is_valid(id) or t.is_empty():
		return null
	var e: ClubEventModel = ClubEventModel.new()
	e.event_id = id
	e.title = t.left(128)
	e.description = DataRead.str_of(d, "description").left(1000)
	e.event_kind = DataRead.str_of(d, "event_kind", "other").left(32)
	e.starts_at = DataRead.str_of(d, "starts_at").left(32)
	e.ends_at = DataRead.str_of(d, "ends_at").left(32)
	return e


static func list_from(data: Variant) -> Array[ClubEventModel]:
	var out: Array[ClubEventModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var e: ClubEventModel = from_dict(raw)
			if e != null:
				out.append(e)
	return out
