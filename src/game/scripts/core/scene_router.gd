extends Node
## Autoload #9 `SceneRouter` (GODOT_ARCHITECTURE §4): scene changes with threaded loading. It reports progress
## through signals so a loading screen can render it; it knows nothing about quests or business rules.

signal scene_load_started(path: String)
signal scene_load_progressed(path: String, progress: float)
signal scene_changed(path: String)
signal scene_load_failed(path: String)

var current_path: String = ""
var _pending_path: String = ""
var _progress: Array = []


func _ready() -> void:
	set_process(false)


func is_loading() -> bool:
	return not _pending_path.is_empty()


## Starts a threaded load of `path`; the scene is switched when loading completes.
func goto(path: String) -> Error:
	if is_loading():
		return ERR_BUSY
	if not ResourceLoader.exists(path):
		scene_load_failed.emit(path)
		return ERR_FILE_NOT_FOUND
	var err: Error = ResourceLoader.load_threaded_request(path, "PackedScene")
	if err != OK:
		scene_load_failed.emit(path)
		return err
	_pending_path = path
	scene_load_started.emit(path)
	set_process(true)
	return OK


func _process(_delta: float) -> void:
	if _pending_path.is_empty():
		set_process(false)
		return
	var status: ResourceLoader.ThreadLoadStatus = ResourceLoader.load_threaded_get_status(_pending_path, _progress)
	match status:
		ResourceLoader.THREAD_LOAD_IN_PROGRESS:
			if not _progress.is_empty():
				scene_load_progressed.emit(_pending_path, float(_progress[0]))
		ResourceLoader.THREAD_LOAD_LOADED:
			var path: String = _pending_path
			_pending_path = ""
			set_process(false)
			var scene: PackedScene = ResourceLoader.load_threaded_get(path) as PackedScene
			if scene == null or get_tree().change_scene_to_packed(scene) != OK:
				scene_load_failed.emit(path)
				return
			current_path = path
			scene_changed.emit(path)
		_:
			var failed: String = _pending_path
			_pending_path = ""
			set_process(false)
			scene_load_failed.emit(failed)
