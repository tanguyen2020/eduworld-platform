class_name ReceiptStateModel
extends RefCounted
## Delivered / read cursor of one other member of a small channel (F119 data-model §1; `chat.receipt.updated`,
## CHAT_SYSTEM §8.1). The status of a message `seq = s` for that member: read if `read_seq ≥ s`, delivered if
## `delivered_seq ≥ s`, otherwise sent. Cursors never go back. Nickname only (AGENTS R16).

var member_id: String = ""
var nickname: String = ""
var delivered_seq: int = 0
var read_seq: int = 0


## AsyncAPI `chat.receipt.updated` payload; null without a member.
static func from_payload(payload: Dictionary) -> ReceiptStateModel:
	var member: Dictionary = DataRead.dict_of(payload, "member")
	var id: String = DataRead.str_of(member, "user_id")
	if not Uuid.is_valid(id):
		return null
	var r: ReceiptStateModel = ReceiptStateModel.new()
	r.member_id = id
	r.nickname = DataRead.str_of(member, "nickname").left(ChatMessageModel.NICKNAME_MAX)
	r.delivered_seq = maxi(0, WsEnvelope.as_int(payload.get("delivered_seq"), 0))
	r.read_seq = maxi(0, WsEnvelope.as_int(payload.get("read_seq"), 0))
	r.delivered_seq = maxi(r.delivered_seq, r.read_seq)
	return r


## Moves the cursors forward with `other` (same member).
func advance(other: ReceiptStateModel) -> void:
	if other == null:
		return
	if not other.nickname.is_empty():
		nickname = other.nickname
	read_seq = maxi(read_seq, other.read_seq)
	delivered_seq = maxi(maxi(delivered_seq, other.delivered_seq), read_seq)
