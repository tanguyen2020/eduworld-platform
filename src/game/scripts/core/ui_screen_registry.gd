class_name UiScreenRegistry
extends RefCounted
## Screens that `EventBus.ui_requested(screen_id, args)` can open in the world (GODOT_ARCHITECTURE §6). Gameplay and
## services emit intents with a screen id; WorldRoot opens the registered scene or, when the id has no screen in this
## client (e.g. `activity_host` before F063, `exam_lobby` before F064), shows `ui.activity_player.update_required`
## (F062-AC-10). Features register their screen here when they ship it.

const SCREENS: Dictionary = {
	&"quest_journal": "res://scenes/quest/QuestJournal.tscn",
	&"settings": "res://scenes/ui/settings/SettingsPanel.tscn",
}


static func has(screen_id: StringName) -> bool:
	return SCREENS.has(screen_id)


static func path_for(screen_id: StringName) -> String:
	return String(SCREENS.get(screen_id, ""))
