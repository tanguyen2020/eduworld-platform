class_name ChatDialogs
extends RefCounted
## The sheets of the chat panel on one ChatSheet (F119 US3/US4; STUDENT_GAME_UX_SOCIAL §7, CHAT_SYSTEM §4.3, §9):
## - message menu (long press / right click / "⋯" = 1 action): the 6 quick reactions, then Trả lời · Trích dẫn ·
##   Chuyển tiếp · Sao chép · Ghim · Sửa · Thu hồi · Ai đã xem as the channel features and the author allow, then — only
##   on another player's message — Tắt tiếng · Chặn · Báo cáo (Báo cáo = action 2, F119-AC-12, SC-001);
## - "Thu hồi" always asks first (F119-AC-19); "Sao chép" copies what the bubble shows (masked text / placeholder);
## - report: a reason chosen by picture + word (MODERATION §5), `createReport` with the match / party / channel context,
##   then the thanks with the offer to block (F119-AC-13); also for a player (no message) from the players list or a
##   result screen (F119-AC-14);
## - block / mute call SocialService; the messages of that player leave the list once the server confirmed (AC-15).
## Nicknames only (AGENTS R16); no chat content is sent with a report (the server holds the evidence). UI only.

## A composer action of the menu (reply, quote, edit) for the panel.
signal composer_action_chosen(action: StringName, message: ChatMessageModel)
## A friendly notice (`key`, args, tone).
signal notice_requested(key: String, args: Dictionary, tone: String)

const ACT_REPLY: StringName = &"reply"
const ACT_QUOTE: StringName = &"quote"
const ACT_FORWARD: StringName = &"forward"
const ACT_COPY: StringName = &"copy"
const ACT_PIN: StringName = &"pin"
const ACT_UNPIN: StringName = &"unpin"
const ACT_EDIT: StringName = &"edit"
const ACT_RECALL: StringName = &"recall"
const ACT_READERS: StringName = &"readers"
const ACT_MUTE: StringName = &"mute"
const ACT_BLOCK: StringName = &"block"
const ACT_REPORT: StringName = &"report"
const ACT_SEND: StringName = &"send"
const ACT_CONFIRM: StringName = &"confirm"
## Menu entries: action, label key, icon.
const MENU_ITEMS: Array = [
	[ACT_REPLY, "ui.chat.menu.reply", "reply"], [ACT_QUOTE, "ui.chat.menu.quote", "quote"],
	[ACT_FORWARD, "ui.chat.menu.forward", "forward"], [ACT_COPY, "ui.chat.menu.copy", "copy"],
	[ACT_PIN, "ui.chat.menu.pin", "pin"], [ACT_UNPIN, "ui.chat.menu.unpin", "pin"],
	[ACT_EDIT, "ui.chat.menu.edit", "edit"], [ACT_RECALL, "ui.chat.menu.recall", "recall"],
	[ACT_READERS, "ui.chat.menu.readers", "tick_double_read"],
]
const SAFETY_ITEMS: Array = [
	[ACT_MUTE, "ui.chat.menu.mute", "sound_off"], [ACT_BLOCK, "ui.chat.menu.block", "block"],
	[ACT_REPORT, "ui.chat.menu.report", "flag"],
]
## Report reasons (ReportService.REASONS) → label key and picture.
const REASON_ITEMS: Dictionary = {
	"rude_language": ["ui.chat.report.reason.rude_language", "report_rude"],
	"bullying": ["ui.chat.report.reason.bullying", "emoji_sad"],
	"asked_personal_info": ["ui.chat.report.reason.asked_personal_info", "lock"],
	"spam": ["ui.chat.report.reason.spam", "report_spam"],
	"inappropriate_content": ["ui.chat.report.reason.inappropriate_content", "eye_off"],
	"cheating": ["ui.chat.report.reason.cheating", "report_cheating"],
	"other": ["ui.chat.report.reason.other", "help"],
}

var sheet: ChatSheet = null
var chat: ChatService = null
var social: SocialService = null
var reports: ReportService = null
## Report context of the panel: `{channel_id, match_id, party_id}`.
var context: Dictionary = {}
## What the sheet shows now: &"menu", &"recall", &"report", &"thanks", &"forward", &"readers", &"players".
var view: StringName = &""
var _message: ChatMessageModel = null
var _target: Dictionary = {}
var _reason: String = ""
var _forward_ids: PackedStringArray = []


func _init(on_sheet: ChatSheet) -> void:
	sheet = on_sheet
	sheet.chosen.connect(_on_chosen)


func bind(chat_service: ChatService, social_service: SocialService, report_service: ReportService) -> void:
	chat = chat_service
	social = social_service
	reports = report_service


# ---------- message menu ----------

## Opens the menu of `m` (1 action). Returns the actions shown (tests, SC-001).
func open_menu(m: ChatMessageModel) -> Array[StringName]:
	_message = m
	view = &"menu"
	var shown: Array[StringName] = []
	var mine: bool = chat != null and m.is_mine(chat.store.self_id)
	var title: String = TranslationServer.translate("ui.chat.menu.title_own" if mine else "ui.chat.menu.title").format(
			{"nickname": m.sender_nickname})
	var body: VBoxContainer = sheet.open_sheet(title)
	if chat != null and chat.can_react(m):
		var row: HFlowContainer = HFlowContainer.new()
		row.name = "Reactions"
		row.alignment = FlowContainer.ALIGNMENT_CENTER
		body.add_child(row)
		for code: String in ChatText.QUICK_REACTIONS:
			var b: Button = UiBuild.button("", ChatText.emoji_icon(code), &"ChoiceChip")
			b.name = "React_" + code
			b.toggle_mode = true
			b.button_pressed = code in m.my_reactions
			b.tooltip_text = String(ChatText.EMOJI_KEYS[code])
			b.expand_icon = true
			b.pressed.connect(_react.bind(code))
			row.add_child(b)
	var actions: HFlowContainer = HFlowContainer.new()
	actions.name = "Actions"
	body.add_child(actions)
	for spec: Array in MENU_ITEMS:
		var act: StringName = spec[0] as StringName
		if _allowed(act, m, mine):
			actions.add_child(_action_button(act, String(spec[1]), String(spec[2])))
			shown.append(act)
	if not mine and not m.is_system():
		var tag: Label = UiBuild.label("ui.chat.menu.safety", &"SectionTag", false)
		tag.uppercase = true
		tag.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
		body.add_child(tag)
		var safety: HFlowContainer = HFlowContainer.new()
		safety.name = "Safety"
		body.add_child(safety)
		for spec: Array in SAFETY_ITEMS:
			safety.add_child(_action_button(spec[0] as StringName, String(spec[1]), String(spec[2])))
			shown.append(spec[0] as StringName)
	sheet.add_button(ChatSheet.CANCEL, "ui.common.close", "cross")
	UiBuild.size_buttons(body)
	return shown


func _allowed(act: StringName, m: ChatMessageModel, mine: bool) -> bool:
	if chat == null:
		return act == ACT_COPY
	match act:
		ACT_REPLY:
			return chat.can_reply(m)
		ACT_QUOTE:
			return chat.can_quote(m)
		ACT_FORWARD:
			return chat.can_forward(m)
		ACT_COPY:
			return not m.is_pending()
		ACT_PIN:
			return chat.can_pin(m) and not m.pinned
		ACT_UNPIN:
			return chat.can_pin(m) and m.pinned
		ACT_EDIT:
			return chat.can_edit(m)
		ACT_RECALL:
			return chat.can_recall(m)
		ACT_READERS:
			var ch: ChatChannelModel = chat.store.channel(m.channel_id)
			return mine and m.seq > 0 and ch != null and ch.receipts_mode() == ChatChannelModel.RECEIPTS_PER_MEMBER
	return false


func _action_button(act: StringName, key: String, icon: String) -> Button:
	var b: Button = UiBuild.button(key, icon, &"SecondaryButton")
	b.name = "Act_" + String(act)
	b.pressed.connect(_on_action.bind(act))
	return b


func _on_action(act: StringName) -> void:
	var m: ChatMessageModel = _message
	if m == null:
		return
	match act:
		ACT_REPLY, ACT_QUOTE, ACT_EDIT:
			sheet.dismiss()
			view = &""
			composer_action_chosen.emit(act, m)
		ACT_COPY:
			sheet.dismiss()
			view = &""
			DisplayServer.clipboard_set(chat.copy_text(m) if chat != null else ChatText.visible_text(m))
			notice_requested.emit("ui.chat.copied", {}, ChatFeedbackMapper.TONE_INFO)
		ACT_PIN, ACT_UNPIN:
			sheet.dismiss()
			view = &""
			await chat.set_pinned(m, act == ACT_PIN)
		ACT_RECALL:
			open_recall_confirm(m)
		ACT_FORWARD:
			open_forward(m)
		ACT_READERS:
			open_readers(m)
		ACT_MUTE, ACT_BLOCK:
			sheet.dismiss()
			view = &""
			await relation(act, m.sender_id, m.sender_nickname)
		ACT_REPORT:
			open_report(ReportService.TARGET_MESSAGE, m.message_id, m.sender_nickname, m.sender_id)


func _react(code: String) -> void:
	var m: ChatMessageModel = _message
	sheet.dismiss()
	view = &""
	if chat != null and m != null:
		await chat.toggle_reaction(m, code)


# ---------- recall, forward, readers ----------

func open_recall_confirm(m: ChatMessageModel) -> void:
	_message = m
	view = &"recall"
	var body: VBoxContainer = sheet.open_sheet(TranslationServer.translate("ui.chat.recall.title"), true)
	var l: Label = UiBuild.label("ui.chat.recall.question")
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	body.add_child(l)
	sheet.add_button(ChatSheet.CANCEL, "ui.common.cancel", "cancel")
	sheet.add_button(ACT_CONFIRM, "ui.chat.menu.recall", "recall")


func open_forward(m: ChatMessageModel) -> void:
	_message = m
	view = &"forward"
	_forward_ids.clear()
	var body: VBoxContainer = sheet.open_sheet(TranslationServer.translate("ui.chat.forward.title"))
	body.add_child(UiBuild.label("ui.chat.forward.hint", &"MutedLabel"))
	var row: HFlowContainer = HFlowContainer.new()
	row.name = "Targets"
	body.add_child(row)
	for c: ChatChannelModel in chat.forward_targets(m):
		var chip: Button = UiBuild.chip("")
		chip.name = "Target"
		chip.text = channel_name(c)
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.pressed.connect(func() -> void:
			if chip.button_pressed and _forward_ids.size() < 5:
				_forward_ids.append(c.channel_id)
			else:
				chip.button_pressed = false
				var kept: PackedStringArray = []
				for id: String in _forward_ids:
					if id != c.channel_id:
						kept.append(id)
				_forward_ids = kept
			UiBuild.mark_chips(row.get_children()))
		row.add_child(chip)
	sheet.add_button(ChatSheet.CANCEL, "ui.common.cancel", "cancel")
	sheet.add_button(ACT_SEND, "ui.chat.forward.send", "forward_light", &"")
	UiBuild.size_buttons(body)


func open_readers(m: ChatMessageModel) -> void:
	view = &"readers"
	var who: Dictionary = chat.store.readers(m)
	var body: VBoxContainer = sheet.open_sheet(TranslationServer.translate("ui.chat.readers.title"))
	for spec: Array in [["read", "ui.chat.readers.read", "tick_double_read"], ["delivered", "ui.chat.readers.delivered", "tick_double"]]:
		var names: PackedStringArray = who[spec[0]] as PackedStringArray
		var row: HBoxContainer = UiBuild.hbox()
		row.add_child(UiBuild.icon_rect(String(spec[2]), 0.8))
		var text: String = TranslationServer.translate(String(spec[1])).format({"names": ", ".join(names) if not names.is_empty()
			else TranslationServer.translate("ui.chat.readers.nobody")})
		var l: Label = UiBuild.data_label(text)
		UiBuild.expand(l)
		row.add_child(l)
		body.add_child(row)
	sheet.add_button(ChatSheet.CANCEL, "ui.common.close", "cross")


# ---------- report, block, mute ----------

## Report of a message (`target_type = message`) or of a player (`user`). `user_id` is the player (block offer).
func open_report(target_type: String, target_id: String, nickname: String, user_id: String) -> void:
	view = &"report"
	_target = {"type": target_type, "id": target_id, "nickname": nickname, "user_id": user_id}
	_reason = ""
	var body: VBoxContainer = sheet.open_sheet(TranslationServer.translate("ui.chat.report.title").format({"nickname": nickname}))
	body.add_child(UiBuild.label("ui.chat.report.question"))
	var grid: HFlowContainer = HFlowContainer.new()
	grid.name = "Reasons"
	body.add_child(grid)
	var group: ButtonGroup = ButtonGroup.new()
	for reason: String in ReportService.REASONS:
		var spec: Array = REASON_ITEMS[reason] as Array
		var chip: Button = UiBuild.button(String(spec[0]), String(spec[1]), &"ChoiceChip")
		chip.name = "Reason_" + reason
		chip.toggle_mode = true
		chip.button_group = group
		chip.set_meta("idle_icon", chip.icon)
		chip.pressed.connect(func() -> void:
			_reason = reason
			for c: Node in grid.get_children():
				var cb: Button = c as Button
				cb.icon = UiBuild.icon("check") if cb.button_pressed else cb.get_meta("idle_icon") as Texture2D
			var send: Button = sheet.footer_button(ACT_SEND)
			if send != null:
				send.disabled = false
				send.icon = UiBuild.icon("flag_light"))
		grid.add_child(chip)
	sheet.add_button(ChatSheet.CANCEL, "ui.common.cancel", "cancel")
	var send_btn: Button = sheet.add_button(ACT_SEND, "ui.chat.report.send", "flag", &"")
	# The white flag is for the blue face; the grey disabled button shows the blue one so it stays readable.
	send_btn.disabled = true
	UiBuild.size_buttons(body)


func chosen_reason() -> String:
	return _reason


## Coroutine: block / mute a player (server first; the list hides the player's messages on success).
func relation(act: StringName, user_id: String, nickname: String) -> bool:
	if social == null:
		return false
	var result: ApiResult = null
	if act == ACT_BLOCK:
		result = await social.block_user(user_id)
	else:
		result = await social.mute_user(user_id)
	if result.ok:
		notice_requested.emit("ui.chat.blocked_done" if act == ACT_BLOCK else "ui.chat.muted_done", {"nickname": nickname},
				ChatFeedbackMapper.TONE_INFO)
	else:
		var fb: Dictionary = ChatFeedbackMapper.for_code(result.code)
		notice_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
	return result.ok


func _send_report() -> void:
	if reports == null or _reason.is_empty():
		return
	var result: ApiResult = await reports.create_report(String(_target["type"]), String(_target["id"]), _reason, context)
	if not result.ok:
		var fb: Dictionary = ChatFeedbackMapper.for_code(result.code)
		notice_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
		return
	_open_thanks()


func _open_thanks() -> void:
	view = &"thanks"
	var body: VBoxContainer = sheet.open_sheet(TranslationServer.translate("ui.chat.report.thanks_title"))
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	var box: VBoxContainer = coach["bubble_box"] as VBoxContainer
	box.add_child(UiBuild.label("ui.chat.report.thanks", &"SpeechBubbleLabel"))
	box.add_child(UiBuild.data_label(TranslationServer.translate("ui.chat.report.block_offer").format(
			{"nickname": String(_target["nickname"])}), &"SpeechBubbleLabel"))
	body.add_child(coach["row"] as Control)
	sheet.add_button(ACT_BLOCK, "ui.chat.menu.block", "block")
	sheet.add_button(ChatSheet.CANCEL, "ui.chat.report.done", "check", &"")


## The players of the match / party with "Báo cáo người chơi" and "Chặn" (F119-AC-14).
func open_players(players: Array[Dictionary]) -> void:
	view = &"players"
	var body: VBoxContainer = sheet.open_sheet(TranslationServer.translate("ui.chat.players.title"))
	var list: ChatPlayerSafety = ChatPlayerSafety.new()
	list.set_players(players)
	list.report_requested.connect(func(user_id: String, nickname: String) -> void:
		open_report(ReportService.TARGET_USER, user_id, nickname, user_id))
	list.block_requested.connect(func(user_id: String, nickname: String) -> void:
		sheet.dismiss()
		view = &""
		await relation(ACT_BLOCK, user_id, nickname))
	body.add_child(list)
	sheet.add_button(ChatSheet.CANCEL, "ui.common.close", "cross")


func _on_chosen(action: StringName) -> void:
	var was: StringName = view
	view = &""
	match [was, action]:
		[&"recall", ACT_CONFIRM]:
			await chat.recall(_message)
		[&"forward", ACT_SEND]:
			await chat.forward(_message, _forward_ids)
		[&"report", ACT_SEND]:
			await _send_report()
		[&"thanks", ACT_BLOCK]:
			await relation(ACT_BLOCK, String(_target["user_id"]), String(_target["nickname"]))


## Name of a channel in the forward list (server title, else the kind of channel).
static func channel_name(c: ChatChannelModel) -> String:
	if not c.title.is_empty():
		return c.title
	match c.channel_type:
		ChatChannelModel.TYPE_PARTY:
			return TranslationServer.translate("ui.chat.title.party")
		ChatChannelModel.TYPE_TEAM:
			return TranslationServer.translate("ui.chat.tab.team")
		ChatChannelModel.TYPE_MATCH:
			return TranslationServer.translate("ui.chat.tab.match")
		ChatChannelModel.TYPE_DIRECT:
			return TranslationServer.translate("ui.chat.title.direct")
	return TranslationServer.translate("ui.chat.title.generic")
