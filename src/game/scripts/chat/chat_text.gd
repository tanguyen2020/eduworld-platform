class_name ChatText
extends RefCounted
## Words of the chat panel built from server data and translation keys only (F119-AC-20, AC-22; ADR-0056): the text a
## bubble shows (also what "Sao chép" copies — the masked text or the placeholder, never an original, F119-AC-19), the
## typing sentence for 1–3 nicknames, the reply / quote / forward labels. Sentences are whole catalog entries with
## `{name}` parameters — never pieces joined by code (NFR-I18N-004). Free text from players is shown as written, never
## translated (CHAT_SYSTEM §8.6); quick messages and system notices follow the viewer's language.

const KEY_RECALLED: String = "ui.chat.recalled"
const KEY_REMOVED: String = "ui.chat.removed"
const KEY_HELD_OWN: String = "ui.chat.held_own"
const KEY_ORIGINAL_GONE: String = "ui.chat.original_gone"
const KEY_FORWARDED: String = "ui.chat.forwarded_from"
const KEY_EDITED: String = "ui.chat.edited"
const KEY_GIF: String = "ui.chat.gif_label"
const KEY_STICKER: String = "ui.chat.sticker_label"
const KEY_SYSTEM_GENERIC: String = "ui.chat.system.generic"
const KEY_TYPING_ONE: String = "ui.chat.typing_one"
const KEY_TYPING_TWO: String = "ui.chat.typing_two"
const KEY_TYPING_MANY: String = "ui.chat.typing_many"
const KEY_YOU: String = "ui.chat.you"
const KEY_REPLY_TO: String = "ui.chat.reply_to"
const KEY_QUOTE: String = "ui.chat.quote_of"
const KEY_EMOJI_UNKNOWN: String = "ui.chat.emoji.unknown"
## Emoji codes the client can draw (SVG icons `emoji_<code>` of design/icons/game) with their accessible names: the
## platform emoji set (backend seed `emoji_sets.json`, CHAT_SYSTEM §7.5) plus the codes of the quick-message catalog.
## A code outside the list shows a neutral smile and the generic name — the server decides what is allowed.
const EMOJI_KEYS: Dictionary = {
	"smile": "ui.chat.emoji.smile", "laugh": "ui.chat.emoji.laugh", "grin": "ui.chat.emoji.grin",
	"wink": "ui.chat.emoji.wink", "heart": "ui.chat.emoji.heart", "thumbs_up": "ui.chat.emoji.thumbs_up",
	"clap": "ui.chat.emoji.clap", "star": "ui.chat.emoji.star", "sparkles": "ui.chat.emoji.sparkles",
	"party": "ui.chat.emoji.party", "trophy": "ui.chat.emoji.trophy", "medal": "ui.chat.emoji.medal",
	"fire": "ui.chat.emoji.fire", "rocket": "ui.chat.emoji.rocket", "rainbow": "ui.chat.emoji.rainbow",
	"sun": "ui.chat.emoji.sun", "book": "ui.chat.emoji.book", "pencil": "ui.chat.emoji.pencil",
	"light_bulb": "ui.chat.emoji.light_bulb", "check": "ui.chat.emoji.check", "hundred": "ui.chat.emoji.hundred",
	"muscle": "ui.chat.emoji.muscle", "wave": "ui.chat.emoji.wave", "hug": "ui.chat.emoji.hug",
	"thinking": "ui.chat.emoji.thinking", "surprised": "ui.chat.emoji.surprised", "sad": "ui.chat.emoji.sad",
	"sleepy": "ui.chat.emoji.sleepy", "ok_hand": "ui.chat.emoji.ok_hand", "pray": "ui.chat.emoji.pray",
	"handshake": "ui.chat.emoji.handshake", "raised_hand": "ui.chat.emoji.raised_hand",
}
## The 30 codes of the emoji tab (the platform set, same order as the seed).
const EMOJI_SET: PackedStringArray = ["smile", "laugh", "grin", "wink", "heart", "thumbs_up", "clap", "star", "sparkles",
	"party", "trophy", "medal", "fire", "rocket", "rainbow", "sun", "book", "pencil", "light_bulb", "check", "hundred",
	"muscle", "wave", "hug", "thinking", "surprised", "sad", "sleepy", "ok_hand", "pray"]
## The 6 quick reactions of the action menu (a subset of the platform reaction set, CHAT_SYSTEM §4.3 "Hàng 6 emoji").
const QUICK_REACTIONS: PackedStringArray = ["thumbs_up", "heart", "laugh", "clap", "star", "surprised"]
const ICON_PREFIX: String = "emoji_"


## What the bubble shows and what "Sao chép" copies (F119-AC-19): the masked text, the quick sentence in the viewer's
## language, the emoji name, the sticker alt text, or the placeholder of a recalled / removed message.
static func visible_text(m: ChatMessageModel, catalog: QuickMessageCatalog = null, assets: ChatAssetCache = null) -> String:
	if m == null:
		return ""
	if m.recalled:
		return tr_key(KEY_RECALLED)
	if m.is_removed():
		return tr_key(KEY_REMOVED)
	match m.kind:
		ChatChannelModel.KIND_TEXT:
			return m.body
		ChatChannelModel.KIND_QUICK:
			return catalog.text_for(m.quick_message_id, m.params) if catalog != null else tr_key(QuickMessageCatalog.KEY_UNKNOWN)
		ChatChannelModel.KIND_EMOJI:
			return emoji_name(m.emoji)
		ChatChannelModel.KIND_STICKER:
			var alt: String = String(assets.sticker(m.sticker_id).get("alt_text", "")) if assets != null else ""
			return alt if not alt.is_empty() else tr_key(KEY_STICKER)
		ChatChannelModel.KIND_GIF:
			return tr_key(KEY_GIF)
		ChatMessageModel.KIND_SYSTEM:
			# The system sentence of the code when the catalog has it (prefix split so tools/extract_pot.gd sees no key here).
			var key: String = "ui.chat." + "system." + m.system_code
			return (tr_key(key) if L10n.has_key(key) else tr_key(KEY_SYSTEM_GENERIC)).format(m.params)
	return ""


static func emoji_name(code: String) -> String:
	return tr_key(String(EMOJI_KEYS.get(code, KEY_EMOJI_UNKNOWN)))


## Icon name of an emoji code (`emoji_<code>`), the neutral smile for an unknown code.
static func emoji_icon(code: String) -> String:
	return ICON_PREFIX + (code if EMOJI_KEYS.has(code) else "smile")


## "An đang soạn tin…", "An và Bình đang soạn tin…", "An, Bình và Chi đang soạn tin…" ("" for nobody).
static func typing_sentence(nicknames: PackedStringArray) -> String:
	match nicknames.size():
		0:
			return ""
		1:
			return tr_key(KEY_TYPING_ONE).format({"nickname": nicknames[0]})
		2:
			return tr_key(KEY_TYPING_TWO).format({"a": nicknames[0], "b": nicknames[1]})
	return tr_key(KEY_TYPING_MANY).format({"a": nicknames[0], "b": nicknames[1], "c": nicknames[2]})


## The reply strip of a bubble: the excerpt of the server, or "Tin nhắn gốc không còn".
static func reply_text(m: ChatMessageModel) -> String:
	return m.reply_excerpt if not m.reply_excerpt.is_empty() else tr_key(KEY_ORIGINAL_GONE)


static func quote_text(m: ChatMessageModel) -> String:
	return m.quote_excerpt if not m.quote_excerpt.is_empty() else tr_key(KEY_ORIGINAL_GONE)


static func forwarded_text(m: ChatMessageModel) -> String:
	return tr_key(KEY_FORWARDED).format({"nickname": m.forwarded_nickname})


## Sender line: the nickname, "{nickname} (em)" for the student (AGENTS R16: nickname only).
static func sender_text(m: ChatMessageModel, self_id: String) -> String:
	if m.is_mine(self_id):
		return tr_key(KEY_YOU).format({"nickname": m.sender_nickname})
	return m.sender_nickname


## Local time of a message ("10:22" / "10:22 AM").
static func time_text(epoch_ms: int, locale: String = "") -> String:
	if epoch_ms <= 0:
		return ""
	return LocaleFormat.time(LocaleFormat.from_unix(epoch_ms / 1000), locale)


static func tr_key(key: String) -> String:
	return TranslationServer.translate(key)
