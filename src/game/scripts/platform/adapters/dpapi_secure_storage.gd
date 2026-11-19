class_name DpapiSecureStorage
extends FileCipherSecureStorage
## Windows adapter (ADR-0086): DPAPI `CryptProtectData` / `CryptUnprotectData`, scope CurrentUser (never
## LOCAL_MACHINE), through the GDExtension class `EduWorldDpapiCipher` of addons/eduworld_platform/windows.
## Ciphertext goes to `user://secure/<key>.bin`; another Windows user or a copied folder cannot decrypt it.
## When the GDExtension is not installed (not built for this machine), `is_available()` is false and
## SecureStorageFactory falls back to MemoryOnly — no plaintext fallback exists.

const NATIVE_CLASS: StringName = &"EduWorldDpapiCipher"


func _init(dir: String = DEFAULT_DIR) -> void:
	super(native_cipher(), dir)


func adapter_name() -> String:
	return "dpapi"


static func native_cipher() -> Object:
	if ClassDB.class_exists(NATIVE_CLASS) and ClassDB.can_instantiate(NATIVE_CLASS):
		return ClassDB.instantiate(NATIVE_CLASS)
	return null
