class_name UiScreenRegistry
extends RefCounted
## Screens that `EventBus.ui_requested(screen_id, args)` can open in the world (GODOT_ARCHITECTURE §6). Gameplay and
## services emit intents with a screen id; WorldRoot opens the registered scene or, when the id has no screen in this
## client yet, shows `ui.activity_player.update_required`
## (F062-AC-10). Features register their screen here when they ship it.

const SCREENS: Dictionary = {
	&"activity_host": "res://scenes/learning/ActivityHost.tscn",
	&"exam_lobby": "res://scenes/exam/ExamLobby.tscn",
	&"quest_journal": "res://scenes/quest/QuestJournal.tscn",
	&"settings": "res://scenes/ui/settings/SettingsPanel.tscn",
	&"wardrobe": "res://scenes/character/CustomizationStudio.tscn",
}


static func has(screen_id: StringName) -> bool:
	return SCREENS.has(screen_id)


static func path_for(screen_id: StringName) -> String:
	return String(SCREENS.get(screen_id, ""))
