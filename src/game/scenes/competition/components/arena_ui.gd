class_name ArenaUi
extends RefCounted
## Builders shared by the arena screens (F109; DESIGN_SYSTEM §5.1 F114 tiers, STUDENT_GAME_UX_SOCIAL §8): the window
## frame of the Party / Settings screens (tier 1 frame over the scrim, blue `WindowTitleBar` with an icon, an upper-case
## title and its buttons), player rows (portrait + nickname only, AGENTS R16), tags in words (state is never shown by
## colour alone) and a Mây line. Theme variations only (R32); texts are keys.

## Width / height above which a screen uses the phone layout (F113 landscape phones, like the Party screen).
const PHONE_ASPECT: float = 1.9


## A window over the scrim: `{window, box, bar_row, title, close}`. `margins` = share of the screen on each side.
static func window(root: Control, title_key: String, icon_name: String, margin_x: float = 0.015, margin_y: float = 0.02) -> Dictionary:
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.mouse_filter = Control.MOUSE_FILTER_STOP
	root.add_child(UiBuild.scrim())
	var win: PanelContainer = UiBuild.panel()
	win.name = "Window"
	win.set_anchors_preset(Control.PRESET_FULL_RECT)
	win.anchor_left = margin_x
	win.anchor_top = margin_y
	win.anchor_right = 1.0 - margin_x
	win.anchor_bottom = 1.0 - margin_y
	for side: Side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
		win.set_offset(side, 0.0)
	root.add_child(win)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.name = "Box"
	win.add_child(box)
	var bar: PanelContainer = UiBuild.panel(&"WindowTitleBar")
	bar.name = "TitleBar"
	box.add_child(bar)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	bar.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect(icon_name)
	icon.name = "TitleIcon"
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var title: Label = UiBuild.label(title_key, &"WindowTitleLabel", false)
	title.name = "Title"
	title.uppercase = true
	title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(title)
	row.add_child(title)
	var close: Button = UiBuild.button("ui.common.close", "cross", &"DangerButton")
	close.name = "Close"
	close.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(close)
	return {"window": win, "box": box, "bar_row": row, "title": title, "close": close}


## A player row: portrait + nickname (+ an optional tag in words on the right).
static func player_row(nickname: String, avatar_config: Dictionary, tag_key: String = "", variation: StringName = &"PartyMemberCard") -> PanelContainer:
	var card: PanelContainer = UiBuild.panel(variation)
	card.name = "Player"
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	card.add_child(row)
	row.add_child(portrait(avatar_config))
	var name_label: Label = UiBuild.data_label(nickname, &"HeadingLabel", false)
	name_label.name = "Nickname"
	# A nickname stays on one line (≤ 32 characters, never cut): its row grows instead (F109-AC-30 edge case).
	name_label.tooltip_text = nickname
	name_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(name_label)
	row.add_child(name_label)
	if not tag_key.is_empty():
		row.add_child(tag(tag_key))
	return card


static func portrait(avatar_config: Dictionary, scale: float = 1.0) -> AvatarThumb:
	var thumb: AvatarThumb = AvatarThumb.new()
	thumb.standalone = true
	thumb.custom_minimum_size = Vector2.ONE * UiBuild.target() * scale
	thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	if not avatar_config.is_empty():
		thumb.set_look(AvatarConfigModel.from_dict(avatar_config))
	return thumb


## A pill tag in words (`SectionTag`; `TileTagCorrect` for a good state, `TileTagPending` for a waiting one).
static func tag(key: String, variation: StringName = &"SectionTag") -> Label:
	var l: Label = UiBuild.label(key, variation, false)
	l.name = "Tag"
	l.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	return l


## Width of the longest word of a label in its own font (a wrapping label never gets narrower, no word cut).
static func longest_word_width(l: Label) -> float:
	var font: Font = l.get_theme_font(&"font")
	if font == null:
		return 0.0
	var px: int = l.get_theme_font_size(&"font_size")
	var widest: float = 0.0
	for word: String in l.text.split(" ", false):
		widest = maxf(widest, font.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x)
	return ceilf(widest) + 2.0


## A section tag (yellow pill, upper case) that hugs its words on the left of a column.
static func section(key: String) -> Label:
	var l: Label = UiBuild.label(key, &"SectionTag", false)
	l.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	return l


## Mây beside a short line (encouraging only — never a sad face, NFR-SAFE-003). Returns the row.
static func coach_line(key: String, args: Dictionary = {}, mood: CloudyMascot.Mood = CloudyMascot.Mood.NEUTRAL) -> HBoxContainer:
	var coach: Dictionary = UiBuild.coach(mood)
	var line: Label = UiBuild.data_label(TranslationServer.translate(key).format(args), &"SpeechBubbleLabel")
	line.name = "CoachLine"
	(coach["bubble_box"] as VBoxContainer).add_child(line)
	# The bubble hugs its line (not the height of Mây).
	var row: HBoxContainer = coach["row"] as HBoxContainer
	(row.get_node("Bubble") as Control).size_flags_vertical = Control.SIZE_SHRINK_CENTER
	return row


## A notice panel (info / warning / error) with its icon and a label; returns `{panel, icon, label}`.
static func notice() -> Dictionary:
	var panel: PanelContainer = UiBuild.panel(&"InfoBannerPanel")
	panel.name = "Notice"
	panel.visible = false
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	panel.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("info")
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	var label: Label = UiBuild.data_label("", &"InfoBannerLabel")
	label.name = "NoticeLabel"
	UiBuild.expand(label)
	row.add_child(label)
	return {"panel": panel, "icon": icon, "label": label}


## Shows `key` (+ args) in a `notice()` dictionary; `tone` "info" | "warning" | "error".
static func show_notice(n: Dictionary, key: String, args: Dictionary = {}, tone: String = "info") -> void:
	var panel: PanelContainer = n["panel"] as PanelContainer
	var label: Label = n["label"] as Label
	var types: Dictionary = {"info": [&"InfoBannerPanel", &"InfoBannerLabel", "info"],
		"warning": [&"WarningBannerPanel", &"WarningBannerLabel", "wifi_off"],
		"error": [&"InlineErrorPanel", &"InlineErrorLabel", "alert"]}
	var t: Array = types.get(tone, types["info"]) as Array
	panel.theme_type_variation = t[0] as StringName
	label.theme_type_variation = t[1] as StringName
	(n["icon"] as TextureRect).texture = UiBuild.icon(String(t[2]))
	label.text = TranslationServer.translate(key).format(args)
	panel.visible = not key.is_empty()


static func is_phone(control: Control) -> bool:
	var view: Vector2 = control.get_viewport_rect().size if control.is_inside_tree() else control.size
	return view.y > 0.0 and view.x / view.y > PHONE_ASPECT


## Grade band of the student (presentation only: density, seconds hidden for g1_3, bracket path for g1_5).
static func band() -> String:
	return UiBuild.band()


## Number in the student's locale (server values only).
static func num(value: float) -> String:
	return LocaleFormat.number(value, "", 0)


## A scrolling column (body of a tab): `{scroll, box}`.
static func scroll_box() -> Dictionary:
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(box)
	scroll.add_child(box)
	return {"scroll": scroll, "box": box}
