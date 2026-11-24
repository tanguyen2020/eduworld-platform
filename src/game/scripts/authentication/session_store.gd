class_name SessionStore
extends RefCounted
## The stored game session `eduworld.game.session` = JSON `{refresh_token, tenant_id, saved_at}` kept through
## `SecureStorage` (F060; Android Keystore on a personal device, MemoryOnly on Web and `shared_device`) —
## ADR-0086, GODOT_ARCHITECTURE §5.2. Never holds the access token, the password or the bootstrap.
## A write failure is logged (`secure_storage_write_failed`, no value) and the session continues in RAM; an
## unreadable or malformed record is erased (F060-AC-26).

const KEY: String = "eduworld.game.session"
const TELEMETRY_WRITE_FAILED: String = "secure_storage_write_failed"

var storage: SecureStorage = null


func _init(secure_storage: SecureStorage) -> void:
	storage = secure_storage


## True when the record survives the process (personal device with a native store).
func is_persistent() -> bool:
	return storage != null and not storage.is_memory_only()


func save(refresh_token: String, tenant_id: String) -> bool:
	if storage == null or refresh_token.is_empty() or tenant_id.is_empty():
		return false
	var record: Dictionary = {
		"refresh_token": refresh_token,
		"tenant_id": tenant_id,
		"saved_at": Time.get_datetime_string_from_system(true) + "Z",
	}
	var err: Error = storage.save(KEY, JSON.stringify(record).to_utf8_buffer())
	if err != OK:
		_report_write_failure(err)
		return false
	return true


## The stored record, or an empty Dictionary (none, unreadable or malformed — the latter two are erased).
func load_record() -> Dictionary:
	if storage == null:
		return {}
	var bytes: PackedByteArray = storage.load(KEY)
	if bytes.is_empty():
		if storage.last_error_code != SecureStorage.CODE_NOT_FOUND and storage.last_error_code != SecureStorage.CODE_OK:
			storage.erase(KEY)
		return {}
	var parsed: Variant = JSON.parse_string(bytes.get_string_from_utf8())
	bytes.fill(0)
	if not parsed is Dictionary:
		storage.erase(KEY)
		return {}
	var record: Dictionary = parsed as Dictionary
	var token: Variant = record.get("refresh_token")
	var tenant: Variant = record.get("tenant_id")
	if not (token is String and tenant is String) or (token as String).is_empty() or not Uuid.is_valid(tenant as String):
		storage.erase(KEY)
		return {}
	return record


func has_record() -> bool:
	return not load_record().is_empty()


func erase() -> void:
	if storage != null:
		storage.erase(KEY)


func _report_write_failure(err: Error) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	if tree == null:
		return
	var log_node: Node = tree.root.get_node_or_null("Log")
	if log_node != null:
		log_node.call("warn", TELEMETRY_WRITE_FAILED, {"adapter": storage.adapter_name(), "error": err})
	var app_state: Node = tree.root.get_node_or_null("AppState")
	if app_state != null:
		app_state.call("queue_telemetry", TELEMETRY_WRITE_FAILED, {"adapter": storage.adapter_name()})
