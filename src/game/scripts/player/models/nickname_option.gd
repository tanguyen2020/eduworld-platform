class_name NicknameOption
extends RefCounted
## One `NicknameSuggestion` (specs/openapi): a server-generated nickname (≤ 24 characters, unique in the tenant at
## generation time) and the `nickname_option_id` the client sends back to choose it — never a free-text nickname
## (F070-AC-10, R16).

var option_id: String = ""
var nickname: String = ""


static func list_from(data: Variant) -> Array[NicknameOption]:
	var out: Array[NicknameOption] = []
	if not data is Array:
		return out
	for entry: Variant in data as Array:
		if not entry is Dictionary:
			continue
		var d: Dictionary = entry as Dictionary
		if d.get("nickname_option_id") is String and d.get("nickname") is String:
			var o: NicknameOption = NicknameOption.new()
			o.option_id = String(d["nickname_option_id"])
			o.nickname = String(d["nickname"])
			out.append(o)
	return out
