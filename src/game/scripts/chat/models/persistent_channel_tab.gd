class_name PersistentChannelTab
extends RefCounted
## F112 (data-model `PersistentChannelTab`): one tab of the HUD chat (ChatHub) — "Khu vực" (`area` of the instance the
## student stands in), "Cả trường" (`world`), a club of the student (its fixed `team` channel), "Bạn bè" (the friends
## and their `direct` channels) or the DM with one friend opened this session. Only what the tab shows: the server
## decides every permission (AGENTS R1, R7).

const KIND_AREA: String = "area"
const KIND_WORLD: String = "world"
const KIND_CLUB: String = "club"
const KIND_FRIENDS: String = "friends"
const KIND_DM: String = "dm"
const FRIENDS_ID: String = "friends"

## "area", "world", "club:<club_id>", "friends", "dm:<user_id>".
var tab_id: String = ""
var kind: String = ""
## The channel shown ("" for the "Bạn bè" list).
var channel_id: String = ""
var channel_type: String = ""
## A translation key ("Khu vực", "Cả trường", "Bạn bè") or "" when `title` (a club name, a nickname) is shown.
var title_key: String = ""
var title: String = ""
var club_id: String = ""
var user_id: String = ""
## The public card of the friend of a DM tab (nickname, level, avatar — AGENTS R16).
var peer: Dictionary = {}
var status: String = ChatChannelModel.STATUS_OPEN
## Unread messages (club / DM / the sum of the DMs on "Bạn bè"); always 0 for World / Area (CHAT_SYSTEM §7.1).
var unread: int = 0
## Unread mentions ("@" badge), on every kind.
var mentions: int = 0
var muted: bool = false


static func make(kind_name: String, id: String) -> PersistentChannelTab:
	var t: PersistentChannelTab = PersistentChannelTab.new()
	t.kind = kind_name
	t.tab_id = id
	return t


## World / Area never show an unread count (only the "@" badge).
func counts_unread() -> bool:
	return kind in [KIND_CLUB, KIND_DM, KIND_FRIENDS]


## The words of the tab: its key, or the club name / nickname (data).
func label() -> String:
	return TranslationServer.translate(title_key) if not title_key.is_empty() else title
