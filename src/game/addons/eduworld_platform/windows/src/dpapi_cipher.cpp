// EduWorld platform GDExtension — Windows DPAPI cipher for SecureStorage (ADR-0086, F060 T051).
#include "dpapi_cipher.h"

#include <godot_cpp/core/class_db.hpp>

#include <cstring>

#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#include <dpapi.h>
#endif

using namespace godot;

namespace eduworld {

namespace {
// Optional DPAPI entropy: separates EduWorld blobs from other applications' DPAPI blobs of the same user.
// It is not a secret (the protection comes from the user's DPAPI master key).
constexpr char kEntropy[] = "eduworld.securestorage.v1";
} // namespace

void EduWorldDpapiCipher::_bind_methods() {
	ClassDB::bind_method(D_METHOD("is_available"), &EduWorldDpapiCipher::is_available);
	ClassDB::bind_method(D_METHOD("protect", "plain"), &EduWorldDpapiCipher::protect);
	ClassDB::bind_method(D_METHOD("unprotect", "sealed"), &EduWorldDpapiCipher::unprotect);
}

bool EduWorldDpapiCipher::is_available() const {
#ifdef _WIN32
	return true;
#else
	return false;
#endif
}

PackedByteArray EduWorldDpapiCipher::protect(const PackedByteArray &p_plain) const {
	PackedByteArray out;
#ifdef _WIN32
	if (p_plain.is_empty()) {
		return out;
	}
	DATA_BLOB input{ static_cast<DWORD>(p_plain.size()), const_cast<BYTE *>(p_plain.ptr()) };
	DATA_BLOB entropy{ static_cast<DWORD>(sizeof(kEntropy) - 1), reinterpret_cast<BYTE *>(const_cast<char *>(kEntropy)) };
	DATA_BLOB sealed{ 0, nullptr };
	// Scope CurrentUser: no CRYPTPROTECT_LOCAL_MACHINE flag. No UI prompt.
	if (CryptProtectData(&input, nullptr, &entropy, nullptr, nullptr, CRYPTPROTECT_UI_FORBIDDEN, &sealed)) {
		out.resize(static_cast<int64_t>(sealed.cbData));
		std::memcpy(out.ptrw(), sealed.pbData, sealed.cbData);
		LocalFree(sealed.pbData);
	}
#endif
	return out;
}

PackedByteArray EduWorldDpapiCipher::unprotect(const PackedByteArray &p_sealed) const {
	PackedByteArray out;
#ifdef _WIN32
	if (p_sealed.is_empty()) {
		return out;
	}
	DATA_BLOB input{ static_cast<DWORD>(p_sealed.size()), const_cast<BYTE *>(p_sealed.ptr()) };
	DATA_BLOB entropy{ static_cast<DWORD>(sizeof(kEntropy) - 1), reinterpret_cast<BYTE *>(const_cast<char *>(kEntropy)) };
	DATA_BLOB plain{ 0, nullptr };
	if (CryptUnprotectData(&input, nullptr, &entropy, nullptr, nullptr, CRYPTPROTECT_UI_FORBIDDEN, &plain)) {
		out.resize(static_cast<int64_t>(plain.cbData));
		std::memcpy(out.ptrw(), plain.pbData, plain.cbData);
		SecureZeroMemory(plain.pbData, plain.cbData);
		LocalFree(plain.pbData);
	}
#endif
	return out;
}

} // namespace eduworld
