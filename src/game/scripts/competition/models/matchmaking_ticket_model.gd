class_name MatchmakingTicketModel
extends RefCounted
## The student's queue entry (`MatchmakingTicket` REST, `matchmaking.ticket.updated` WS; F109-AC-02, AC-04,
## MATCHMAKING §5). Wait time and the "search widened" flag are the server's; the client only shows them.

const ACTIVE: PackedStringArray = ["queued", "searching", "proposed"]
const ENDED: PackedStringArray = ["matched", "cancelled", "expired"]

var ticket_id: String = ""
var status: String = ""
var mode: String = ""
var subject_id: String = ""
var wait_ms: int = 0
var estimated_wait_ms: int = 0
var search_widened: bool = false
var bot_offer_available: bool = false
var requeued: bool = false
var cancel_reason: String = ""
var match_id: String = ""


## REST `MatchmakingTicket` (seconds) or WS payload (ms); null without a valid id / status.
static func from_dict(data: Variant) -> MatchmakingTicketModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var t: MatchmakingTicketModel = MatchmakingTicketModel.new()
	t.ticket_id = DataRead.str_of(d, "ticket_id")
	t.status = DataRead.str_of(d, "status")
	if not Uuid.is_valid(t.ticket_id) or not (t.status in ACTIVE or t.status in ENDED):
		return null
	t.mode = DataRead.str_of(d, "mode")
	t.subject_id = DataRead.str_of(d, "subject_id")
	t.wait_ms = DataRead.int_of(d, "wait_ms", DataRead.int_of(d, "wait_seconds") * 1000)
	t.estimated_wait_ms = DataRead.int_of(d, "estimated_wait_ms", DataRead.int_of(d, "estimated_wait_seconds") * 1000)
	t.search_widened = DataRead.bool_of(d, "search_widened")
	t.bot_offer_available = DataRead.bool_of(d, "bot_offer_available")
	t.requeued = DataRead.bool_of(d, "requeued")
	t.cancel_reason = DataRead.str_of(d, "cancel_reason")
	t.match_id = DataRead.str_of(d, "match_id")
	return t


func is_active() -> bool:
	return status in ACTIVE


## Ended by the queue timeout (`expired`, or `cancel_reason = timeout`): suggest another mode / the practice robot.
func timed_out() -> bool:
	return status == "expired" or cancel_reason == "timeout"
