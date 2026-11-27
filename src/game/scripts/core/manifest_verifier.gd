class_name ManifestVerifier
extends RefCounted
## Signed release files of the CDN (F133 T023, F133-AC-05, spec Edge Cases, research R2, ADR-0044): the bundle index
## `index/<client_version>.json` and the release manifest `releases/<platform>/<channel>.json`, each with a detached
## base64 RSA-SHA256 (PKCS#1 v1.5) signature `<file>.sig` made by scripts/godot/sign-manifest.mjs. The public key is
## `Config.manifest_public_key` (written by gen-config.mjs). A file whose signature or shape is wrong is refused as a
## whole — the caller keeps what it has cached and reports `client_error`. Rules mirror
## specs/schemas/bundle-index.schema.json and client-release-manifest.schema.json. No Node, no scene (domain class).

const ERR_SIGNATURE: String = "MANIFEST_SIGNATURE_INVALID"
const ERR_KEY: String = "MANIFEST_KEY_INVALID"
const ERR_FORMAT: String = "MANIFEST_FORMAT_INVALID"

const MAX_BUNDLES: int = 500
const URL_MAX: int = 2048
const SIG_MAX: int = 2048
const READ_CHUNK: int = 1 << 20
const SEMVER: String = "^[0-9]+\\.[0-9]+\\.[0-9]+(-[0-9A-Za-z.-]+)?$"
const SHA256_HEX: String = "^[a-f0-9]{64}$"
const BUNDLE_ID: String = "^[a-z0-9][a-z0-9._-]{0,127}$"
const GRADE_BANDS: PackedStringArray = ["g1_3", "g4_5", "g6_7", "g8_9"]
const PLATFORMS: PackedStringArray = ["android", "ios", "web"]
const CHANNELS: PackedStringArray = ["staging", "production"]


static func sha256_bytes(bytes: PackedByteArray) -> PackedByteArray:
	var ctx: HashingContext = HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	ctx.update(bytes)
	return ctx.finish()


static func sha256_hex(bytes: PackedByteArray) -> String:
	return sha256_bytes(bytes).hex_encode()


## SHA-256 (hex) of a file, read in 1 MiB chunks (packs are up to 40 MB); empty when the file cannot be read.
static func file_sha256(path: String) -> String:
	var f: FileAccess = FileAccess.open(path, FileAccess.READ)
	if f == null:
		return ""
	var ctx: HashingContext = HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	while f.get_position() < f.get_length():
		ctx.update(f.get_buffer(READ_CHUNK))
	f.close()
	return ctx.finish().hex_encode()


static func matches_sha256(path: String, expected_hex: String) -> bool:
	var got: String = file_sha256(path)
	return not got.is_empty() and got == expected_hex.to_lower()


## True when `sig_b64` is a valid signature of `bytes` by the public key `public_pem`.
static func verify_signature(bytes: PackedByteArray, sig_b64: String, public_pem: String) -> Result:
	var key: CryptoKey = CryptoKey.new()
	if public_pem.is_empty() or key.load_from_string(public_pem, true) != OK:
		return Result.failure(ERR_KEY)
	var sig_text: String = sig_b64.strip_edges()
	if sig_text.is_empty() or sig_text.length() > SIG_MAX or not _is_base64(sig_text):
		return Result.failure(ERR_SIGNATURE)
	var sig: PackedByteArray = Marshalls.base64_to_raw(sig_text)
	if sig.is_empty() or not Crypto.new().verify(HashingContext.HASH_SHA256, sha256_bytes(bytes), sig, key):
		return Result.failure(ERR_SIGNATURE)
	return Result.success()


## Signed bundle index of `client_version`: value = the parsed index Dictionary.
static func verify_bundle_index(bytes: PackedByteArray, sig_b64: String, public_pem: String,
		client_version: String) -> Result:
	var signed: Result = verify_signature(bytes, sig_b64, public_pem)
	if not signed.ok:
		return signed
	var data: Variant = JSON.parse_string(bytes.get_string_from_utf8())
	if not data is Dictionary:
		return Result.failure(ERR_FORMAT, {"field": "$"})
	var index: Dictionary = data as Dictionary
	if int(index.get("schema_version", 0)) != 1 or String(index.get("client_version", "")) != client_version:
		return Result.failure(ERR_FORMAT, {"field": "client_version"})
	var bundles: Variant = index.get("bundles")
	if not bundles is Array or (bundles as Array).size() > MAX_BUNDLES:
		return Result.failure(ERR_FORMAT, {"field": "bundles"})
	var seen: Dictionary = {}
	for item: Variant in bundles as Array:
		var field: String = _bundle_problem(item)
		if field.is_empty() and seen.has(String((item as Dictionary)["bundle_id"])):
			field = "bundle_id"
		if not field.is_empty():
			return Result.failure(ERR_FORMAT, {"field": field})
		seen[String((item as Dictionary)["bundle_id"])] = true
	return Result.success(index)


## Signed release manifest of `platform`/`channel`: value = the parsed manifest Dictionary.
static func verify_release_manifest(bytes: PackedByteArray, sig_b64: String, public_pem: String, platform: String,
		channel: String) -> Result:
	var signed: Result = verify_signature(bytes, sig_b64, public_pem)
	if not signed.ok:
		return signed
	var data: Variant = JSON.parse_string(bytes.get_string_from_utf8())
	if not data is Dictionary:
		return Result.failure(ERR_FORMAT, {"field": "$"})
	var m: Dictionary = data as Dictionary
	if int(m.get("schema_version", 0)) != 1:
		return Result.failure(ERR_FORMAT, {"field": "schema_version"})
	if String(m.get("platform", "")) != platform or not platform in PLATFORMS:
		return Result.failure(ERR_FORMAT, {"field": "platform"})
	if String(m.get("channel", "")) != channel or not channel in CHANNELS:
		return Result.failure(ERR_FORMAT, {"field": "channel"})
	if not _matches(SEMVER, m.get("client_version")) or String(m["client_version"]).length() > 32:
		return Result.failure(ERR_FORMAT, {"field": "client_version"})
	if not _is_https(m.get("url")):
		return Result.failure(ERR_FORMAT, {"field": "url"})
	if not _matches(SHA256_HEX, m.get("sha256")) or not _is_count(m.get("size_bytes")) or not _is_count(m.get("build")):
		return Result.failure(ERR_FORMAT, {"field": "sha256"})
	return Result.success(m)


## Name of the first field of a bundle entry that breaks the schema, empty when the entry is valid.
static func _bundle_problem(item: Variant) -> String:
	if not item is Dictionary:
		return "bundles"
	var b: Dictionary = item as Dictionary
	if not _matches(BUNDLE_ID, b.get("bundle_id")):
		return "bundle_id"
	if not _is_count(b.get("version")) or int(b["version"]) < 1:
		return "version"
	if not _matches(SHA256_HEX, b.get("sha256")):
		return "sha256"
	if not _is_count(b.get("size_bytes")):
		return "size_bytes"
	var url: Variant = b.get("url")
	if not _is_https(url) or (url as String).contains("?") or (url as String).contains("#") \
			or not (url as String).ends_with("/bundles/%s.pck" % String(b["sha256"])):
		return "url"
	var bands: Variant = b.get("grade_bands")
	if not bands is Array or (bands as Array).is_empty() or (bands as Array).size() > GRADE_BANDS.size():
		return "grade_bands"
	for band: Variant in bands as Array:
		if not String(band) in GRADE_BANDS:
			return "grade_bands"
	return ""


static func _matches(pattern: String, value: Variant) -> bool:
	return value is String and RegEx.create_from_string(pattern).search(value as String) != null


static func _is_https(value: Variant) -> bool:
	return value is String and (value as String).begins_with("https://") and (value as String).length() <= URL_MAX


## JSON numbers arrive as float: a whole number ≥ 0.
static func _is_count(value: Variant) -> bool:
	if value is int:
		return int(value) >= 0
	return value is float and float(value) >= 0.0 and is_equal_approx(float(value), floorf(float(value)))


static func _is_base64(text: String) -> bool:
	return RegEx.create_from_string("^[A-Za-z0-9+/]+={0,2}$").search(text) != null
