extends GdUnitTestSuite
## Contract of client telemetry (`postTelemetry`, `TelemetryBatch` / `TelemetryEvent` of specs/openapi/eduworld-api.v1.yaml;
## GAME_ANALYTICS §3.2): the batch TelemetryService really sends with `chat_seq_gap` and `content_download` validates
## against the generated `TelemetryBatch` fixture (closed property set, value kinds — `additionalProperties: false`)
## and against the enums and ranges of the spec (`type`, `ChannelType`, `source`, `gap_count` 1–1 000, `bytes`
## 0–10 GiB). The client's own lists (types, fields, sources, channel types) stay equal to the spec.

const FIXTURES: String = "res://tests/fixtures/contract"
const SPEC_RELATIVE: String = "../../specs/openapi/eduworld-api.v1.yaml"
const ROUTE: String = "POST /game/telemetry"
const COMMON: Array[String] = ["type", "client_event_id", "occurred_at"]

var h: AuthHarness = null
var telemetry: TelemetryService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	telemetry = TelemetryService.new(h.api, h.clock)


func after_test() -> void:
	h.dispose()


func _json(text: String) -> Variant:
	var parser: JSON = JSON.new()
	assert_int(parser.parse(text)).override_failure_message(parser.get_error_message()).is_equal(OK)
	return parser.data


func _fixture() -> Dictionary:
	return _json(FileAccess.get_file_as_string(FIXTURES.path_join("openapi/TelemetryBatch.json"))) as Dictionary


func _spec() -> String:
	var spec_path: String = ProjectSettings.globalize_path("res://").path_join(SPEC_RELATIVE).simplify_path()
	assert_bool(FileAccess.file_exists(spec_path)).override_failure_message(spec_path).is_true()
	return FileAccess.get_file_as_string(spec_path)


## The text of component schema `name` (from its line to the next component at the same indent).
func _schema_block(spec: String, name: String) -> String:
	var start: int = spec.find("\n    %s:\n" % name)
	assert_int(start).override_failure_message(name).is_greater(0)
	var re: RegEx = RegEx.create_from_string("\\n    [A-Za-z0-9_]+:\\n")
	var next: RegExMatch = re.search(spec, start + 1)
	return spec.substr(start, (next.get_start() - start) if next != null else -1)


## The values of the first `enum: [...]` on the line of `key` inside `block` (or after it).
func _enum(block: String, key: String) -> PackedStringArray:
	var at: int = block.find(key)
	assert_int(at).override_failure_message(key).is_greater_equal(0)
	var open: int = block.find("enum: [", at)
	var close: int = block.find("]", open)
	var out: PackedStringArray = []
	for part: String in block.substr(open + 7, close - open - 7).split(","):
		out.append(part.strip_edges())
	return out


## {minimum, maximum} of the property line `key: { … }` in `block`.
func _range(block: String, key: String) -> Dictionary:
	var re: RegEx = RegEx.create_from_string("\\n\\s+%s: \\{[^\\n]*minimum: (\\d+), maximum: (\\d+)" % key)
	var m: RegExMatch = re.search(block)
	assert_object(m).override_failure_message(key).is_not_null()
	return {"minimum": int(m.get_string(1)), "maximum": int(m.get_string(2))}


func _sent_batch() -> Dictionary:
	telemetry.queue_chat_seq_gap("direct", 4)
	telemetry.queue_chat_seq_gap("match", 5000)
	for source: String in TelemetryService.DOWNLOAD_SOURCES:
		telemetry.queue_content_download(source, 2048)
	telemetry.queue_content_download("cdn", TelemetryService.DOWNLOAD_BYTES_MAX + 1)
	h.http.reply(ROUTE, 202)
	var result: ApiResult = await telemetry.flush(true)
	assert_bool(result.ok).is_true()
	return _json(h.http.last(ROUTE).raw_body) as Dictionary


func _same_kind(value: Variant, sample: Variant) -> bool:
	var numeric: Array[int] = [TYPE_INT, TYPE_FLOAT]
	if typeof(sample) in numeric:
		return typeof(value) in numeric
	return typeof(value) == typeof(sample)


func test_sent_batch_matches_the_generated_telemetry_batch_fixture() -> void:
	var fixture: Dictionary = _fixture()
	var sample_event: Dictionary = (fixture["events"] as Array)[0] as Dictionary
	var batch: Dictionary = await _sent_batch()
	for key: Variant in batch.keys():
		assert_bool(fixture.has(key)).override_failure_message("batch.%s" % key).is_true()
		assert_bool(_same_kind(batch[key], fixture[key])).override_failure_message("batch.%s" % key).is_true()
	var events: Array = batch["events"] as Array
	assert_int(events.size()).is_equal(6)
	for raw: Variant in events:
		var e: Dictionary = raw as Dictionary
		for key: Variant in e.keys():
			assert_bool(sample_event.has(key)).override_failure_message("event.%s" % key).is_true()
			assert_bool(_same_kind(e[key], sample_event[key])).override_failure_message("event.%s" % key).is_true()
		for key: String in COMMON:
			assert_bool(e.has(key)).override_failure_message(key).is_true()
		var expected: Array[String] = COMMON.duplicate()
		if e["type"] == TelemetryService.TYPE_CHAT_SEQ_GAP:
			expected.append_array(["channel_type", "gap_count"])
		else:
			expected.append_array(["source", "bytes"])
		assert_array(e.keys()).contains_exactly_in_any_order(expected)


func test_sent_values_are_inside_the_spec_enums_and_ranges() -> void:
	var spec: String = _spec()
	var event_block: String = _schema_block(spec, "TelemetryEvent")
	var types: PackedStringArray = _enum(event_block, "        type:")
	var channel_types: PackedStringArray = _enum(_schema_block(spec, "ChannelType"), "enum:")
	var sources: PackedStringArray = _enum(event_block, "        source:")
	var gap: Dictionary = _range(event_block, "gap_count")
	var size: Dictionary = _range(event_block, "bytes")
	var batch: Dictionary = await _sent_batch()
	for raw: Variant in batch["events"] as Array:
		var e: Dictionary = raw as Dictionary
		assert_bool(String(e["type"]) in types).override_failure_message(String(e["type"])).is_true()
		assert_bool(Uuid.is_valid(String(e["client_event_id"]))).is_true()
		if e["type"] == TelemetryService.TYPE_CHAT_SEQ_GAP:
			assert_bool(String(e["channel_type"]) in channel_types).is_true()
			var n: float = float(e["gap_count"])
			assert_float(n).is_equal(floorf(n))
			assert_bool(n >= gap["minimum"] and n <= gap["maximum"]).override_failure_message(str(n)).is_true()
		else:
			assert_bool(String(e["source"]) in sources).is_true()
			var b: float = float(e["bytes"])
			assert_float(b).is_equal(floorf(b))
			assert_bool(b >= size["minimum"] and b <= size["maximum"]).override_failure_message(str(b)).is_true()


func test_client_lists_equal_the_spec() -> void:
	var spec: String = _spec()
	var event_block: String = _schema_block(spec, "TelemetryEvent")
	assert_array(Array(TelemetryService.TYPES)).contains_exactly_in_any_order(Array(_enum(event_block, "        type:")))
	assert_array(Array(TelemetryService.DOWNLOAD_SOURCES)).contains_exactly_in_any_order(
			Array(_enum(event_block, "        source:")))
	assert_array(Array(ChatChannelModel.TYPES)).contains_exactly_in_any_order(
			Array(_enum(_schema_block(spec, "ChannelType"), "enum:")))
	assert_int(TelemetryService.GAP_COUNT_MAX).is_equal(int(_range(event_block, "gap_count")["maximum"]))
	assert_int(TelemetryService.DOWNLOAD_BYTES_MAX).is_equal(int(_range(event_block, "bytes")["maximum"]))
	var sample_event: Dictionary = (_fixture()["events"] as Array)[0] as Dictionary
	for field: String in TelemetryService.FIELDS:
		assert_bool(sample_event.has(field)).override_failure_message(field).is_true()
