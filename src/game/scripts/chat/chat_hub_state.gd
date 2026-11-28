class_name ChatHubState
extends RefCounted
## F112 (F112-AC-01, AC-03, AC-04, AC-05, AC-10; data-model `ChatHubState`; CHAT_SYSTEM §2, §4, §7.1, §8.2): the tabs of
## the HUD chat, pure (no Node, no network). Built from the channels the server listed (`listMyChannels`), the clubs of
## the student (`listClubs` `scope = mine`, F111), the friends (`listFriends`, F110) and the DMs opened this session:
## - only channels the server returned make tabs, in this order — "Khu vực" (`area`, only while the student stands in a
##   hub instance), "Cả trường" (`world`), one tab per club whose `chat_channel_id` is a listed `team` channel, "Bạn bè"
##   (when DMs are allowed or a `direct` channel is listed), then the DMs opened this session; an unknown
##   `channel_type` never reaches here (ChatChannelModel drops it) and there is no tab per class or grade (ADR-0095);
## - a new `area` channel (another zone / instance) replaces the old one: `rebuild` returns the old id to leave and drop;
## - World / Area have no unread count (CHAT_SYSTEM §7.1) — only the "@" badge; club and DM counts come from the store
##   (`listMyChannels`, `chat.unread.updated` replaces, never adds); a DM whose friend is unknown only adds to the badge
##   of "Bạn bè" (F112-AC-10, R16);
## - the HUD badge is the sum of the unread counts the server sent, and the "@" badge the sum of the mentions.

var tabs: Array[PersistentChannelTab] = []
var peers: DirectPeerMap = DirectPeerMap.new()
## Tab on screen ("" = none).
var open_tab: String = ""
var area_channel_id: String = ""
## Friends whose DM tab was opened this session (order of opening).
var dm_users: PackedStringArray = []
var dm_allowed: bool = true


## Rebuilds the tabs. `listed`: the channels of `listMyChannels`; `store`: live unread / mentions / status (may be
## null); `clubs`: `{club_id, name, chat_channel_id}` of the student's clubs; `friends`: `{user_id, nickname, level,
## avatar_config}`; `in_hub`: the student stands in a hub instance (no match / full-screen activity).
## Returns `{leave: channel ids to leave and drop}` (an `area` channel replaced or gone).
func rebuild(listed: Array[ChatChannelModel], store: ChatStore, clubs: Array[Dictionary], friends: Array[Dictionary],
		in_hub: bool) -> Dictionary:
	var leave: PackedStringArray = []
	var by_type: Dictionary = {}
	for c: ChatChannelModel in listed:
		if c == null:
			continue
		if not by_type.has(c.channel_type):
			by_type[c.channel_type] = []
		(by_type[c.channel_type] as Array).append(c)
		peers.from_channel(c)
	for f: Dictionary in friends:
		var cid: String = peers.channel_of(DataRead.str_of(f, "user_id"))
		if not cid.is_empty():
			peers.remember(cid, f)
	var next: Array[PersistentChannelTab] = []
	# Khu vực: the area channel of the instance the student stands in.
	var area: ChatChannelModel = null
	if by_type.has(ChatChannelModel.TYPE_AREA):
		area = (by_type[ChatChannelModel.TYPE_AREA] as Array).front() as ChatChannelModel
	var area_id: String = area.channel_id if area != null and in_hub else ""
	if not area_channel_id.is_empty() and area_channel_id != area_id:
		leave.append(area_channel_id)
	area_channel_id = area_id
	if not area_id.is_empty():
		next.append(_channel_tab(PersistentChannelTab.KIND_AREA, "area", area, store, "ui.chat.hub.tab.area"))
	if by_type.has(ChatChannelModel.TYPE_WORLD):
		next.append(_channel_tab(PersistentChannelTab.KIND_WORLD, "world",
				(by_type[ChatChannelModel.TYPE_WORLD] as Array).front() as ChatChannelModel, store, "ui.chat.hub.tab.world"))
	var teams: Dictionary = {}
	for c: Variant in by_type.get(ChatChannelModel.TYPE_TEAM, []) as Array:
		teams[(c as ChatChannelModel).channel_id] = c
	for club: Dictionary in clubs:
		var cid: String = DataRead.str_of(club, "chat_channel_id")
		if not teams.has(cid):
			continue
		var t: PersistentChannelTab = _channel_tab(PersistentChannelTab.KIND_CLUB,
				"club:%s" % DataRead.str_of(club, "club_id"), teams[cid] as ChatChannelModel, store, "")
		t.club_id = DataRead.str_of(club, "club_id")
		t.title = DataRead.str_of(club, "name")
		next.append(t)
	var directs: Array = by_type.get(ChatChannelModel.TYPE_DIRECT, []) as Array
	if dm_allowed or not directs.is_empty():
		var friends_tab: PersistentChannelTab = PersistentChannelTab.make(PersistentChannelTab.KIND_FRIENDS,
				PersistentChannelTab.FRIENDS_ID)
		friends_tab.title_key = "ui.chat.hub.tab.friends"
		for c: Variant in directs:
			friends_tab.unread += _unread(c as ChatChannelModel, store)
			friends_tab.mentions += _mentions(c as ChatChannelModel, store)
		next.append(friends_tab)
	for user_id: String in dm_users:
		var cid: String = peers.channel_of(user_id)
		var dm: ChatChannelModel = null
		for c: Variant in directs:
			if (c as ChatChannelModel).channel_id == cid:
				dm = c as ChatChannelModel
		if dm == null and store != null:
			dm = store.channel(cid)
		if dm == null:
			continue
		var t: PersistentChannelTab = _channel_tab(PersistentChannelTab.KIND_DM, "dm:%s" % user_id, dm, store, "")
		t.user_id = user_id
		t.peer = peers.peer(cid)
		t.title = DataRead.str_of(t.peer, "nickname")
		next.append(t)
	tabs = next
	if not open_tab.is_empty() and tab(open_tab) == null:
		open_tab = ""
	return {"leave": leave}


## A friend's DM was opened ("Nhắn tin"): its tab stays for the session.
func open_dm(user_id: String) -> void:
	if Uuid.is_valid(user_id) and not user_id in dm_users:
		dm_users.append(user_id)


## The channel left the student (left / removed from the club, history `404`): its DM tab goes too.
func forget_channel(channel_id: String) -> void:
	var user_id: String = peers.user_of(channel_id)
	if not user_id.is_empty():
		var i: int = dm_users.find(user_id)
		if i >= 0:
			dm_users.remove_at(i)
	if channel_id == area_channel_id:
		area_channel_id = ""


func tab(tab_id: String) -> PersistentChannelTab:
	for t: PersistentChannelTab in tabs:
		if t.tab_id == tab_id:
			return t
	return null


func tab_of_channel(channel_id: String) -> PersistentChannelTab:
	for t: PersistentChannelTab in tabs:
		if t.channel_id == channel_id and not channel_id.is_empty():
			return t
	return null


func tab_ids() -> PackedStringArray:
	var out: PackedStringArray = []
	for t: PersistentChannelTab in tabs:
		out.append(t.tab_id)
	return out


## Channel ids of every tab (what the hub may have joined).
func channel_ids() -> PackedStringArray:
	var out: PackedStringArray = []
	for t: PersistentChannelTab in tabs:
		if not t.channel_id.is_empty():
			out.append(t.channel_id)
	return out


## HUD badge: the unread messages of the clubs and the DMs (World / Area never count).
func hud_unread() -> int:
	var n: int = 0
	for t: PersistentChannelTab in tabs:
		if t.kind in [PersistentChannelTab.KIND_CLUB, PersistentChannelTab.KIND_FRIENDS]:
			n += t.unread
	return n


## HUD "@" badge: the unread mentions of every channel.
func hud_mentions() -> int:
	var n: int = 0
	for t: PersistentChannelTab in tabs:
		if t.kind != PersistentChannelTab.KIND_DM:
			n += t.mentions
	return n


## Unread of the DM with `user_id` when its channel is known ("Bạn bè" list rows).
func friend_unread(user_id: String, store: ChatStore) -> int:
	var cid: String = peers.channel_of(user_id)
	return store.unread(cid) if store != null and not cid.is_empty() else 0


func clear() -> void:
	tabs.clear()
	peers.clear()
	open_tab = ""
	area_channel_id = ""
	dm_users.clear()


static func badge_text(n: int) -> String:
	return TranslationServer.translate("ui.chat.unread_many") if n > 99 else str(n)


func _channel_tab(kind_name: String, id: String, c: ChatChannelModel, store: ChatStore, key: String) -> PersistentChannelTab:
	var t: PersistentChannelTab = PersistentChannelTab.make(kind_name, id)
	t.channel_id = c.channel_id
	t.channel_type = c.channel_type
	t.title_key = key
	var live: ChatChannelModel = store.channel(c.channel_id) if store != null else null
	t.status = live.status if live != null else c.status
	t.muted = c.muted
	t.unread = _unread(c, store) if t.counts_unread() else 0
	t.mentions = _mentions(c, store)
	return t


func _unread(c: ChatChannelModel, store: ChatStore) -> int:
	var live: ChatChannelModel = store.channel(c.channel_id) if store != null else null
	return live.unread_count if live != null else c.unread_count


func _mentions(c: ChatChannelModel, store: ChatStore) -> int:
	var live: ChatChannelModel = store.channel(c.channel_id) if store != null else null
	return live.unread_mentions if live != null else c.unread_mentions
