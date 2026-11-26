extends GdUnitTestSuite
## T022 service side, T003 (F119-AC-16, AC-17, AC-20; TC-CHAT-033, TC-PERF-019): pictures come only from the EduWorld
## CDN (any other host is refused before a request is made), locked sticker packs keep the server hint, GIF topics are
## the library tags, at most 3 animations play and only while visible; the models read both the REST `Message` and the
## realtime `ChatMessage` shapes, drop unknown channel types and keep nothing of a recalled message.

var http: FakeHttp = null
var cache: ChatAssetCache = null


func before_test() -> void:
	http = FakeHttp.new()
	cache = ChatAssetCache.new(http, ChatFixtures.CDN)
	cache.use_disk = false


func test_only_the_eduworld_cdn_serves_chat_pictures() -> void:
	assert_bool(cache.is_allowed(ChatFixtures.CDN + "/media/abc/x.png")).is_true()
	assert_bool(cache.is_allowed("https://gifs.example.com/x.gif")).is_false()
	assert_bool(cache.is_allowed("https://cdn.example.invalid.evil.test/x.png")).is_false()
	assert_bool(cache.is_allowed(ChatFixtures.CDN + "/media/../x.png")).is_false()
	var tex: Texture2D = await cache.texture("https://gifs.example.com/x.gif")
	assert_object(tex).is_null()
	assert_int(http.requests.size()).is_equal(0)


func test_cdn_picture_is_fetched_once_and_cached() -> void:
	http.reply_bytes(ChatFixtures.ROUTE_CDN, 200, ChatFixtures.png())
	var url: String = ChatFixtures.CDN + "/media/aaa/may_wave.png"
	var a: Texture2D = await cache.texture(url)
	var b: Texture2D = await cache.texture(url)
	assert_object(a).is_not_null()
	assert_object(b).is_same(a)
	assert_int(http.requests.size()).is_equal(1)


func test_locked_pack_keeps_the_unlock_hint_and_gif_topics_come_from_tags() -> void:
	cache.load_packs(ChatFixtures.stickers()["data"])
	assert_int(cache.packs.size()).is_equal(2)
	var locked: Dictionary = cache.packs[1]
	assert_bool(bool(locked["unlocked"])).is_false()
	assert_str(String(locked["unlock_hint"])).contains("Mặt Trăng")
	cache.load_gifs("", ChatFixtures.gifs()["data"])
	assert_array(Array(cache.topics())).contains(["cổ vũ", "chào hỏi"])
	assert_int(cache.gifs("").size()).is_equal(2)


func test_at_most_three_animations_play_and_only_while_visible() -> void:
	var budget: ChatAnimationBudget = ChatAnimationBudget.new()
	assert_bool(budget.request("a", true)).is_true()
	assert_bool(budget.request("b", true)).is_true()
	assert_bool(budget.request("c", true)).is_true()
	assert_bool(budget.request("d", true)).is_false()
	assert_bool(budget.request("e", false)).is_false()
	assert_bool(budget.request("a", false)).is_false()
	assert_bool(budget.request("d", true)).is_true()
	assert_int(budget.playing_count()).is_equal(ChatAnimationBudget.MAX_PLAYING)


func test_rest_and_realtime_message_shapes_are_read_alike() -> void:
	var page: Array = ChatFixtures.history()["data"] as Array
	var rest_emoji: ChatMessageModel = ChatMessageModel.from_dict(page[4])
	assert_str(rest_emoji.emoji).is_equal("star")
	var rest_mine: ChatMessageModel = ChatMessageModel.from_dict(page[2])
	assert_int(int(rest_mine.reactions["thumbs_up"])).is_equal(2)
	var edited: ChatMessageModel = ChatMessageModel.from_dict(page[3])
	assert_int(edited.edited_at_ms).is_greater(0)
	assert_str(edited.reply_excerpt).is_equal("Đi thôi nào!")
	var ws_emoji: ChatMessageModel = ChatMessageModel.from_dict(ChatFixtures.ws("chat.message.created.emoji"))
	assert_str(ws_emoji.emoji).is_equal("clap")
	var fwd: ChatMessageModel = ChatMessageModel.from_dict(ChatFixtures.ws("chat.message.created.reply_forwarded"))
	assert_bool(fwd.is_forwarded).is_true()
	assert_str(fwd.forwarded_nickname).is_equal("CaHeoVui07")
	assert_int(fwd.created_at_ms).is_greater(0)


func test_recalled_message_keeps_no_content_and_unknown_channel_types_are_ignored() -> void:
	var raw: Dictionary = ChatFixtures.ws("chat.message.created.text")
	raw["recalled"] = true
	var m: ChatMessageModel = ChatMessageModel.from_dict(raw)
	assert_str(m.body).is_empty()
	assert_object(ChatChannelModel.from_dict({"channel_id": ChatFixtures.PARTY_CHANNEL, "channel_type": "class",
		"allowed_kinds": ["text"]})).is_null()
	var c: ChatChannelModel = ChatChannelModel.from_dict((ChatFixtures.channels()["data"] as Array)[1])
	assert_bool(c.is_closed()).is_true()
	assert_int(c.purge_after_ms).is_greater(0)


func test_school_preset_quick_and_emote_comes_from_server_kinds() -> void:
	var c: ChatChannelModel = ChatChannelModel.from_dict({"channel_id": ChatFixtures.PARTY_CHANNEL, "channel_type": "party",
		"allowed_kinds": ["quick", "emoji"]})
	assert_bool(c.is_quick_only()).is_true()
	assert_bool(c.allows("text")).is_false()
	assert_bool(ChatChannelModel.placeholder(ChatFixtures.PARTY_CHANNEL, "party").is_quick_only()).is_false()
