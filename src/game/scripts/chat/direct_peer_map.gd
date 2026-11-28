class_name DirectPeerMap
extends RefCounted
## F112 (F112-AC-07, AC-10; data-model `DirectPeer`, Clarifications 2026-11-28 F112-5): which friend a `direct` channel is
## with — RAM only, cleared on logout / `session.session.replaced` (F112-AC-31). Sources, best first: `Channel.peer` of
## `listMyChannels` / `openDirectChannel` (the public card: user id, nickname, level, avatar — AGENTS R16), the channel the
## student opened with "Nhắn tin" (`openDirectChannel` for a friend of `listFriends`), the sender of the other student's
## message in that channel. A `direct` channel nobody of these names stays unknown: it only adds to the badge of the
## "Bạn bè" tab and never shows a guessed name.

## Card fields kept (the public card of PlayerCard — never a real name, a school or a grade).
const CARD_FIELDS: PackedStringArray = ["user_id", "nickname", "level", "avatar_config"]

var _by_channel: Dictionary = {}
var _by_user: Dictionary = {}


## Remembers that `channel_id` is the DM with the player of `card` (`user_id` required).
func remember(channel_id: String, card: Dictionary) -> void:
	var user_id: String = DataRead.str_of(card, "user_id")
	if not Uuid.is_valid(channel_id) or not Uuid.is_valid(user_id):
		return
	var kept: Dictionary = {}
	for k: String in CARD_FIELDS:
		if card.has(k):
			kept[k] = card[k]
	var old: Dictionary = _by_channel.get(channel_id, {}) as Dictionary
	if not old.is_empty() and String(old.get("user_id", "")) == user_id:
		# A later source without the nickname (a bare id) never wipes what is known.
		for k: String in old.keys():
			if not kept.has(k) or (kept[k] is String and String(kept[k]).is_empty()):
				kept[k] = old[k]
	_by_channel[channel_id] = kept
	_by_user[user_id] = channel_id


## `Channel.peer` of a `direct` channel (F112-5), when the server sent it.
func from_channel(c: ChatChannelModel) -> void:
	if c != null and c.channel_type == ChatChannelModel.TYPE_DIRECT and not c.peer.is_empty():
		remember(c.channel_id, c.peer)


## The other student's message in a `direct` channel names the peer of that channel.
func from_message(m: ChatMessageModel, channel_type: String, self_id: String) -> void:
	if m == null or channel_type != ChatChannelModel.TYPE_DIRECT or m.sender_id.is_empty() or m.sender_id == self_id:
		return
	if _by_channel.has(m.channel_id):
		return
	remember(m.channel_id, {"user_id": m.sender_id, "nickname": m.sender_nickname})


func knows(channel_id: String) -> bool:
	return _by_channel.has(channel_id)


func peer(channel_id: String) -> Dictionary:
	return (_by_channel.get(channel_id, {}) as Dictionary).duplicate()


func user_of(channel_id: String) -> String:
	return String((_by_channel.get(channel_id, {}) as Dictionary).get("user_id", ""))


func channel_of(user_id: String) -> String:
	return String(_by_user.get(user_id, ""))


func forget(channel_id: String) -> void:
	var user_id: String = user_of(channel_id)
	_by_channel.erase(channel_id)
	if not user_id.is_empty() and String(_by_user.get(user_id, "")) == channel_id:
		_by_user.erase(user_id)


func clear() -> void:
	_by_channel.clear()
	_by_user.clear()
