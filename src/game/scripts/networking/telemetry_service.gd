class_name TelemetryService
extends RefCounted
## Client telemetry batches for `POST /game/telemetry` (`postTelemetry`, `TelemetryBatch`, GAME_ANALYTICS §3.2,
## MONITORING §10). Events wait in a RAM queue; `flush()` sends up to 100 of them and drops them only after a 2xx,
## so an event lost to the network is sent again later with the same `client_event_id` (server de-duplicates,
## F062-AC-24). No PII: only the fields of `TelemetryEvent` are kept (AGENTS R14). When the school turns product
## telemetry off, `screen_view`/`tutorial_step` are dropped; perf, errors, `break_reminder_shown` and the metric-only
## `chat_seq_gap` / `content_download` still go. The two metric-only types (MONITORING §3 counters) are merged per label
## (`channel_type` / `source`) into the event still waiting for the next batch — at most one per label per flush
## window, the count / bytes summed up to the contract maximum — and never into an event of a batch already sent.
## Not persisted to disk (GODOT_ARCHITECTURE §8.2).

signal flushed(sent: int)

const PATH: String = "/game/telemetry"
## `TelemetryBatch.events` maxItems.
const MAX_BATCH: int = 100
## RAM cap; the oldest events are dropped beyond it.
const QUEUE_MAX: int = 300
## MONITORING §10: rate limit 1 request / 30 s / session.
const MIN_FLUSH_INTERVAL_SEC: float = 30.0
const TYPE_BREAK_REMINDER: String = "break_reminder_shown"
const TYPE_CLIENT_ERROR: String = "client_error"
const TYPE_PERF_SAMPLE: String = "perf_sample"
const TYPE_LOAD_TIME: String = "load_time"
## Metric-only (`eduworld_chat_seq_gaps_total{channel_type}`, `eduworld_client_content_download_bytes_total{source}`).
const TYPE_CHAT_SEQ_GAP: String = "chat_seq_gap"
const TYPE_CONTENT_DOWNLOAD: String = "content_download"
const TYPES: PackedStringArray = [
	"screen_view", "tutorial_step", "perf_sample", "load_time", "client_error", "crash", "network_sample",
	"break_reminder_shown", "chat_seq_gap", "content_download",
]
## Non-essential product events (GAME_ANALYTICS §3.2).
const PRODUCT_TYPES: PackedStringArray = ["screen_view", "tutorial_step"]
## Properties of `TelemetryEvent` besides `type`, `client_event_id`, `occurred_at`.
const FIELDS: PackedStringArray = [
	"screen_code", "duration_ms", "step_code", "result", "scene_code", "fps_p50", "fps_p10", "frame_ms_p95",
	"render_scale", "mem_mb", "kind", "ms", "error_code", "fatal", "crash_signature", "rtt_p50_ms", "rtt_p95_ms",
	"reconnects", "reminder_no", "minutes_played", "channel_type", "gap_count", "source", "bytes",
]
const CODE_MAX_LENGTH: int = 64
## `TelemetryEvent.gap_count` maximum (minimum 1).
const GAP_COUNT_MAX: int = 1000
## `TelemetryEvent.bytes` maximum (10 GiB, minimum 0).
const DOWNLOAD_BYTES_MAX: int = 10737418240
## `TelemetryEvent.source` (PERFORMANCE_BUDGET §10.1).
const DOWNLOAD_SOURCE_PREINSTALLED: String = "preinstalled"
const DOWNLOAD_SOURCE_LAN_CACHE: String = "lan_cache"
const DOWNLOAD_SOURCE_CDN: String = "cdn"
const DOWNLOAD_SOURCES: PackedStringArray = [DOWNLOAD_SOURCE_PREINSTALLED, DOWNLOAD_SOURCE_LAN_CACHE, DOWNLOAD_SOURCE_CDN]

var api: RestApiClient = null
var clock: ClientClock = null
var queue: Array[Dictionary] = []
var product_telemetry_enabled: bool = true
var platform: String = ""
var client_version: String = ""
## Batch context (`QualityPreset`, `RendererKind`), filled by QualityManager.
var preset: String = ""
var renderer: String = ""
var _in_flight: bool = false
var _last_flush_msec: int = -1
## "type|label" → the queued metric-only event still open for merging (not yet part of a batch being sent).
var _open_metrics: Dictionary = {}


func _init(rest: RestApiClient = null, time_source: ClientClock = null) -> void:
	api = rest
	clock = time_source if time_source != null else ClientClock.new()


## Queues one event; returns it (empty when dropped: unknown type or product telemetry off).
func queue_event(event_type: String, fields: Dictionary = {}) -> Dictionary:
	if not event_type in TYPES:
		return {}
	if event_type in PRODUCT_TYPES and not product_telemetry_enabled:
		return {}
	var event: Dictionary = {
		"type": event_type,
		"client_event_id": Uuid.v7(),
		"occurred_at": Time.get_datetime_string_from_unix_time(int(clock.unix_time())) + "Z",
	}
	for key: Variant in fields.keys():
		if String(key) in FIELDS:
			var value: Variant = fields[key]
			event[String(key)] = (value as String).left(CODE_MAX_LENGTH) if value is String else value
	queue.append(event)
	while queue.size() > QUEUE_MAX:
		queue.pop_front()
	return event


## A seq gap of a chat channel (ChatStore `gap_detected`): only the channel type and the number of missing messages
## (clamped 1–1 000) — never the channel id, a user id or any content (AGENTS R14). Unknown type → dropped ({}).
func queue_chat_seq_gap(channel_type: String, gap_count: int) -> Dictionary:
	if not channel_type in ChatChannelModel.TYPES:
		return {}
	return _add_metric(TYPE_CHAT_SEQ_GAP, "channel_type", channel_type, "gap_count",
			clampi(gap_count, 1, GAP_COUNT_MAX), GAP_COUNT_MAX)


## A content package / public media finished downloading from `source` (`preinstalled`, `lan_cache`, `cdn`), `bytes`
## clamped 0–10 GiB (F133-AC-21). No URL, file name, hash or bundle id is sent. Unknown source → dropped ({}).
func queue_content_download(source: String, bytes: int) -> Dictionary:
	if not source in DOWNLOAD_SOURCES:
		return {}
	return _add_metric(TYPE_CONTENT_DOWNLOAD, "source", source, "bytes", clampi(bytes, 0, DOWNLOAD_BYTES_MAX),
			DOWNLOAD_BYTES_MAX)


## Adds `amount` (≤ `cap`) to the open event of (`event_type`, `label`), filling it up to `cap`; the rest (or the
## whole amount when no event is open) goes into a new event, which becomes the open one. Returns the event that took
## the first part of `amount`.
func _add_metric(event_type: String, label_key: String, label: String, count_key: String, amount: int,
		cap: int) -> Dictionary:
	var key: String = "%s|%s" % [event_type, label]
	var open: Dictionary = _open_metrics.get(key, {}) as Dictionary
	if not open.is_empty() and _is_queued(open):
		var room: int = cap - int(open[count_key])
		if amount <= room:
			open[count_key] = int(open[count_key]) + amount
			return open
		open[count_key] = cap
		_open_metrics[key] = queue_event(event_type, {label_key: label, count_key: amount - room})
		return open
	var event: Dictionary = queue_event(event_type, {label_key: label, count_key: amount})
	_open_metrics[key] = event
	return event


func _is_queued(event: Dictionary) -> bool:
	for queued: Dictionary in queue:
		if is_same(queued, event):
			return true
	return false


## F062-AC-24: one `break_reminder_shown` per reminder actually shown (no name, username, student id or position).
func queue_break_reminder(reminder_no: int, minutes_played: int) -> Dictionary:
	return queue_event(TYPE_BREAK_REMINDER, {
		"reminder_no": clampi(reminder_no, 1, PlayTimeService.MAX_REMINDERS),
		"minutes_played": clampi(minutes_played, 0, PlayTimeService.MAX_MINUTES_PLAYED),
	})


## A client-side error without PII (`client_error`, e.g. `client_unsupported_assignment_kind`, `renderer_fallback`).
func queue_client_error(error_code: String, scene_code: String = "", fatal: bool = false) -> Dictionary:
	return queue_event(TYPE_CLIENT_ERROR, {"error_code": error_code, "scene_code": scene_code, "fatal": fatal})


## Moves the events queued on AppState by earlier features (client_bug, i18n.missing_key, auth_refresh_reused…)
## into this queue as `client_error` with the event name as `error_code` (their fields are not sent).
func drain_app_state(app_state: Node) -> int:
	if app_state == null:
		return 0
	var pending: Array[Dictionary] = app_state.get("pending_telemetry") as Array[Dictionary]
	var moved: int = 0
	for item: Dictionary in pending:
		if not queue_client_error(String(item.get("event", ""))).is_empty():
			moved += 1
	pending.clear()
	return moved


func can_flush() -> bool:
	if queue.is_empty() or _in_flight or api == null:
		return false
	return _last_flush_msec < 0 or clock.now_msec() - _last_flush_msec >= int(MIN_FLUSH_INTERVAL_SEC * 1000.0)


## Coroutine: sends the oldest ≤ 100 events. Returns the ApiResult, or null when nothing was sent.
func flush(force: bool = false) -> ApiResult:
	if not force and not can_flush():
		return null
	if queue.is_empty() or _in_flight or api == null:
		return null
	_in_flight = true
	_last_flush_msec = clock.now_msec()
	# The events of this batch are on their way: later counts open new events (a sent event is never changed).
	_open_metrics.clear()
	var batch: Array[Dictionary] = queue.slice(0, MAX_BATCH)
	var body: Dictionary = {
		"platform": platform if not platform.is_empty() else api.platform,
		"client_version": client_version if not client_version.is_empty() else api.client_version,
		"events": batch,
	}
	if not preset.is_empty():
		body["preset"] = preset
	if not renderer.is_empty():
		body["renderer"] = renderer
	var locale: String = RestApiClient.accept_language()
	if L10n.is_supported(locale, false):
		body["locale"] = locale
	var req: ApiRequest = ApiRequest.make_post(PATH, body)
	# Telemetry never waits on 429: the batch stays queued for the next flush.
	req.max_rate_limit_retries = 0
	var result: ApiResult = await api.send(req)
	_in_flight = false
	if result.ok:
		var sent: Dictionary = {}
		for event: Dictionary in batch:
			sent[event["client_event_id"]] = true
		var kept: Array[Dictionary] = []
		for event: Dictionary in queue:
			if not sent.has(event["client_event_id"]):
				kept.append(event)
		queue = kept
		flushed.emit(batch.size())
	return result


func clear() -> void:
	queue.clear()
	_open_metrics.clear()
	_last_flush_msec = -1
