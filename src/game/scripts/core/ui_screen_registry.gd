class_name UiScreenRegistry
extends RefCounted
## Screens that `EventBus.ui_requested(screen_id, args)` can open in the world (GODOT_ARCHITECTURE §6). Gameplay and
## services emit intents with a screen id; WorldRoot opens the registered scene or, when the id has no screen in this
## client yet, shows `ui.activity_player.update_required`
## (F062-AC-10). Features register their screen here when they ship it.
## F069: `inventory` (S08 Tủ đồ · Cửa hàng · Lịch sử; HUD Tủ đồ / I / B / Xu chip), `achievements` (S09), `journey`
## ("Hành trình của tôi"), `challenge_board` / `challenge_result` (S06 solo), `game_menu` (HUD "Thêm"). `wardrobe`
## stays the F070 CustomizationStudio ("Phối đồ" in S08). F123: `notification_inbox` (HUD bell, a toast without a screen).
## F102: `party` (Party Lobby: HUD "Nhóm" / P, the pending invitations, the session countdown and summary; also the
## target of a `party_invitation` notification — args `{view: "invitations", id}`).

const SCREENS: Dictionary = {
	&"achievements": "res://scenes/achievement/AchievementCollectionScreen.tscn",
	&"activity_host": "res://scenes/learning/ActivityHost.tscn",
	&"challenge_board": "res://scenes/challenge/ChallengeBoard.tscn",
	&"challenge_result": "res://scenes/challenge/ChallengeResult.tscn",
	&"exam_lobby": "res://scenes/exam/ExamLobby.tscn",
	&"game_menu": "res://scenes/ui/hud/GameMenu.tscn",
	&"inventory": "res://scenes/inventory/InventoryShopScreen.tscn",
	&"journey": "res://scenes/progression/JourneyScreen.tscn",
	&"minigame_host": "res://scenes/minigames/MiniGameHost.tscn",
	&"notification_inbox": "res://scenes/ui/notifications/NotificationInbox.tscn",
	&"party": "res://scenes/social/PartyLobby.tscn",
	&"npc_dialogue": "res://scenes/npc/DialogueBubble.tscn",
	&"quest_journal": "res://scenes/quest/QuestJournal.tscn",
	&"settings": "res://scenes/ui/settings/SettingsPanel.tscn",
	&"wardrobe": "res://scenes/character/CustomizationStudio.tscn",
	&"world_map": "res://scenes/world/WorldMap.tscn",
}


static func has(screen_id: StringName) -> bool:
	return SCREENS.has(screen_id)


static func path_for(screen_id: StringName) -> String:
	return String(SCREENS.get(screen_id, ""))
