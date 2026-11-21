class_name DialogueActions
extends RefCounted
## Turns the actions of an NPC dialogue into server calls and UI intents (F071-AC-19, AGENTS R1): quest actions go to
## the quest API (F072 — `quest_api`, absent in this client → the dialogue shows the fallback line, spec edge case),
## learning actions open the activity host / mini-game screen (F063/F073, which call their own APIs), `open_ui` opens a
## registered screen, `play_emote` plays an emote locally. It returns what the server/screen answered and never
## changes quest, reward or unlock state by itself (ADR-0016).

signal notice_requested(key: String)
signal emote_requested(emote_id: String)
signal activity_opened(screen_id: StringName)

const CODE_UNAVAILABLE: String = "QUEST_API_UNAVAILABLE"
const NOTICE_UNAVAILABLE: String = "ui.world.dialogue.unavailable"
const UI_SCREENS: Dictionary = {"quest_journal": &"quest_journal", "practice_board": &"practice_board",
	"leaderboard": &"leaderboard", "wardrobe": &"wardrobe"}
const SCREEN_ACTIVITY: StringName = &"activity_host"
const SCREEN_MINIGAME: StringName = &"minigame_host"

## `(action: Dictionary) -> ApiResult` coroutine of the quest service (F072); invalid = not available in this client.
var quest_api: Callable = Callable()
## `(screen_id: StringName, args: Dictionary) -> void`; default `EventBus.ui_requested`.
var open_screen: Callable = Callable()


func handle(action: Dictionary) -> Variant:
	var type: String = String(action.get("type", ""))
	match type:
		"give_quest", "complete_step":
			if not quest_api.is_valid():
				notice_requested.emit(NOTICE_UNAVAILABLE)
				return ApiResult.failure(CODE_UNAVAILABLE)
			var result: ApiResult = await quest_api.call(action)
			if not result.ok:
				notice_requested.emit(NOTICE_UNAVAILABLE)
			return result
		"open_activity":
			_open(SCREEN_ACTIVITY, {"quest_id": action.get("quest_id", ""), "step_id": action.get("step_id", ""), "mode": "quest"})
			activity_opened.emit(SCREEN_ACTIVITY)
		"open_minigame":
			_open(SCREEN_MINIGAME, {"minigame_config_id": action.get("minigame_config_id", "")})
			activity_opened.emit(SCREEN_MINIGAME)
		"open_ui":
			var screen: StringName = UI_SCREENS.get(String(action.get("ui_screen", "")), &"") as StringName
			if not screen.is_empty():
				_open(screen, {})
		"play_emote":
			emote_requested.emit(String(action.get("emote_id", "")))
	return null


func _open(screen: StringName, args: Dictionary) -> void:
	if open_screen.is_valid():
		open_screen.call(screen, args)
	else:
		EventBus.ui_requested.emit(screen, args)
