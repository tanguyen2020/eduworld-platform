extends Node
## Autoload `ActivityRendererRegistry` (GODOT_ARCHITECTURE §6.2, ADR-0048, ACTIVITY_ENGINE §12): `activity_type` →
## renderer scene under scenes/learning/activity_renderers/. F063 registers `quiz` and `interactive_exercise`;
## F065 (`video_lesson`), F066 (`audio_lesson`, `simulation`) and F073 (`minigame`) add theirs. Before a renderer is
## opened, `check()` refuses an unknown type or a version newer than this client (`ActivityRender.min_client_version`,
## `ContentManifest.activity_min_client_versions`): the player shows "update required" instead (F063-AC-25).

const REASON_OK: String = ""
const REASON_UNKNOWN_TYPE: String = "unknown_type"
const REASON_CLIENT_TOO_OLD: String = "client_too_old"

var renderers: Dictionary = {
	"quiz": "res://scenes/learning/activity_renderers/quiz/QuizRenderer.tscn",
	"interactive_exercise":
			"res://scenes/learning/activity_renderers/interactive_exercise/InteractiveExerciseRenderer.tscn",
}


func register(activity_type: String, scene_path: String) -> void:
	renderers[activity_type] = scene_path


func unregister(activity_type: String) -> void:
	renderers.erase(activity_type)


func has_renderer(activity_type: String) -> bool:
	return renderers.has(activity_type) and ResourceLoader.exists(String(renderers[activity_type]))


func scene_path(activity_type: String) -> String:
	return String(renderers.get(activity_type, ""))


## REASON_OK when this client can open `activity_type` whose content needs `min_versions` (empty = no minimum).
func check(activity_type: String, client_version: String, min_versions: PackedStringArray = []) -> String:
	if not has_renderer(activity_type):
		return REASON_UNKNOWN_TYPE
	for minimum: String in min_versions:
		if not minimum.is_empty() and VersionCheck.compare_versions(client_version, minimum) < 0:
			return REASON_CLIENT_TOO_OLD
	return REASON_OK


## A new renderer for `activity_type`, null when none is registered.
func instantiate(activity_type: String) -> ActivityRendererBase:
	if not has_renderer(activity_type):
		return null
	return (load(scene_path(activity_type)) as PackedScene).instantiate() as ActivityRendererBase
