class_name ChatFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): the chat contract fixtures of tools/gen_fixtures.mjs (OpenAPI response examples
## `listMyChannels.200`, `listChannelMessages.200`, `listQuickMessages.200`, `listStickerPacks.200`, `listGifs.200`,
## `createReport.201` and the AsyncAPI `chat.*` / `system.command.*` examples) with the routes of ChatService,
## ReportService and SocialService. Invented nicknames and neutral placeholder text only (AGENTS R15, R16) — never a
## real sensitive word; masked text is shown as "***". `as_self` makes MeoXanh42 (a1) the signed-in student.

const OPENAPI: String = "res://tests/fixtures/contract/openapi"
const PARTY_ID: String = "0193b0a0-0000-7000-8000-000000000101"
const PARTY_CHANNEL: String = "0193b0a0-0000-7000-8000-000000000102"
const MATCH_ID: String = "0193b0a0-0000-7000-8000-000000000500"
const MATCH_CHANNEL: String = "0193b0a0-0000-7000-8000-000000000501"
const TEAM_CHANNEL: String = "0193b0a0-0000-7000-8000-000000000502"
const SELF_SAMPLE: String = "0193b0a0-0000-7000-8000-0000000000a1"
const OTHER: String = "0193b0a0-0000-7000-8000-0000000000a2"
const THIRD: String = "0193b0a0-0000-7000-8000-0000000000a3"
const MSG_QUICK: String = "0193b0a0-0000-7000-8000-000000000611"
const MSG_TEXT: String = "0193b0a0-0000-7000-8000-000000000612"
const MSG_MINE: String = "0193b0a0-0000-7000-8000-000000000613"
const MSG_MASKED: String = "0193b0a0-0000-7000-8000-000000000614"
const MSG_EMOJI: String = "0193b0a0-0000-7000-8000-000000000615"
const STICKER: String = "0193b0a0-0000-7000-8000-000000000711"
const GIF: String = "0193b0a0-0000-7000-8000-000000000801"
const CDN: String = "https://cdn.example.invalid"
const ROUTE_CHANNELS: String = "GET /chat/channels"
const ROUTE_MESSAGES: String = "GET /chat/channels/*/messages"
const ROUTE_SEND: String = "POST /chat/channels/*/messages"
const ROUTE_PINS: String = "GET /chat/channels/*/pins"
const ROUTE_QUICK: String = "GET /chat/quick-messages"
const ROUTE_STICKERS: String = "GET /chat/stickers"
const ROUTE_GIFS: String = "GET /chat/gifs"
const ROUTE_REPORT: String = "POST /chat/reports"
const ROUTE_BLOCK: String = "PUT /social/blocks/*"
const ROUTE_MUTE: String = "PUT /social/mutes/*"
const ROUTE_CDN: String = "GET https://cdn.example.invalid/media/*"


static func openapi(name: String) -> Dictionary:
	return (JSON.parse_string(FileAccess.get_file_as_string("%s/%s.json" % [OPENAPI, name])) as Dictionary).duplicate(true)


## A JSON value with MeoXanh42's id replaced by `self_id` (the signed-in student wrote those messages).
static func as_self(value: Variant, self_id: String) -> Variant:
	if self_id.is_empty() or self_id == SELF_SAMPLE:
		return value
	return JSON.parse_string(JSON.stringify(value).replace(SELF_SAMPLE, self_id))


static func channels(self_id: String = "") -> Dictionary:
	return as_self(openapi("listMyChannels.200"), self_id) as Dictionary


## `listMyChannels` with the match channel closed `days` (+ a few minutes) after `now_ms` — retention from the server.
static func channels_closed_match(now_ms: int, days: int) -> Dictionary:
	var body: Dictionary = channels()
	for c: Variant in body["data"] as Array:
		if String((c as Dictionary)["channel_id"]) == MATCH_CHANNEL:
			(c as Dictionary)["purge_after"] = Time.get_datetime_string_from_unix_time(
					int(now_ms / 1000) + days * 86400 - 600) + "Z"
	return body


## `listMyChannels` with every channel closed or with `allowed_kinds` replaced (school preset quick + emote).
static func channels_with(kinds: Array, status: String = "open") -> Dictionary:
	var body: Dictionary = channels()
	for c: Variant in body["data"] as Array:
		(c as Dictionary)["allowed_kinds"] = kinds.duplicate()
		(c as Dictionary)["status"] = status
	return body


static func history(self_id: String = "") -> Dictionary:
	return as_self(openapi("listChannelMessages.200"), self_id) as Dictionary


static func quick() -> Dictionary:
	return openapi("listQuickMessages.200")


static func stickers() -> Dictionary:
	return openapi("listStickerPacks.200")


static func gifs() -> Dictionary:
	return openapi("listGifs.200")


static func report_created() -> Dictionary:
	return openapi("createReport.201")


## The payload of the AsyncAPI example `<type>.<name>` (e.g. "chat.message.created.text").
static func ws(name: String, self_id: String = "") -> Dictionary:
	var env: Dictionary = RealtimeHarness.fixture(name)
	return as_self(env["payload"], self_id) as Dictionary


## The whole envelope of an AsyncAPI example (for a RealtimeHarness push).
static func envelope(name: String, self_id: String = "") -> Dictionary:
	return as_self(RealtimeHarness.fixture(name), self_id) as Dictionary


## REST answer of `sendChannelMessage` for the pending message `payload` (server id + seq).
static func sent_message(payload: Dictionary, seq: int, self_id: String, moderation: String = "allowed") -> Dictionary:
	return {"message_id": Uuid.v4(), "channel_id": String(payload.get("channel_id", PARTY_CHANNEL)), "seq": seq,
		"sender": {"user_id": self_id, "nickname": "MeoXanh42"}, "kind": String(payload.get("kind", "text")),
		"body": String(payload.get("body", "")), "revision": 1, "moderation_state": moderation,
		"created_at": "2026-11-21T08:05:00Z"}


## A small PNG (an emoji picture of the game) served as a CDN sticker / GIF preview.
static func png() -> PackedByteArray:
	var img: Image = (load("res://resources/ui/icons/emoji_star.svg") as Texture2D).get_image()
	if img.is_compressed():
		img.decompress()
	return img.save_png_to_buffer()


## Scripts every read route of the chat panel on `http` plus report / block / mute and the CDN pictures.
static func script(http: FakeHttp, self_id: String = "") -> void:
	for route: String in [ROUTE_CHANNELS, ROUTE_MESSAGES, ROUTE_PINS, ROUTE_QUICK, ROUTE_STICKERS, ROUTE_GIFS, ROUTE_REPORT,
			ROUTE_BLOCK, ROUTE_MUTE, ROUTE_CDN]:
		http.clear(route)
	http.reply(ROUTE_CHANNELS, 200, channels(self_id))
	http.reply(ROUTE_MESSAGES, 200, history(self_id))
	http.reply(ROUTE_PINS, 200, {"data": []})
	http.reply(ROUTE_QUICK, 200, quick())
	http.reply(ROUTE_STICKERS, 200, stickers())
	http.reply(ROUTE_GIFS, 200, gifs())
	http.reply(ROUTE_REPORT, 201, report_created())
	http.reply(ROUTE_BLOCK, 204)
	http.reply(ROUTE_MUTE, 204)
	http.reply_bytes(ROUTE_CDN, 200, png())
