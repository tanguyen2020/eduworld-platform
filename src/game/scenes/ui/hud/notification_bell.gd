extends Button
## Bell of the HUD (STUDENT_GAME_UX_CORE S03 `NotificationBell`, F123-AC-15): the bell icon of design/icons/game
## (F096-AC-15: no emoji) and, when the student has unread notifications, the count in a `CountBadge` pill on the right
## of the button ("99+" past 99) — the number carries the state, never a colour alone (NFR-A11Y-001). The tooltip /
## screen-reader name says "Em có {count} thông báo mới". Pressing it is an intent handled by the HUD owner (the inbox
## opens). UI only.

## Gap between the bell icon and the count pill, as a share of the icon size.
const GAP_FACTOR: float = 0.25

var count: int = 0

@onready var _count: Label = %Count


func _ready() -> void:
	resized.connect(_place_badge)
	set_count(count)


func _notification(what: int) -> void:
	if what == NOTIFICATION_TRANSLATION_CHANGED and is_node_ready():
		set_count(count)


func set_count(value: int) -> void:
	count = maxi(0, value)
	if _count == null:
		return
	_count.text = NotificationService.badge_text(count)
	_count.visible = count > 0
	text = ""
	tooltip_text = tr("ui.notification.bell_unread").format({"count": count}) if count > 0 \
			else tr("ui.notification.bell_tooltip")
	accessibility_name = tooltip_text
	icon_alignment = HORIZONTAL_ALIGNMENT_LEFT if count > 0 else HORIZONTAL_ALIGNMENT_CENTER
	_fit_width()
	_place_badge()


## The button grows to hold the icon and the pill side by side (never over the icon); square again at zero.
func _fit_width() -> void:
	var height: float = maxf(custom_minimum_size.y, float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[UiBuild.band()]))
	if count <= 0:
		custom_minimum_size.x = height
		return
	var box: StyleBox = get_theme_stylebox(&"normal")
	var pad: float = box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT) if box != null else 0.0
	var icon_w: float = float(get_theme_constant(&"icon_max_width"))
	var pill: Vector2 = _count.get_combined_minimum_size()
	custom_minimum_size.x = ceilf(maxf(height, pad + icon_w + icon_w * GAP_FACTOR + maxf(pill.x, pill.y)))


## The pill sits on the right of the button face (the face is above the hard bottom edge of tier 1, F114).
func _place_badge() -> void:
	if _count == null or not _count.visible:
		return
	var box: StyleBox = get_theme_stylebox(&"normal")
	var right: float = box.get_margin(SIDE_RIGHT) if box != null else 0.0
	var top: float = box.get_margin(SIDE_TOP) if box != null else 0.0
	var bottom: float = box.get_margin(SIDE_BOTTOM) if box != null else 0.0
	var pill: Vector2 = _count.get_combined_minimum_size()
	var w: float = maxf(pill.x, pill.y)
	var face_mid: float = (top + size.y - bottom) * 0.5
	_count.size = Vector2(w, pill.y)
	_count.position = Vector2(size.x - right - w, face_mid - pill.y * 0.5)
