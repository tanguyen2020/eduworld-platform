extends RefCounted
## Test double of an OS cipher for FileCipherSecureStorage (F060 US5). NOT a security mechanism: it only makes
## sure the plaintext bytes never appear as-is on disk and that tampered records are detected.

const MAGIC: PackedByteArray = [0x45, 0x57, 0x46, 0x43]
const MASK: int = 0x5A

var available: bool = true
var fail_protect: bool = false


func is_available() -> bool:
	return available


func protect(plain: PackedByteArray) -> PackedByteArray:
	if fail_protect or plain.is_empty():
		return PackedByteArray()
	var out: PackedByteArray = MAGIC.duplicate()
	var body: PackedByteArray = plain.duplicate()
	body.reverse()
	for b: int in body:
		out.append(b ^ MASK)
	out.append(_checksum(plain))
	return out


func unprotect(sealed: PackedByteArray) -> PackedByteArray:
	if sealed.size() <= MAGIC.size() + 1 or sealed.slice(0, MAGIC.size()) != MAGIC:
		return PackedByteArray()
	var plain: PackedByteArray = PackedByteArray()
	for i: int in range(sealed.size() - 2, MAGIC.size() - 1, -1):
		plain.append(sealed[i] ^ MASK)
	if _checksum(plain) != sealed[sealed.size() - 1]:
		return PackedByteArray()
	return plain


static func _checksum(data: PackedByteArray) -> int:
	var sum: int = 0
	for b: int in data:
		sum = (sum * 31 + b) % 251
	return sum
