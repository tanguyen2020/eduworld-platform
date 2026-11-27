extends GdUnitTestSuite
## F133 T015 (F133-AC-05, spec Edge Cases, research R2): ManifestVerifier accepts the bundle index and the release
## manifest signed by scripts/godot/sign-manifest.mjs (RSA-3072, PKCS#1 v1.5 SHA-256, base64 `.sig`) and refuses a
## changed byte, another key, a malformed signature, a manifest of another platform/channel and an index whose entries
## break the schema rules; SHA-256 of files is streamed. Fixtures: res://tests/fixtures/release (make_fixtures.mjs).

const DIR: String = "res://tests/fixtures/release"


func _bytes(file_name: String) -> PackedByteArray:
	return FileAccess.get_file_as_bytes(DIR.path_join(file_name))


func _text(file_name: String) -> String:
	return FileAccess.get_file_as_string(DIR.path_join(file_name))


func _pem() -> String:
	return _text("release_public.pem")


func test_node_signed_bundle_index_verifies_and_parses() -> void:
	var r: Result = ManifestVerifier.verify_bundle_index(_bytes("bundle_index.json"), _text("bundle_index.json.sig"),
			_pem(), "1.2.3")
	assert_bool(r.ok).override_failure_message(r.code).is_true()
	var index: Dictionary = r.value as Dictionary
	assert_int((index["bundles"] as Array).size()).is_equal(2)
	assert_str(String(((index["bundles"] as Array)[0] as Dictionary)["bundle_id"])).is_equal("zone.math_g3")


func test_one_changed_byte_another_key_or_bad_base64_is_refused() -> void:
	var bytes: PackedByteArray = _bytes("bundle_index.json")
	bytes[10] = bytes[10] ^ 1
	assert_str(ManifestVerifier.verify_bundle_index(bytes, _text("bundle_index.json.sig"), _pem(), "1.2.3").code) \
			.is_equal(ManifestVerifier.ERR_SIGNATURE)
	var other: CryptoKey = Crypto.new().generate_rsa(2048)
	assert_str(ManifestVerifier.verify_bundle_index(_bytes("bundle_index.json"), _text("bundle_index.json.sig"),
			other.save_to_string(true), "1.2.3").code).is_equal(ManifestVerifier.ERR_SIGNATURE)
	assert_str(ManifestVerifier.verify_bundle_index(_bytes("bundle_index.json"), "%%%not-base64", _pem(), "1.2.3").code) \
			.is_equal(ManifestVerifier.ERR_SIGNATURE)
	assert_str(ManifestVerifier.verify_bundle_index(_bytes("bundle_index.json"), _text("bundle_index.json.sig"),
			"-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n", "1.2.3").code).is_equal(ManifestVerifier.ERR_KEY)


func test_index_of_another_client_version_is_refused() -> void:
	assert_str(ManifestVerifier.verify_bundle_index(_bytes("bundle_index.json"), _text("bundle_index.json.sig"), _pem(),
			"1.2.4").code).is_equal(ManifestVerifier.ERR_FORMAT)


func test_release_manifest_verifies_for_its_platform_and_channel_only() -> void:
	var ok: Result = ManifestVerifier.verify_release_manifest(_bytes("release_manifest.json"),
			_text("release_manifest.json.sig"), _pem(), "android", "production")
	assert_bool(ok.ok).override_failure_message(ok.code).is_true()
	assert_str(String((ok.value as Dictionary)["client_version"])).is_equal("1.3.0")
	assert_str(ManifestVerifier.verify_release_manifest(_bytes("release_manifest.json"),
			_text("release_manifest.json.sig"), _pem(), "ios", "production").code).is_equal(ManifestVerifier.ERR_FORMAT)


## Entries that break the schema (signed with a throwaway key, so only the shape is wrong).
func test_index_entries_must_follow_the_schema() -> void:
	var key: CryptoKey = Crypto.new().generate_rsa(2048)
	var good_sha: String = "a".repeat(64)
	var entry: Dictionary = {"bundle_id": "zone.x", "version": 1, "sha256": good_sha, "size_bytes": 10,
		"url": "https://b.example/bundles/%s.pck" % good_sha, "grade_bands": ["g1_3"]}
	var cases: Array[Dictionary] = [
		{"url": "https://b.example/bundles/%s.pck" % "b".repeat(64)},
		{"url": "http://b.example/bundles/%s.pck" % good_sha},
		{"url": "https://b.example/bundles/%s.pck?sig=x" % good_sha},
		{"sha256": "XYZ"},
		{"bundle_id": "Zone X"},
		{"grade_bands": ["g10"]},
		{"size_bytes": -1},
	]
	assert_bool(_signed_index(key, [entry]).ok).is_true()
	for change: Dictionary in cases:
		var bad: Dictionary = entry.duplicate(true)
		bad.merge(change, true)
		assert_str(_signed_index(key, [bad]).code).override_failure_message(str(change)) \
				.is_equal(ManifestVerifier.ERR_FORMAT)


func test_sha256_of_a_file_is_streamed_and_compared() -> void:
	var pack: String = DIR.path_join("5d63107c5862ced567a532fb8bfca97c50e2d69c1dcbdcd4eaacfdabe599857e.pck")
	var hex: String = ManifestVerifier.file_sha256(pack)
	assert_str(hex).is_equal(ManifestVerifier.sha256_hex(FileAccess.get_file_as_bytes(pack)))
	assert_bool(ManifestVerifier.matches_sha256(pack, hex)).is_true()
	assert_bool(ManifestVerifier.matches_sha256(pack, "0".repeat(64))).is_false()
	assert_bool(ManifestVerifier.matches_sha256("user://missing.pck", hex)).is_false()


func _signed_index(key: CryptoKey, bundles: Array) -> Result:
	var bytes: PackedByteArray = JSON.stringify({"schema_version": 1, "client_version": "1.0.0",
		"generated_at": "2026-11-28T02:00:00Z", "bundles": bundles}).to_utf8_buffer()
	var crypto: Crypto = Crypto.new()
	var sig: PackedByteArray = crypto.sign(HashingContext.HASH_SHA256, ManifestVerifier.sha256_bytes(bytes), key)
	return ManifestVerifier.verify_bundle_index(bytes, Marshalls.raw_to_base64(sig), key.save_to_string(true), "1.0.0")
