class_name LiveEventModel
extends RefCounted
## A live-ops event of the Event center (OpenAPI `GameEventSummary` of `listGameEvents`, `GameEventDetail` of
## `getGameEvent`; AsyncAPI `event.status.changed`, `event.progress.updated`; F111 data-model §1–§3, EVENT_SEASON_SYSTEM
## §3–§6, §9–§11). The server already filtered the events of the student's scope and grade (`audience.grade_levels`) —
## nothing is filtered here. A tenant name may be Vietnamese only: `display_name` falls back to `vi`. The time left is in
## whole days ("Còn 5 ngày", "Hôm nay là ngày cuối", "Sắp diễn ra" when scheduled) — never hours, minutes or seconds.
## Points, the daily points left and the milestones are the server's values; an update REPLACES them (ADR-0016).

const SCHEDULED: String = "scheduled"
const LIVE: String = "live"
const SUSPENDED: String = "suspended"
const ENDED: String = "ended"
const CANCELLED: String = "cancelled"
const ARCHIVED: String = "archived"
const STATUSES: PackedStringArray = [SCHEDULED, LIVE, SUSPENDED, ENDED, CANCELLED, ARCHIVED]
## event_kind → icon of the card (no banner download: the thumbnail is the icon on the themed card, NFR-COMP-001).
const KIND_ICONS: Dictionary = {
	"world_festival": "star", "weekend": "emoji_sun", "school": "assignments", "community_goal": "group",
	"learning_campaign": "emoji_book", "special_boss": "area_challenge_mountain", "limited_quest": "journal",
}

var event_id: String = ""
var event_run_id: String = ""
var event_kind: String = ""
var owner_scope: String = ""
var name: Dictionary = {}
var short_tagline: Dictionary = {}
var description: Dictionary = {}
var banner_url: String = ""
var starts_at: String = ""
var ends_at: String = ""
var status: String = LIVE
var joined: bool = false
var event_points: int = 0
## -1 = the server did not say (summary only).
var daily_points_remaining: int = -1
var quest_ids: PackedStringArray = []
var milestones: Array[EventMilestoneModel] = []
var community_goal: CommunityGoalModel = null
var claim_deadline_at: String = ""
## True once `getGameEvent` filled the detail fields.
var has_detail: bool = false


static func from_dict(data: Variant) -> LiveEventModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "event_id")
	var n: Dictionary = DataRead.text_of(d, "name")
	if not Uuid.is_valid(id) or L10n.pick(n, "vi").is_empty():
		return null
	var e: LiveEventModel = LiveEventModel.new()
	e.event_id = id
	e.apply(d)
	return e


static func list_from(data: Variant) -> Array[LiveEventModel]:
	var out: Array[LiveEventModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var e: LiveEventModel = from_dict(raw)
			if e != null and e.status != CANCELLED and e.status != ARCHIVED:
				out.append(e)
	return out


## Merges a summary or a detail body (the detail fields only when present).
func apply(d: Dictionary) -> void:
	event_run_id = DataRead.str_of(d, "event_run_id", event_run_id).left(36)
	event_kind = DataRead.str_of(d, "event_kind", event_kind).left(32)
	owner_scope = DataRead.str_of(d, "owner_scope", owner_scope).left(16)
	var n: Dictionary = DataRead.text_of(d, "name")
	if not n.is_empty():
		name = n
	var tag: Dictionary = DataRead.text_of(d, "short_tagline")
	if not tag.is_empty():
		short_tagline = tag
	banner_url = DataRead.str_of(d, "banner_url", banner_url).left(2048)
	starts_at = DataRead.str_of(d, "starts_at", starts_at).left(32)
	ends_at = DataRead.str_of(d, "ends_at", ends_at).left(32)
	var st: String = DataRead.str_of(d, "status", status)
	status = st if st in STATUSES else status
	joined = DataRead.bool_of(d, "joined", joined)
	event_points = maxi(0, DataRead.int_of(d, "event_points", event_points))
	if d.has("description"):
		description = DataRead.text_of(d, "description")
	if d.has("quest_ids"):
		quest_ids = DataRead.strings_of(d, "quest_ids", 50)
	if d.has("milestones"):
		milestones = EventMilestoneModel.list_from(d.get("milestones"))
		has_detail = true
	if d.get("community_goal") is Dictionary:
		community_goal = CommunityGoalModel.from_dict(d["community_goal"])
	if d.has("daily_points_remaining"):
		daily_points_remaining = maxi(0, DataRead.int_of(d, "daily_points_remaining"))
	if d.has("claim_deadline_at"):
		claim_deadline_at = DataRead.str_of(d, "claim_deadline_at").left(32)
	if d.has("description") or d.has("quest_ids"):
		has_detail = true


## `event.progress.updated`: the server totals replace the shown values (a repeated message changes nothing);
## the milestones it names get the static "Mới đạt" badge.
func apply_progress(payload: Dictionary) -> void:
	event_points = maxi(0, DataRead.int_of(payload, "event_points", event_points))
	if payload.has("daily_points_remaining"):
		daily_points_remaining = maxi(0, DataRead.int_of(payload, "daily_points_remaining"))
	for id: String in DataRead.strings_of(payload, "milestones_reached", 20):
		var m: EventMilestoneModel = milestone(id)
		if m != null and not m.claimed:
			m.reached = true
			m.just_reached = true


func milestone(milestone_id: String) -> EventMilestoneModel:
	for m: EventMilestoneModel in milestones:
		if m.milestone_id == milestone_id:
			return m
	return null


func display_name(locale: String = "") -> String:
	return L10n.pick(name, locale)


func display_tagline(locale: String = "") -> String:
	return L10n.pick(short_tagline, locale)


func display_description(locale: String = "") -> String:
	return L10n.pick(description, locale)


func is_live() -> bool:
	return status == LIVE


func is_scheduled() -> bool:
	return status == SCHEDULED


func is_suspended() -> bool:
	return status == SUSPENDED


func is_ended() -> bool:
	return status == ENDED


## An ended event still lets the student claim a reached milestone until `claim_deadline_at` (F111-AC-12).
func claims_open(now_unix: int) -> bool:
	if status == LIVE:
		return true
	if status != ENDED or claim_deadline_at.is_empty():
		return false
	return now_unix < LiveOpsTime.unix_of(claim_deadline_at)


func days_left(now_unix: int) -> int:
	return LiveOpsTime.days_left(LiveOpsTime.unix_of(ends_at), now_unix)


## Key of the time-left words: upcoming, paused, ended, last day or "Còn {days} ngày".
func time_left_key(now_unix: int) -> String:
	match status:
		SCHEDULED:
			return "ui.events.upcoming"
		SUSPENDED:
			return "ui.events.suspended"
		ENDED:
			return "ui.events.ended"
	return "ui.events.last_day" if days_left(now_unix) <= 1 else "ui.events.time_left_days"


func time_left_text(now_unix: int) -> String:
	return TranslationServer.translate(time_left_key(now_unix)).format(
			{"days": LocaleFormat.number(days_left(now_unix))})


func icon_name() -> String:
	return String(KIND_ICONS.get(event_kind, "star"))
