class_name ClubFormOptions
extends RefCounted
## F112 (F112-AC-19, AC-22, AC-23; Clarifications 2026-11-28 F112-3, F112-4; OpenAPI `ClubUpdate`, `ClubGoalCreate`,
## `ClubEventCreate`, ADR-0041): the choices and bounds of the club owner's forms — data, not scene content (AGENTS R2):
## - weekly goal metrics: the three the server counts (`ClubGoalAcceptedMetric`); `listClubGoals.accepted_metrics`
##   narrows them further (never widens: an unknown metric is dropped — the safest reading of F112-4), each with a
##   picture and words; target 1…100 000 (contract), title ≤ 128 characters, no `subject_id`;
## - club event kinds: `practice_session`, `coop_quest`, `boss_raid`, `other` — never `internal_tournament` (F112-3);
##   title 1–128, description ≤ 1000; the start is a day + an hour in ICT, never before the server clock;
## - the club edit form: name 2–64, description ≤ 500, an icon of the F111 library (ClubModel.ICONS).
## Pure checks: each returns "" or the key of a friendly sentence; the server checks again (AGENTS R1, R5).

## [metric, label key, icon].
const METRICS: Array = [
	["activities_completed", "ui.club.manage.metric.activities_completed", "emoji_book"],
	["quests_completed", "ui.club.manage.metric.quests_completed", "journal"],
	["discoveries_found", "ui.club.manage.metric.discoveries_found", "map"],
]
## [event_kind, label key, icon].
const EVENT_KINDS: Array = [
	["practice_session", "ui.club.manage.kind.practice_session", "emoji_pencil"],
	["coop_quest", "ui.club.manage.kind.coop_quest", "group"],
	["boss_raid", "ui.club.manage.kind.boss_raid", "arena"],
	["other", "ui.club.manage.kind.other", "star"],
]
## `ClubGoalCreate.target` bounds (contract).
const TARGET_MIN: int = 1
const TARGET_MAX: int = 100000
const GOAL_TITLE_MAX: int = 128
const EVENT_TITLE_MIN: int = 1
const EVENT_TITLE_MAX: int = 128
const EVENT_DESCRIPTION_MAX: int = 1000
## −/+ steps of the target (small numbers first: a club of children counts tens, not thousands).
const TARGET_STEP: int = 5


## Metric ids offered: the three known ones, narrowed by the server's `accepted_metrics` when it sent any.
static func metrics(accepted: PackedStringArray = PackedStringArray()) -> PackedStringArray:
	var out: PackedStringArray = []
	for spec: Array in METRICS:
		if accepted.is_empty() or String(spec[0]) in accepted:
			out.append(String(spec[0]))
	return out


static func event_kinds() -> PackedStringArray:
	var out: PackedStringArray = []
	for spec: Array in EVENT_KINDS:
		out.append(String(spec[0]))
	return out


static func metric_spec(metric: String) -> Array:
	for spec: Array in METRICS:
		if String(spec[0]) == metric:
			return spec
	return []


static func kind_spec(kind: String) -> Array:
	for spec: Array in EVENT_KINDS:
		if String(spec[0]) == kind:
			return spec
	return []


static func clamp_target(value: int) -> int:
	return clampi(value, TARGET_MIN, TARGET_MAX)


## "Sửa thông tin": the fields that changed against `club` (`ClubUpdate` body, empty = nothing to save).
static func club_changes(club: ClubModel, form: Dictionary) -> Dictionary:
	var out: Dictionary = {}
	if club == null:
		return out
	var n: String = String(form.get("name", club.name)).strip_edges()
	if n != club.name:
		out["name"] = n
	var d: String = String(form.get("description", club.description)).strip_edges()
	if d != club.description:
		out["description"] = d
	var icon: String = String(form.get("icon_id", club.icon_id))
	if icon != club.icon_id and ClubModel.ICONS.has(icon):
		out["icon_id"] = icon
	return out


static func check_club(changes: Dictionary) -> String:
	if changes.has("name"):
		var n: String = String(changes["name"])
		if n.length() < ClubModel.NAME_MIN or n.length() > ClubModel.NAME_MAX:
			return "ui.club.form.name_short"
	if changes.has("description") and String(changes["description"]).length() > ClubModel.DESCRIPTION_MAX:
		return "ui.club.form.description"
	return ""


## Weekly goal form → `ClubGoalCreate` body, or {} when it may not be sent.
static func goal_body(form: Dictionary, accepted: PackedStringArray = PackedStringArray()) -> Dictionary:
	var metric: String = String(form.get("metric", ""))
	if not metric in metrics(accepted):
		return {}
	var target: int = int(form.get("target", 0))
	if target < TARGET_MIN or target > TARGET_MAX:
		return {}
	var title: String = String(form.get("title", "")).strip_edges()
	if title.length() > GOAL_TITLE_MAX:
		return {}
	var body: Dictionary = {"metric": metric, "target": target}
	if not title.is_empty():
		body["title"] = title
	return body


## Club event form → "" when it may be sent, else the key of the problem. `starts_unix` / `ends_unix` are UTC seconds,
## `now_unix` the server clock.
static func check_event(form: Dictionary, now_unix: int) -> String:
	var title: String = String(form.get("title", "")).strip_edges()
	if title.length() < EVENT_TITLE_MIN or title.length() > EVENT_TITLE_MAX:
		return "ui.club.manage.event.title_needed"
	if String(form.get("description", "")).length() > EVENT_DESCRIPTION_MAX:
		return "ui.club.manage.event.description_long"
	if not String(form.get("event_kind", "")) in event_kinds():
		return "ui.club.manage.event.kind_needed"
	var starts: int = int(form.get("starts_unix", 0))
	if starts <= now_unix:
		return "ui.club.manage.event.past"
	var ends: int = int(form.get("ends_unix", 0))
	if ends != 0 and ends <= starts:
		return "ui.club.manage.event.end_before_start"
	return ""


## `ClubEventCreate` body of a valid form (ISO times in UTC; no `tournament_template_id`, F112-3).
static func event_body(form: Dictionary) -> Dictionary:
	var body: Dictionary = {"title": String(form.get("title", "")).strip_edges(),
		"event_kind": String(form.get("event_kind", "other")),
		"starts_at": Time.get_datetime_string_from_unix_time(int(form["starts_unix"])) + "Z"}
	var d: String = String(form.get("description", "")).strip_edges()
	if not d.is_empty():
		body["description"] = d
	if int(form.get("ends_unix", 0)) > 0:
		body["ends_at"] = Time.get_datetime_string_from_unix_time(int(form["ends_unix"])) + "Z"
	return body
