class_name ChatSpaceProbe
extends RefCounted
## TEST-ONLY (AGENTS R17): the "≥ 3 messages" rule of the chat layout (STUDENT_GAME_UX_SOCIAL §7, owner of the chat
## layout; F119 Clarifications 2026-11-27). On an open ChatDock (party lobby PC column, phone slide-in panel, match /
## team dock) with the composer in its default state, the visible part of the message list must be at least as tall
## as three typical text messages (a one-line text of another player: name, sentence, time — measured with a real
## ChatBubble of the band laid out at the bubble width, plus the gaps between them) or 45% of the dock content height,
## whichever is larger. The collapsed dock and the 2-line strip shown while a question is on screen are not checked.

## Typical text messages that must fit in the visible list.
const MESSAGES: int = 3
## Share of the dock content height the list must keep at least.
const SHARE: float = 0.45
const TOLERANCE_PX: float = 1.0
## A short sentence of another player (neutral placeholder, AGENTS R15) — one line in every band.
const SAMPLE_BODY: String = "Đội mình cố lên!"
const SAMPLE_SENDER: String = "0193b0a0-0000-7000-8000-0000000000a2"
const SAMPLE_CHANNEL: String = "0193b0a0-0000-7000-8000-0000000000c1"

## Numbers of the last dock checked (printed by the matrix for the evidence table).
static var last_list: float = 0.0
static var last_content: float = 0.0
static var last_row: float = 0.0
static var last_need: float = 0.0
static var last_checked: int = 0


## Coroutine: checks every open, visible ChatDock under `root` drawn in `vp`; returns one line per dock that breaks the
## rule. A screen without an open dock returns nothing (and `last_checked` = 0).
static func check(root: Node, vp: Viewport, label: String) -> PackedStringArray:
	var out: PackedStringArray = []
	last_checked = 0
	var docks: Array[Node] = root.find_children("*", "ChatDock", true, false)
	if root is ChatDock:
		docks.append(root)
	for node: Node in docks:
		var dock: ChatDock = node as ChatDock
		if dock.get_viewport() != vp or not dock.is_visible_in_tree() or dock.mode != ChatDock.Mode.EXPANDED:
			continue
		last_checked += 1
		var problem: String = await check_dock(dock, vp)
		if not problem.is_empty():
			out.append("%s: %s" % [label, problem])
	return out


## Coroutine: "" when `dock` keeps the rule, else what is missing.
static func check_dock(dock: ChatDock, vp: Viewport) -> String:
	var box: Control = dock.get_node_or_null("Panel/Box") as Control
	var list: ChatMessageList = dock.list()
	if box == null or list == null:
		return "no message list"
	var view: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size))
	var shown: Rect2 = list.get_global_rect().intersection(box.get_global_rect()).intersection(view)
	last_list = shown.size.y if list.is_visible_in_tree() else 0.0
	last_content = box.size.y
	last_row = await row_height(dock, list)
	var gap: float = float(list.get_theme_constant(&"separation", &"VBoxContainer"))
	last_need = maxf(float(MESSAGES) * last_row + float(MESSAGES - 1) * gap, SHARE * last_content)
	if last_list + TOLERANCE_PX < last_need:
		return "message list %.0f px < %.0f (%d messages of %.0f px, or %.0f%% of %.0f px content)" % [last_list,
				last_need, MESSAGES, last_row, SHARE * 100.0, last_content]
	return ""


## Coroutine: height of one typical text message bubble laid out at the bubble width of `list` (band Theme of the dock).
static func row_height(dock: ChatDock, list: ChatMessageList) -> float:
	var m: ChatMessageModel = ChatMessageModel.from_dict({"message_id": "0193b0a0-0000-7000-8000-00000000f001",
		"channel_id": SAMPLE_CHANNEL, "seq": 1, "kind": "text", "body": SAMPLE_BODY,
		"sender": {"user_id": SAMPLE_SENDER, "nickname": "GauNau15"}, "created_at": "2026-11-27T08:01:00Z"})
	var bubble: ChatBubble = ChatBubble.new()
	bubble.modulate.a = 0.0
	dock.add_child(bubble)
	bubble.bind(m, {"self_id": ""})
	var width: float = maxf(list.size.x - float(list.get_v_scroll_bar().size.x), 1.0) * ChatBubble.WIDTH_SHARE
	var h: float = 0.0
	# Wrapped labels learn their height once laid out at the width: settle a few frames.
	for i: int in 3:
		bubble.size = Vector2(width, h)
		await dock.get_tree().process_frame
		h = bubble.get_combined_minimum_size().y
	dock.remove_child(bubble)
	bubble.free()
	return h
