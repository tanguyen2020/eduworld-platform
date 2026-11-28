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
## F109: `arena` (Đấu trường: HUD "Thêm" / K / NPC Trọng tài Sấm — args `{tab, tournament_id, duel}`), `leaderboard`
## (the same screen on its "Bảng xếp hạng" tab — NPC `open_ui`), `battle` (the match screen — args `{match_id, mode}`).
## F110: `friends` (Bạn bè: HUD "Thêm" / F / NPC Chú Ong Kết Nối / a friend request notification — args `{tab}`),
## `boss_info` (the boss card of a gate / NPC `open_ui` — args `{area_id}` or `{boss_id}`), `boss_arena` (the boss
## screen — args `{encounter_id, mode, visual_ref}`), `coop_step` (the roles + clue cards of an `interdependent` step —
## args `{quest_id, step_id}`).
## F111: `event_center` (Sự kiện: HUD calendar button / "Thêm" / the event board of Quảng trường Tia Sáng / an
## `open_event` notification — args `{tab, id}`); the club tab is `friends` with `{tab: "clubs", id}`.
## F112: `chat` (the HUD chat of the persistent channels: HUD chat button / `Enter` / `T` / "Nhắn tin" of a friend —
## args `{tab}`, `{dm: user_id}` or `{channel_id}`).

const SCREENS: Dictionary = {
	&"achievements": "res://scenes/achievement/AchievementCollectionScreen.tscn",
	&"activity_host": "res://scenes/learning/ActivityHost.tscn",
	&"arena": "res://scenes/competition/ArenaScreen.tscn",
	&"battle": "res://scenes/competition/BattleArena.tscn",
	&"boss_arena": "res://scenes/challenge/boss/BossArena.tscn",
	&"boss_info": "res://scenes/challenge/boss/BossInfoCard.tscn",
	&"coop_step": "res://scenes/quest/InterdependentStepPanel.tscn",
	&"event_center": "res://scenes/liveops/EventCenterScreen.tscn",
	&"friends": "res://scenes/social/FriendsScreen.tscn",
	&"chat": "res://scenes/chat/ChatHub.tscn",
	&"challenge_board": "res://scenes/challenge/ChallengeBoard.tscn",
	&"challenge_result": "res://scenes/challenge/ChallengeResult.tscn",
	&"exam_lobby": "res://scenes/exam/ExamLobby.tscn",
	&"game_menu": "res://scenes/ui/hud/GameMenu.tscn",
	&"inventory": "res://scenes/inventory/InventoryShopScreen.tscn",
	&"journey": "res://scenes/progression/JourneyScreen.tscn",
	&"leaderboard": "res://scenes/competition/LeaderboardScreen.tscn",
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
