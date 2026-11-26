class_name NotificationFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): sample in-game notifications (F123) shaped like the contract fixtures of tools/gen_fixtures.mjs
## (`openapi/listNotifications.200.json`, `asyncapi/notification.item.created.*`) for scene tests, the grade-band matrix
## and the review captures. Only nicknames and content names (AGENTS R16) — invented samples, no real student.

const ROUTE_LIST: String = "GET /notifications"
const ROUTE_READ: String = "POST /notifications/*:read"
const ROUTE_READ_ALL: String = "POST /notifications:read-all"
const NICK: String = "SaoBang12"


static func item(kind: String, code: String, params: Dictionary, read: bool, action_kind: String,
		created_at: String) -> Dictionary:
	return {"notification_id": Uuid.v7(), "kind": kind, "message_code": code, "params": params, "read": read,
		"ref_type": kind, "ref_id": Uuid.v7(), "action": {"kind": action_kind, "id": Uuid.v7()},
		"created_at": created_at}


## Five notifications, newest first; the first two unread.
static func sample_items() -> Array:
	return [
		item("friend_request", "friend.request_received", {"nickname": NICK}, false, "open_friend_requests",
				"2026-11-21T08:05:00Z"),
		item("assignment_new", "assignment.new", {"title": "Phép nhân bảng 3"}, false, "open_assignment",
				"2026-11-21T07:30:00Z"),
		item("assignment_results_released", "assignment.results_released", {"title": "Kiểm tra 15 phút: Đo độ dài"},
				true, "open_assignment_result", "2026-11-20T09:00:00Z"),
		item("level_reached", "progression.level_reached", {"level": "4"}, true, "open_profile", "2026-11-19T15:20:00Z"),
		item("achievement_unlocked", "achievement.unlocked", {"tier": "bronze"}, true, "open_achievements",
				"2026-11-19T15:10:00Z"),
	]


static func page(items: Array, unread: int, next_cursor: String = "") -> Dictionary:
	return {"data": items, "page": {"next_cursor": next_cursor if not next_cursor.is_empty() else null,
		"has_more": not next_cursor.is_empty(), "limit": 20}, "unread_count": unread}


static func sample_page() -> Dictionary:
	return page(sample_items(), 2)


## Scripts the inbox routes of `http`: the sample page (or an empty one) and the read marks.
static func script(http: FakeHttp, empty: bool = false) -> void:
	http.reply(ROUTE_LIST, 200, page([], 0) if empty else sample_page())
	http.reply(ROUTE_READ, 200, {})
	http.reply(ROUTE_READ_ALL, 200, {"unread_count": 0})


## The payload of a realtime `notification.item.created` example (AsyncAPI fixture) with a fresh id.
static func created(name: String) -> Dictionary:
	var payload: Dictionary = RealtimeHarness.fixture("notification.item.created." + name)["payload"] as Dictionary
	payload["notification_id"] = Uuid.v7()
	return payload
