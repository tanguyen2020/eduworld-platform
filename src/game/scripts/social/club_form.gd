class_name ClubForm
extends RefCounted
## The checks of the "Tạo club" form before anything is sent (F111-AC-22; OpenAPI `ClubCreate`, ADR-0041): a student type
## (`interest_group`, `competition_team`), a name of 2–64 characters (spaces trimmed), a description of at most 500
## characters and an icon id of the client library. The server checks again and runs its moderation (F120, AGENTS R15).

## Form → "" when it may be sent, else the key of the friendly sentence.
static func validate(form: Dictionary) -> String:
	if not String(form.get("club_type", "")) in ClubModel.STUDENT_TYPES:
		return "ui.club.form.type"
	var n: String = String(form.get("name", "")).strip_edges()
	if n.length() < ClubModel.NAME_MIN or n.length() > ClubModel.NAME_MAX:
		return "ui.club.form.name_short"
	if String(form.get("description", "")).length() > ClubModel.DESCRIPTION_MAX:
		return "ui.club.form.description"
	return ""


## The `ClubCreate` body of a valid form (empty description and unknown icon left out).
static func body(form: Dictionary) -> Dictionary:
	var b: Dictionary = {"club_type": String(form["club_type"]), "name": String(form["name"]).strip_edges()}
	var d: String = String(form.get("description", "")).strip_edges()
	if not d.is_empty():
		b["description"] = d
	var icon: String = String(form.get("icon_id", ""))
	if ClubModel.ICONS.has(icon):
		b["icon_id"] = icon
	return b
