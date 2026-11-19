// EduWorld platform GDExtension — Windows DPAPI cipher for SecureStorage (ADR-0086, F060 T051).
#pragma once

#include <godot_cpp/classes/ref_counted.hpp>
#include <godot_cpp/variant/packed_byte_array.hpp>

namespace eduworld {

// Seals bytes with DPAPI scope CurrentUser (CryptProtectData without CRYPTPROTECT_LOCAL_MACHINE): only the same
// Windows user on the same machine can unseal them. Used by scripts/platform/adapters/dpapi_secure_storage.gd,
// which writes the sealed bytes to user://secure/<key>.bin. Never logs or keeps the plaintext.
class EduWorldDpapiCipher : public godot::RefCounted {
	GDCLASS(EduWorldDpapiCipher, godot::RefCounted)

protected:
	static void _bind_methods();

public:
	bool is_available() const;
	// Returns the sealed blob, or an empty array on failure.
	godot::PackedByteArray protect(const godot::PackedByteArray &p_plain) const;
	// Returns the plaintext, or an empty array when the blob is corrupt or belongs to another user/machine.
	godot::PackedByteArray unprotect(const godot::PackedByteArray &p_sealed) const;
};

} // namespace eduworld
