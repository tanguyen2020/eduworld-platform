class_name FriendCodeModel
extends RefCounted
## The friend code this student made (OpenAPI `FriendCode`; F110 data-model §1, ADR-0041): `^[A-Z0-9]{6,12}$` and the
## server `expires_at` (the countdown follows it; `friend_code_ttl` is the server's).

const PATTERN: String = "^[A-Z0-9]{6,12}$"
const MAX_LENGTH: int = 12

var code: String = ""
var expires_ms: int = 0


static func from_dict(data: Variant) -> FriendCodeModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var c: String = DataRead.str_of(d, "code")
	if not is_valid(c):
		return null
	var m: FriendCodeModel = FriendCodeModel.new()
	m.code = c
	m.expires_ms = ExamCountdown.iso_to_ms(DataRead.str_of(d, "expires_at"))
	return m


static func is_valid(text: String) -> bool:
	return RegEx.create_from_string(PATTERN).search(text) != null


## What the code field keeps of a typed text: upper case, only A-Z and 0-9, at most 12 characters (F110-AC-23).
static func normalize(text: String) -> String:
	var out: String = ""
	for ch: String in text.to_upper():
		if out.length() >= MAX_LENGTH:
			break
		var u: int = ch.unicode_at(0)
		if (u >= 65 and u <= 90) or (u >= 48 and u <= 57):
			out += ch
	return out


func left_ms(now_ms: int) -> int:
	return maxi(0, expires_ms - now_ms) if expires_ms > 0 else 0
