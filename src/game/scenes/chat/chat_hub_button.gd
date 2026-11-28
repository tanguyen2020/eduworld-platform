class_name ChatHubButton
extends Button
## The chat button of the HUD side bar (F112-AC-01, AC-05, AC-06; STUDENT_GAME_UX_SOCIAL §7): opens the HUD chat
## (`ui_requested("chat")`); on PC also `Enter` / `T` while nothing else has the keyboard focus. Its badge is the number of
## unread messages of the clubs and DMs the server counted ("99+" past 99) and a separate "@" badge for mentions — both
## STATIC (no blink, no sound — AGENTS R19) and gone once read. Hidden while a test / exam is on screen (F112-AC-06).
## On a compact phone the button shows its chat icon alone (the word stays as tooltip and accessible name). Styled by the
## generated Theme only (AGENTS R32); UI only (AGENTS R1).

const SCREEN: StringName = &"chat"
const LABEL_KEY: String = "ui.chat.hub.hud"

var _badge: Label = null
var _mention: Label = null
var _hub: ChatHubService = null
var unread: int = 0
var mentions: int = 0


func _init() -> void:
	name = "ChatHubButton"
	theme_type_variation = &"SecondaryButton"
	icon = UiBuild.icon("chat")
	text = LABEL_KEY
	autowrap_mode = TextServer.AUTOWRAP_OFF
	size_flags_horizontal = Control.SIZE_SHRINK_END
	_badge = UiBuild.data_label("", &"CountBadge", false)
	_badge.name = "Badge"
	_badge.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_badge.visible = false
	add_child(_badge)
	_mention = UiBuild.label("ui.chat.mention_badge", &"CountBadge", false)
	_mention.name = "MentionBadge"
	_mention.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_mention.visible = false
	add_child(_mention)
	pressed.connect(func() -> void: EventBus.ui_requested.emit(SCREEN, {}))
	resized.connect(_place_badges)


func _ready() -> void:
	AppState.compact_changed.connect(_apply_compact)
	_apply_compact(AppState.compact)


func _exit_tree() -> void:
	bind_hub(null)
	if AppState.compact_changed.is_connected(_apply_compact):
		AppState.compact_changed.disconnect(_apply_compact)


func bind_hub(hub: ChatHubService) -> void:
	if _hub != null:
		for pair: Array in [[_hub.badge_changed, set_badge], [_hub.availability_changed, _on_available]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	_hub = hub
	if _hub != null:
		_hub.badge_changed.connect(set_badge)
		_hub.availability_changed.connect(_on_available)
		var b: Vector2i = _hub.badge()
		set_badge(b.x, b.y)
		_on_available(_hub.available())


## The hub redraws the badge at most once a second (F112-AC-29).
func _process(_delta: float) -> void:
	if _hub != null:
		_hub.tick(Time.get_ticks_msec())


func set_badge(unread_count: int, mention_count: int) -> void:
	unread = maxi(0, unread_count)
	mentions = maxi(0, mention_count)
	_badge.text = ChatHubState.badge_text(unread) if unread > 0 else ""
	_badge.visible = unread > 0
	_mention.visible = mentions > 0
	accessibility_description = tr("ui.chat.hub.unread").format({"count": _badge.text}) if unread > 0 else ""
	_place_badges()


func badge_text() -> String:
	return _badge.text if _badge.visible else ""


func mention_visible() -> bool:
	return _mention.visible


func _unhandled_key_input(event: InputEvent) -> void:
	var key: InputEventKey = event as InputEventKey
	if key == null or not key.pressed or key.echo or not is_visible_in_tree() or AppState.compact:
		return
	if key.keycode != KEY_T and key.keycode != KEY_ENTER:
		return
	if get_viewport().gui_get_focus_owner() != null:
		return
	get_viewport().set_input_as_handled()
	EventBus.ui_requested.emit(SCREEN, {})


func _on_available(available: bool) -> void:
	visible = available


func _apply_compact(on: bool) -> void:
	UiBuild.icon_only(self, LABEL_KEY, on)
	_place_badges()


## The badges sit inside the top-right corner of the button ("@" left of the number).
func _place_badges() -> void:
	var x: float = size.x
	for b: Label in [_badge, _mention]:
		if not b.visible:
			continue
		var s: Vector2 = b.get_combined_minimum_size()
		b.size = s
		x -= s.x
		b.position = Vector2(x, -s.y * 0.6)
