class_name NotificationModel
extends RefCounted
## One in-game notification of the student (F123, CHAT_SYSTEM §10, SOCIAL_SYSTEM §8): the `Notification` of
## `listNotifications` (REST, `created_at` ISO date-time, `read`) or the payload of the realtime message
## `notification.item.created` (`created_at` epoch ms, always unread). Read tolerantly (GAME_NETWORKING §9): a missing
## or wrong field gives the fallback; an item without a valid `notification_id` or `kind` is dropped. Texts are built
## on the client from `message_code` (`ui.notification.<code>`) + `params` in the current language, falling back to the
## server `title`/`body` (F123-AC-17, ADR-0056). `params` hold only a nickname, a content name or a level/tier
## (F123-AC-05) and are never logged (AGENTS R14).

## `NotificationKind` of specs/schemas/common/enums.schema.json (one enum for REST and WS, RB-513).
const KINDS: PackedStringArray = [
	"assignment_new", "assignment_due", "party_invitation", "friend_request", "quest_available", "reward_granted",
	"tournament_update", "moderation_notice", "system", "assignment_results_released", "level_reached",
	"achievement_unlocked",
]
## Contract bounds (ADR-0041): params ≤ 20 entries of ≤ 255 characters; kind/ref_type/action.kind ≤ 64; code ≤ 128.
const MAX_PARAMS: int = 20
const MAX_PARAM_LEN: int = 255
const MAX_CODE_LEN: int = 128
const MAX_KIND_LEN: int = 64
const KEY_PREFIX: String = "ui.notification."
## Shown when neither the message code nor the server texts can be displayed.
const KEY_GENERIC: String = "ui.notification.generic"
## Translation keys of the message codes of the chat module MessageCatalog (listed literally so extract_pot keeps them
## in the catalogs; a code outside this list still uses `ui.notification.<code>` when a translation exists).
const MESSAGE_KEYS: PackedStringArray = [
	"ui.notification.friend.request_received", "ui.notification.party.invitation_received",
	"ui.notification.assignment.new", "ui.notification.assignment.results_released",
	"ui.notification.progression.level_reached", "ui.notification.achievement.unlocked",
	"ui.notification.moderation.report_handled", "ui.notification.assignments_pending",
]

var notification_id: String = ""
var kind: String = ""
var message_code: String = ""
## String → String (contract `additionalProperties: {type: string}`); local notices may carry a number.
var params: Dictionary = {}
## `LocalizedName` / `LocalizedText` fallbacks for `L10n.pick()`.
var title: Dictionary = {}
var body: Dictionary = {}
var ref_type: String = ""
var ref_id: String = ""
var action_kind: String = ""
var action_id: String = ""
var read: bool = false
## Server time of creation, epoch ms (0 when unknown).
var created_at_ms: int = 0


## Null when the item cannot be used (no valid id or kind).
static func from_dict(d: Variant) -> NotificationModel:
	if not d is Dictionary:
		return null
	var src: Dictionary = d as Dictionary
	var m: NotificationModel = NotificationModel.new()
	m.notification_id = DataRead.str_of(src, "notification_id")
	m.kind = DataRead.str_of(src, "kind")
	if not Uuid.is_valid(m.notification_id) or m.kind.is_empty() or m.kind.length() > MAX_KIND_LEN:
		return null
	m.message_code = DataRead.str_of(src, "message_code").left(MAX_CODE_LEN)
	m.params = _params_of(DataRead.dict_of(src, "params"))
	m.title = DataRead.text_of(src, "title")
	m.body = DataRead.text_of(src, "body")
	m.ref_type = DataRead.str_of(src, "ref_type").left(MAX_KIND_LEN)
	var ref: String = DataRead.str_of(src, "ref_id")
	m.ref_id = ref if Uuid.is_valid(ref) else ""
	var action: Dictionary = DataRead.dict_of(src, "action")
	m.action_kind = DataRead.str_of(action, "kind").left(MAX_KIND_LEN)
	var target: String = DataRead.str_of(action, "id")
	m.action_id = target if Uuid.is_valid(target) else ""
	m.read = DataRead.bool_of(src, "read")
	m.created_at_ms = _epoch_ms(src.get("created_at"))
	return m


## Items of a `listNotifications` page (`data`), unusable entries dropped.
static func list_from(data: Variant) -> Array[NotificationModel]:
	var out: Array[NotificationModel] = []
	if not data is Array:
		return out
	for raw: Variant in data as Array:
		var m: NotificationModel = from_dict(raw)
		if m != null:
			out.append(m)
	return out


## A notice built on the client (no server id, never marked read on the server): e.g. the grouped "pending school
## work" toast of the first refresh of a session (F064-AC-21).
static func local(code: String, values: Dictionary, local_kind: String, open_kind: String) -> NotificationModel:
	var m: NotificationModel = NotificationModel.new()
	m.kind = local_kind
	m.message_code = code
	m.params = values.duplicate()
	m.action_kind = open_kind
	m.read = true
	return m


func is_local() -> bool:
	return notification_id.is_empty()


func is_known_kind() -> bool:
	return kind in KINDS


## The text to show in the current language (F123-AC-17): `ui.notification.<message_code>` formatted with `params`;
## without a translation the server `title` (+ `body`); else a generic sentence.
func display_text() -> String:
	if not message_code.is_empty():
		var key: String = KEY_PREFIX + message_code
		if L10n.has_key(key):
			return String(TranslationServer.translate(key)).format(params)
	var head: String = L10n.pick(title)
	var detail: String = L10n.pick(body)
	if not head.is_empty() and not detail.is_empty():
		return "%s\n%s" % [head, detail]
	if not head.is_empty() or not detail.is_empty():
		return head + detail
	return String(TranslationServer.translate(KEY_GENERIC))


static func _params_of(raw: Dictionary) -> Dictionary:
	var out: Dictionary = {}
	for key: Variant in raw.keys():
		if out.size() >= MAX_PARAMS:
			break
		var value: Variant = raw[key]
		if key is String and value is String and (key as String).length() <= MAX_KIND_LEN:
			out[key] = (value as String).left(MAX_PARAM_LEN)
	return out


## Epoch ms of WS `created_at` (number) or REST `created_at` (RFC 3339 date-time); 0 when unreadable.
static func _epoch_ms(value: Variant) -> int:
	if value is int or value is float:
		return maxi(0, int(value))
	if value is String and not (value as String).is_empty():
		var text: String = (value as String).strip_edges()
		var seconds: int = Time.get_unix_time_from_datetime_string(text.trim_suffix("Z").split(".")[0])
		return maxi(0, seconds * 1000) if seconds > 0 else 0
	return 0
