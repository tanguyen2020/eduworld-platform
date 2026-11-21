extends Button
## 🔔 of the HUD (STUDENT_GAME_UX_CORE S03 `NotificationBell`, F064-AC-20, F064-AC-22): shows the badge of
## NotificationService (new school work seen in this session). Pressing it is an intent handled by the HUD owner. UI only.


func set_count(count: int) -> void:
	text = tr("ui.notification.bell_count").format({"count": count}) if count > 0 else tr("ui.notification.bell")
