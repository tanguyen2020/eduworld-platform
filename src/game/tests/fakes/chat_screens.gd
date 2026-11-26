class_name ChatScreens
extends RefCounted
## TEST-ONLY (AGENTS R17): the F119 chat states built the same way for the grade-band matrix and the review captures —
## a ChatDock on the right of the screen (where F109 / F110 embed it in a match or boss; the party lobby has its own
## screen) fed by the contract fixtures: `team` (team battle, tabs Đội / Cả trận, reactions, reply, receipts, typing),
## `menu`, `report`, `thanks`, `picker` (sticker tab with a locked pack), `blocked` (PII sentence), `muted` (composer
## locked until a time), `closed` (match ended, retention days), `care` (care message), `mini` (2-line strip while a
## question is shown). Neutral placeholder text only (AGENTS R15).

const VIEWS: PackedStringArray = ["team", "menu", "report", "thanks", "picker", "blocked", "muted", "closed", "care", "mini"]


## Points the chat of `services` at the fixtures (CDN of the fixtures, no disk cache).
static func prepare(services: ServiceLocator, http: FakeHttp) -> void:
	var chat: ChatService = services.chat
	chat.clear()
	chat._refresh_self()
	chat.assets.cdn_base_url = ChatFixtures.CDN
	chat.assets.use_disk = false
	ChatFixtures.script(http, chat.store.self_id)


## Coroutine: builds `view` in `parent` (full-screen Control) and returns the dock.
static func build(view: String, parent: Control, services: ServiceLocator, http: FakeHttp) -> ChatDock:
	prepare(services, http)
	var chat: ChatService = services.chat
	var self_id: String = chat.store.self_id
	var history: Dictionary = ChatFixtures.history(self_id)
	for m: Variant in history["data"] as Array:
		(m as Dictionary)["channel_id"] = ChatFixtures.TEAM_CHANNEL
	http.clear(ChatFixtures.ROUTE_MESSAGES).reply(ChatFixtures.ROUTE_MESSAGES, 200, history)
	if view == "closed":
		var closed: Dictionary = ChatFixtures.channels_closed_match(chat.now_ms(), 5)
		http.clear(ChatFixtures.ROUTE_CHANNELS).reply(ChatFixtures.ROUTE_CHANNELS, 200, closed)
	var dock: ChatDock = (load("res://scenes/chat/ChatDock.tscn") as PackedScene).instantiate() as ChatDock
	dock.set_anchors_preset(Control.PRESET_FULL_RECT)
	if view == "mini":
		_anchor(dock, 0.02, 0.70, 0.46, 0.97)
	else:
		# Under the top HUD row, beside the right column (where a match screen keeps its chat).
		_anchor(dock, 0.55, 0.11, 0.985, 0.97)
	parent.add_child(dock)
	dock.bind_services(chat, services.social, services.reports)
	dock.show_match(ChatFixtures.MATCH_ID, ChatFixtures.MATCH_CHANNEL, ChatFixtures.TEAM_CHANNEL,
			[{"user_id": ChatFixtures.OTHER, "nickname": "CaHeoVui07"}, {"user_id": ChatFixtures.THIRD, "nickname": "GauNau15"}])
	await _frames(parent, 14)
	if view == "closed":
		dock.select_channel(ChatFixtures.MATCH_CHANNEL)
		await _frames(parent, 4)
		return dock
	_live(chat, self_id)
	await _frames(parent, 4)
	var team: String = ChatFixtures.TEAM_CHANNEL
	match view:
		"menu":
			dock.dialogs().open_menu(chat.store.find(team, ChatFixtures.MSG_TEXT))
		"report":
			var m: ChatMessageModel = chat.store.find(team, ChatFixtures.MSG_TEXT)
			dock.dialogs().open_report(ReportService.TARGET_MESSAGE, m.message_id, m.sender_nickname, m.sender_id)
			var chip: Button = dock.sheet().find_child("Reason_rude_language", true, false) as Button
			chip.button_pressed = true
			chip.pressed.emit()
		"thanks":
			var m2: ChatMessageModel = chat.store.find(team, ChatFixtures.MSG_TEXT)
			dock.dialogs().open_report(ReportService.TARGET_MESSAGE, m2.message_id, m2.sender_nickname, m2.sender_id)
			dock.dialogs()._open_thanks()
		"picker":
			chat.assets.load_packs(ChatFixtures.stickers()["data"])
			for raw: Variant in ChatFixtures.stickers()["data"] as Array:
				for s: Variant in (raw as Dictionary)["stickers"] as Array:
					await chat.assets.texture(String((s as Dictionary)["url"]))
			dock.composer().set_board(true)
			dock.composer().picker().show_tab(ChatPicker.TAB_STICKER)
			dock.composer().picker().refresh_stickers(chat.assets)
		"blocked":
			dock.composer().input().text = "Mình ở gần đây"
			dock.composer().input().text_changed.emit()
			dock.show_notice(ChatFeedbackMapper.KEY_PII, {}, ChatFeedbackMapper.TONE_WARNING)
		"muted":
			chat.handle_message("chat.sanction.applied", {"sanction_type": "mute", "reason_code": "auto_spam",
				"until": chat.now_ms() + 10 * 60 * 1000})
			dock._refresh_lock()
		"care":
			dock.show_notice(ChatFeedbackMapper.KEY_CARE, {}, ChatFeedbackMapper.TONE_CARE)
		"mini":
			EventBus.question_shown.emit()
	await _frames(parent, 6)
	return dock


## Live messages of the team battle: a quick message, a sticker-free text, reactions, a receipt, someone typing.
static func _live(chat: ChatService, self_id: String) -> void:
	var team: String = ChatFixtures.TEAM_CHANNEL
	var p: Dictionary = ChatFixtures.ws("chat.message.created.team", self_id)
	p["seq"] = 16
	chat.handle_message("chat.message.created", p)
	var q: Dictionary = ChatFixtures.ws("chat.message.created.quick", self_id)
	q["channel_id"] = team
	q["seq"] = 17
	chat.handle_message("chat.message.created", q)
	var r: Dictionary = ChatFixtures.ws("chat.reaction.updated.counts", self_id)
	r["channel_id"] = team
	chat.handle_message("chat.reaction.updated", r)
	chat.store.set_my_reaction(ChatFixtures.MSG_MINE, "heart", true)
	chat.handle_message("chat.receipt.updated", {"channel_id": team, "member": {"user_id": ChatFixtures.OTHER,
		"nickname": "CaHeoVui07"}, "delivered_seq": 17, "read_seq": 17})
	chat.handle_message("chat.typing.updated", {"channel_id": team, "typing": [{"user_id": ChatFixtures.THIRD,
		"nickname": "GauNau15"}]})


static func _anchor(c: Control, l: float, t: float, r: float, b: float) -> void:
	c.anchor_left = l
	c.anchor_top = t
	c.anchor_right = r
	c.anchor_bottom = b
	c.offset_left = 0.0
	c.offset_top = 0.0
	c.offset_right = 0.0
	c.offset_bottom = 0.0


static func _frames(node: Node, n: int) -> void:
	var tree: SceneTree = node.get_tree() if node.is_inside_tree() else Engine.get_main_loop() as SceneTree
	for i: int in n:
		await tree.process_frame
