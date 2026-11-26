extends Button
## Bell of the HUD (STUDENT_GAME_UX_CORE S03 `NotificationBell`, F064-AC-20, F064-AC-22): the bell icon of
## design/icons/game (F096-AC-15: no emoji, the Web build has no emoji font) with the badge count of NotificationService
## (new school work seen in this session) beside it. Pressing it is an intent handled by the HUD owner. UI only.


func set_count(count: int) -> void:
	text = tr("ui.notification.bell_count").format({"count": count}) if count > 0 else ""
