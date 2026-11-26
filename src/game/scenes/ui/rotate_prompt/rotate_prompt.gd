extends Control
## Rotate prompt (F113; STUDENT_GAME_UX §2 landscape only): a full-screen view shown by OrientationGuard while the
## window is taller than wide — the S01 sky, the rotate-phone icon and Mây saying "Xoay ngang điện thoại để chơi nhé!"
## with a hint and a line for computers. View only (AGENTS R1): showing, hiding, pausing and the Web lock belong to
## OrientationGuard. Every touch on it is swallowed so nothing reaches the paused game behind. Sizes from tokens,
## texts as translation keys (auto-translated, AGENTS R32).

## The phone icon is the picture of the screen: five icon sizes (UiTokens.COMPONENT_ICON_SIZE); its SVG is imported at
## 6× (`svg/scale` in rotate_phone.svg.import) so it stays sharp at that size on a phone.
const PHONE_ICON_SCALE: float = 5.0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_STOP
	var phone: TextureRect = %Phone as TextureRect
	phone.texture = UiBuild.icon("rotate_phone")
	var side: float = float(UiTokens.COMPONENT_ICON_SIZE) * PHONE_ICON_SCALE
	phone.custom_minimum_size = Vector2(side, side)
