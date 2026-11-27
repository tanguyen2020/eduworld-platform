class_name MatchProposalModel
extends RefCounted
## A match the server proposes (ready check; `matchmaking.proposal.created/updated`, F109-AC-03, MATCHMAKING §6).
## `deadline_at` is server time (ms): the dialog counts down with the server clock, never a local timer of its own.

var proposal_id: String = ""
var ticket_id: String = ""
var deadline_at: int = 0
var mode: String = ""
var participants: Array[PublicPlayerModel] = []
var accepted_count: int = 0
var total: int = 0
## The student pressed "Vào trận" (the server confirmed the accept).
var accepted: bool = false


static func from_dict(data: Variant) -> MatchProposalModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var p: MatchProposalModel = MatchProposalModel.new()
	p.proposal_id = DataRead.str_of(d, "proposal_id")
	p.deadline_at = DataRead.int_of(d, "deadline_at")
	if not Uuid.is_valid(p.proposal_id) or p.deadline_at <= 0:
		return null
	p.ticket_id = DataRead.str_of(d, "ticket_id")
	p.mode = DataRead.str_of(d, "mode")
	for raw: Variant in DataRead.arr_of(d, "participants"):
		var pl: PublicPlayerModel = PublicPlayerModel.from_dict(raw)
		if pl != null:
			p.participants.append(pl)
	p.total = p.participants.size()
	return p


## Seconds left at server time `now_ms` (never below 0).
func seconds_left(now_ms: int) -> int:
	return maxi(0, ceili(float(deadline_at - now_ms) / 1000.0))
