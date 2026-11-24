class_name SecureStorageFactory
extends RefCounted
## Chooses the SecureStorage adapter once at start-up (ADR-0086, ADR-0101, GODOT_ARCHITECTURE §5.2, F060-AC-23,
## F060-AC-24): shared device or Web → MemoryOnly; Android → Android Keystore; iOS until F133, operating systems
## that are not released (Windows, macOS, Linux when run from the editor or a debug build — ADR-0101) or an
## unavailable native store → MemoryOnly. On a shared device it also removes any file record left in
## `user://secure` by an earlier file-backed adapter.


## Adapter for this process, from Config (`shared_device`) and the running platform.
static func create_default() -> SecureStorage:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var shared: bool = true
	if tree != null and tree.root.has_node("Config"):
		shared = bool(tree.root.get_node("Config").get("shared_device"))
	return create(shared, platform_key())


## `platform` uses the Config keys: android, ios, web, other.
static func create(shared_device: bool, platform: String) -> SecureStorage:
	if shared_device or platform == "web":
		purge_file_records(FileCipherSecureStorage.DEFAULT_DIR)
		if platform == "android":
			AndroidKeystoreSecureStorage.new().erase_all()
		return MemoryOnlySecureStorage.new()
	var native: SecureStorage = null
	if platform == "android":
		native = AndroidKeystoreSecureStorage.new()
	if native != null and native.is_available():
		return native
	return MemoryOnlySecureStorage.new()


static func platform_key() -> String:
	if OS.has_feature("web"):
		return "web"
	match OS.get_name():
		"Android":
			return "android"
		"iOS":
			return "ios"
	return "other"


## Removes file records of file-backed adapters (best effort; the files only ever hold ciphertext).
static func purge_file_records(dir_path: String) -> int:
	var dir: DirAccess = DirAccess.open(dir_path)
	if dir == null:
		return 0
	var removed: int = 0
	for file_name: String in dir.get_files():
		if file_name.ends_with(FileCipherSecureStorage.FILE_EXTENSION) \
				or file_name.ends_with(FileCipherSecureStorage.TEMP_SUFFIX):
			if dir.remove(file_name) == OK:
				removed += 1
	return removed
