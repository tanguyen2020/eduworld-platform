class_name GameRealtimeClient
extends Node
## Autoload #7 `RealtimeClient` (GODOT_ARCHITECTURE §4, GAME_NETWORKING §4; F092 US1). One WebSocket per game
## session to `/realtime/v1` (subprotocol `eduworld.v1`), polled in `_process` without blocking the main thread:
## - ticket: `createRealtimeTicket` through ApiClient (`X-Tenant-Id`), opened once as `url?ticket=…` — the ticket and
##   the access token are never logged, stored or sent over the socket (R14, ADR-0008); `wss` only in release builds;
## - `system.session.welcome` → limits, heartbeat, server clock, resume token (RAM only);
## - heartbeat `system.heartbeat.ping` every `heartbeat_s`; nothing received for 2 × `heartbeat_s` → reconnect;
## - commands: `send_command(type, payload)` with a UUIDv7 `id`, completed by `system.command.acked`/`failed` with the
##   same `ref` or `client_command_timeout`; `hub.player.move` is fire-and-forget (`send_fire_and_forget`);
## - dispatch by type prefix (`hub.*` → HubSync, …) after the envelope and payload checks; unknown types are dropped;
## - reconnect by close code (data-model §2, GAME_NETWORKING §4.4) and resume with `last_seq`; duplicates are dropped;
##   `system.session.resume_failed` → `resync_required` (REST resync + hub join by the services);
## - `session.session.replaced`/4409, `session.session.ended`/4403/4410 stop the client and are handed to AuthService.
## It decides nothing about the game (server-authoritative, R5) and never connects to NATS or Valkey (R25).

signal connection_state_changed(state: int)
signal message_received(type: String, payload: Dictionary)
## Online again after a resume (`system.session.resumed`).
signal session_resumed()
## `system.session.resume_failed` (or a `seq` gap): services resync over REST and join their rooms again.
signal resync_required()
## `session.session.replaced` then close 4409 (ADR-0031).
signal session_replaced_received()
## Close 4403 / 4410 after `session.session.ended` (reason, `reopen_at` epoch ms or 0); reason empty when the server
## closed with 4403 without the message.
signal session_ended_received(reason: String, reopen_at_ms: int)
## `createRealtimeTicket` refused for good (`GAME_ACCESS_STUDENT_ONLY`, `CURFEW_ACTIVE`, game session gone).
signal access_stopped(code: String)
## `system.service.degraded` (features) — empty when the service is back.
signal service_degraded(features: PackedStringArray)

enum State { DISCONNECTED, FETCHING_TICKET, CONNECTING, WELCOMED, RESUMING, RESYNC, ONLINE, BACKOFF, STOPPED }

const PROTOCOL: String = "eduworld.v1"
const PATH_TICKETS: String = "/game/sessions/%s/realtime-tickets"
## data-model §2: connect/welcome timeout 10 s.
const CONNECT_TIMEOUT_MS: int = 10000
## CONFIG_DEFAULTS `client_command_timeout` (10 s).
const COMMAND_TIMEOUT_MS: int = 10000
## GAME_NETWORKING §4.4: close 4429 → wait at least 30 s.
const RATE_LIMITED_WAIT_MS: int = 30000
## GAME_NETWORKING §4.3: silence of 2 × heartbeat → reconnect.
const SILENCE_FACTOR: int = 2
## Mobile background longer than this closes the socket (GODOT_ARCHITECTURE §5, 5 min).
const BACKGROUND_CLOSE_SEC: float = 300.0
const OFFSET_SAMPLES: int = 5
## CA bundle shared with ApiClient (main + backup CA, packaged by F133; OQ pinning 2026-11-20). Absent → engine CAs.
const CA_BUNDLE_PATH: String = "res://config/tls/ca_bundle.crt"

const CLOSE_NORMAL: int = 1000
const CLOSE_SERVICE_RESTART: int = 1012
const CLOSE_BAD_MESSAGE: int = 4400
const CLOSE_TICKET_INVALID: int = 4401
const CLOSE_REVOKED: int = 4403
const CLOSE_HEARTBEAT_TIMEOUT: int = 4408
const CLOSE_REPLACED: int = 4409
const CLOSE_SESSION_ENDED: int = 4410
const CLOSE_RATE_LIMITED: int = 4429

const TYPE_PING: String = "system.heartbeat.ping"
const TYPE_PONG: String = "system.heartbeat.pong"
const TYPE_WELCOME: String = "system.session.welcome"
const TYPE_RESUME: String = "system.session.resume"
const TYPE_RESUMED: String = "system.session.resumed"
const TYPE_RESUME_FAILED: String = "system.session.resume_failed"
const TYPE_ACKED: String = "system.command.acked"
const TYPE_FAILED: String = "system.command.failed"
const TYPE_DEGRADED: String = "system.service.degraded"
const TYPE_DRAINING: String = "system.server.draining"
const TYPE_REPLACED: String = "session.session.replaced"
const TYPE_ENDED: String = "session.session.ended"
## Ticket refusals that stop without retry (F092-AC-02; TENANT_MISMATCH also reaches AuthService through ApiClient).
const STOP_CODES: PackedStringArray = ["GAME_ACCESS_STUDENT_ONLY", "TENANT_MISMATCH", "CURFEW_ACTIVE"]
const CONNECTED_STATES: Array[int] = [State.WELCOMED, State.RESUMING, State.RESYNC, State.ONLINE]

var api: RestApiClient = null
var clock: ClientClock = null
var rng: RandomNumberGenerator = RandomNumberGenerator.new()
## `() -> WsPeer`; tests return a FakeWebSocketPeer.
var peer_factory: Callable = Callable()
## Debug builds may use `ws://` (local compose); release builds only `wss://` (F092-AC-07).
var allow_insecure: bool = OS.is_debug_build()
## Client TLS options of the release build (CA bundle shared with ApiClient, OQ pinning 2026-11-20); null = engine
## defaults (Web uses the browser TLS).
var tls_options: TLSOptions = null
## Poll in `_process` (tests drive `tick()` themselves).
var auto_poll: bool = true
var state: State = State.DISCONNECTED
var session: RealtimeSession = RealtimeSession.new()
var stats: NetStats = NetStats.new()
var game_session_id: String = ""
var degraded_features: PackedStringArray = []
## True while the reconnect follows `system.server.draining` (no banner, F092-AC-04).
var quiet_reconnect: bool = false
## Server clock estimate: server epoch ms = clock.now_msec() + offset.
var server_offset_ms: int = 0

var _peer: WsPeer = null
var _handlers: Dictionary = {}
var _pending: Dictionary = {}
var _deadline_ms: int = 0
var _last_rx_ms: int = 0
var _next_ping_ms: int = 0
var _attempt: int = 0
var _generation: int = 0
var _drain_after_ms: int = -1
var _ended_reason: String = ""
var _ended_reopen_ms: int = 0
var _replaced: bool = false
var _resume_token_sent: String = ""
## Token of the new connection while resuming: it applies when the resume fails.
var _welcome_token: String = ""
var _was_online: bool = false
## [rtt_ms, offset_ms] pairs (64-bit ints: an epoch offset does not fit a Vector2i).
var _offset_samples: Array[Array] = []


func _ready() -> void:
	# F113-AC-04: the socket keeps polling (heartbeat, messages) behind the rotate prompt, which pauses the tree.
	process_mode = Node.PROCESS_MODE_ALWAYS
	set_process(auto_poll)
	if not OS.has_feature("web"):
		tls_options = load_tls_options(CA_BUNDLE_PATH)


## `TLSOptions.client(trusted_chain)` of the packaged CA bundle, or null when the build has none (T030).
static func load_tls_options(path: String) -> TLSOptions:
	if not FileAccess.file_exists(path):
		return null
	var chain: X509Certificate = X509Certificate.new()
	if chain.load(path) != OK:
		Log.warn("realtime_ca_bundle_invalid", {})
		return null
	return TLSOptions.client(chain)


func _process(_delta: float) -> void:
	if auto_poll:
		tick()


## Test seam and app wiring: ApiClient (tickets) and the time source.
func configure(rest: RestApiClient, time_source: ClientClock, factory: Callable = Callable()) -> void:
	api = rest
	clock = time_source
	peer_factory = factory


# ---------- lifecycle ----------

## Opens the connection of `session_id` (after `createGameSession`, F092-AC-01). A new session forgets the resume
## token of the previous one.
func start(session_id: String) -> void:
	if session_id != game_session_id:
		session.clear()
	game_session_id = session_id
	_replaced = false
	_ended_reason = ""
	_attempt = 0
	quiet_reconnect = false
	_was_online = false
	_close_peer(CLOSE_NORMAL)
	_fetch_ticket()


## Closes and forgets everything (logout, session end, takeover): no reconnect.
func stop() -> void:
	_generation += 1
	_close_peer(CLOSE_NORMAL)
	session.clear()
	game_session_id = ""
	degraded_features.clear()
	_fail_pending(CommandResult.CODE_OFFLINE)
	_set_state(State.DISCONNECTED)


## Mobile back from background: after BACKGROUND_CLOSE_SEC the socket is closed and opened again with a new ticket
## and a resume (GODOT_ARCHITECTURE §5).
func on_app_resumed(paused_sec: float) -> void:
	if paused_sec >= BACKGROUND_CLOSE_SEC and state in CONNECTED_STATES:
		_close_peer(CLOSE_NORMAL)
		_attempt = 0
		_fetch_ticket()


func is_online() -> bool:
	return state == State.ONLINE


## Write commands are refused while reconnecting (F092-AC-08).
func can_write() -> bool:
	return state == State.ONLINE


## Online, or resyncing after a failed resume (the resync joins its rooms again, F092-AC-05).
func _accepts_commands() -> bool:
	return state == State.ONLINE or state == State.RESYNC


## Banner "Đang kết nối lại…": the connection was up before and is coming back (not after `system.server.draining`).
func is_reconnecting() -> bool:
	return _was_online and not quiet_reconnect and state in [State.FETCHING_TICKET, State.CONNECTING,
			State.WELCOMED, State.RESUMING, State.RESYNC, State.BACKOFF]


func server_now_ms() -> int:
	return _now() + server_offset_ms


## Registers `handler(type: String, payload: Dictionary, envelope: WsEnvelope)` for messages whose type starts with
## `prefix` + "." (GAME_NETWORKING §4.2 dispatcher).
func register_handler(prefix: String, handler: Callable) -> void:
	var list: Array = _handlers.get(prefix, []) as Array
	if not handler in list:
		list.append(handler)
	_handlers[prefix] = list


func unregister_handler(prefix: String, handler: Callable) -> void:
	var list: Array = _handlers.get(prefix, []) as Array
	list.erase(handler)
	_handlers[prefix] = list


# ---------- sending ----------

## Coroutine: sends a command and returns its outcome (ack, failure code, TIMEOUT, OFFLINE).
func send_command(type: String, payload: Dictionary) -> CommandResult:
	if not _accepts_commands():
		return CommandResult.failure(CommandResult.CODE_OFFLINE)
	var id: String = Uuid.v7()
	if not _send(type, id, payload):
		return CommandResult.failure(CommandResult.CODE_INVALID)
	var pending: PendingCommand = PendingCommand.new()
	pending.id = id
	pending.type = type
	pending.deadline_ms = _now() + COMMAND_TIMEOUT_MS
	_pending[id] = pending
	var result: CommandResult = await pending.completed
	return result


## Sends without waiting for an ack (`hub.player.move`); false when offline or the frame is invalid/too large.
func send_fire_and_forget(type: String, payload: Dictionary) -> bool:
	if not can_write():
		return false
	return _send(type, Uuid.v7(), payload)


func pending_count() -> int:
	return _pending.size()


func _send(type: String, id: String, payload: Dictionary) -> bool:
	if _peer == null or _peer.get_ready_state() != WsPeer.ReadyState.OPEN:
		return false
	var text: String = WsEnvelope.encode(type, id, payload, _now(), session.max_frame_bytes)
	if text.is_empty():
		Log.debug("realtime_frame_rejected", {"type": type})
		return false
	if _peer.send_text(text) != OK:
		return false
	stats.bytes_out += text.to_utf8_buffer().size()
	return true


# ---------- polling ----------

## One poll step: reads frames, follows the close code, runs heartbeat, timeouts and backoff.
func tick() -> void:
	var now: int = _now()
	if _peer != null:
		_peer.poll()
		var ready: WsPeer.ReadyState = _peer.get_ready_state()
		if state == State.CONNECTING and ready == WsPeer.ReadyState.OPEN and _peer.get_selected_protocol() != PROTOCOL:
			Log.warn("realtime_protocol_mismatch", {})
			_close_peer(CLOSE_NORMAL)
			_schedule_backoff(0)
			return
		while _peer != null and _peer.get_available_packet_count() > 0:
			_on_frame(_peer.get_packet_text())
		if _peer != null and _peer.get_ready_state() == WsPeer.ReadyState.CLOSED:
			var code: int = _peer.get_close_code()
			_peer = null
			_on_closed(code)
			return
	match state:
		State.CONNECTING:
			if now >= _deadline_ms:
				_close_peer(CLOSE_NORMAL)
				_schedule_backoff(0)
		State.BACKOFF:
			if now >= _deadline_ms:
				_fetch_ticket()
		State.WELCOMED, State.RESUMING, State.RESYNC, State.ONLINE:
			_tick_heartbeat(now)
	_expire_pending(now)


func _tick_heartbeat(now: int) -> void:
	var heartbeat_ms: int = session.heartbeat_s * 1000
	if now - _last_rx_ms >= SILENCE_FACTOR * heartbeat_ms:
		Log.info("realtime_silence", {"heartbeat_s": session.heartbeat_s})
		_close_peer(CLOSE_NORMAL)
		_schedule_backoff(0)
		return
	if now >= _next_ping_ms:
		_next_ping_ms = now + heartbeat_ms
		_send(TYPE_PING, Uuid.v7(), {"client_time": now})


func _expire_pending(now: int) -> void:
	for id: Variant in _pending.keys():
		var p: PendingCommand = _pending[id] as PendingCommand
		if now >= p.deadline_ms:
			_pending.erase(id)
			p.finish(CommandResult.failure(CommandResult.CODE_TIMEOUT))


func _fail_pending(code: String) -> void:
	var list: Array = _pending.values()
	_pending.clear()
	for p: Variant in list:
		(p as PendingCommand).finish(CommandResult.failure(code))


# ---------- ticket and connect ----------

func _fetch_ticket() -> void:
	if game_session_id.is_empty() or api == null:
		_set_state(State.DISCONNECTED)
		return
	_generation += 1
	var generation: int = _generation
	_set_state(State.FETCHING_TICKET)
	var req: ApiRequest = ApiRequest.make_post(PATH_TICKETS % game_session_id)
	var result: ApiResult = await api.send(req)
	if generation != _generation:
		return
	if not result.ok:
		_on_ticket_failed(result)
		return
	var body: Dictionary = result.body()
	var ticket: String = String(body.get("ticket", "")) if body.get("ticket") is String else ""
	var url: String = String(body.get("url", "")) if body.get("url") is String else ""
	if ticket.is_empty() or not _url_allowed(url):
		Log.warn("realtime_ticket_invalid", {"insecure": not url.begins_with("wss://")})
		_schedule_backoff(0)
		return
	_peer = _new_peer()
	var target: String = "%s%sticket=%s" % [url, "&" if url.contains("?") else "?", ticket.uri_encode()]
	var err: Error = _peer.connect_to_url(target, PackedStringArray([PROTOCOL]), tls_options)
	target = ""
	ticket = ""
	if err != OK:
		_peer = null
		_schedule_backoff(0)
		return
	_deadline_ms = _now() + CONNECT_TIMEOUT_MS
	_set_state(State.CONNECTING)


func _on_ticket_failed(result: ApiResult) -> void:
	if result.status == 403 and result.code in STOP_CODES:
		_set_state(State.STOPPED)
		if result.code != "TENANT_MISMATCH":
			access_stopped.emit(result.code)
		return
	if result.status == 401:
		_set_state(State.STOPPED)
		return
	if result.status == 404 or result.status == 409:
		_set_state(State.STOPPED)
		access_stopped.emit("GAME_SESSION_ENDED")
		return
	if result.status == 426 or result.code == RestApiClient.CODE_INSECURE_TRANSPORT:
		_set_state(State.STOPPED)
		return
	var wait_ms: int = 0
	if result.status == 429 or result.status == 503:
		wait_ms = int(result.retry_after_sec() * 1000.0)
	_schedule_backoff(wait_ms)


func _url_allowed(url: String) -> bool:
	if url.begins_with("wss://"):
		return true
	return allow_insecure and url.begins_with("ws://")


func _new_peer() -> WsPeer:
	if peer_factory.is_valid():
		return peer_factory.call() as WsPeer
	return WsPeer.new()


func _close_peer(code: int) -> void:
	if _peer != null:
		_peer.close(code)
		_peer = null


## Backoff `client_reconnect_backoff` (0.5–15 s, ±20 %), or at least `min_wait_ms`.
func _schedule_backoff(min_wait_ms: int) -> void:
	_attempt += 1
	stats.reconnects += 1
	var wait_ms: int = maxi(int(Backoff.delay(_attempt, rng) * 1000.0), min_wait_ms)
	_deadline_ms = _now() + wait_ms
	_fail_pending(CommandResult.CODE_OFFLINE)
	_set_state(State.BACKOFF)


# ---------- receiving ----------

func _on_frame(text: String) -> void:
	stats.bytes_in += text.to_utf8_buffer().size()
	var env: WsEnvelope = WsEnvelope.parse(text)
	if env == null:
		Log.debug("realtime_frame_dropped", {"reason": "envelope"})
		return
	_last_rx_ms = _now()
	if not WsMessageSchemas.validate(env.type, env.payload):
		Log.debug("realtime_frame_dropped", {"reason": "payload", "type": env.type})
		return
	match env.type:
		TYPE_WELCOME:
			_on_welcome(env)
			return
		TYPE_RESUMED:
			_on_resumed(env)
			return
		TYPE_RESUME_FAILED:
			_on_resume_failed(env)
			return
	if env.seq <= session.last_seq:
		return
	session.last_seq = env.seq
	match env.type:
		TYPE_PONG:
			_on_pong(env.payload)
		TYPE_ACKED:
			_complete(env.ref, CommandResult.success(env.payload.get("result", {}) as Dictionary
					if env.payload.get("result") is Dictionary else {}))
		TYPE_FAILED:
			_on_failed(env)
		TYPE_DEGRADED:
			_on_degraded(env.payload)
		TYPE_DRAINING:
			_drain_after_ms = clampi(int(env.payload["reconnect_after_ms"]), 0, 120000)
		TYPE_REPLACED:
			_replaced = true
		TYPE_ENDED:
			_ended_reason = String(env.payload["reason"])
			_ended_reopen_ms = WsEnvelope.as_int(env.payload.get("reopen_at"))
		_:
			_dispatch(env)


func _on_welcome(env: WsEnvelope) -> void:
	var old_token: String = session.resume_token
	var resuming: bool = not old_token.is_empty() and session.last_seq > 0
	session.apply_welcome(env.payload)
	_set_offset(session.server_time_ms - _now(), 1 << 30)
	_attempt = 0
	_next_ping_ms = _now() + session.heartbeat_s * 1000
	if resuming:
		_resume_token_sent = old_token
		_welcome_token = String(env.payload["resume_token"])
		_set_state(State.RESUMING)
		_send(TYPE_RESUME, Uuid.v7(), {"resume_token": old_token, "last_seq": session.last_seq})
		return
	session.resume_token = String(env.payload["resume_token"])
	session.last_seq = env.seq
	_set_state(State.WELCOMED)
	_go_online()


func _on_resumed(env: WsEnvelope) -> void:
	if state != State.RESUMING:
		return
	# The server keeps the resumed logical connection: its token stays valid, its seq continues (F090-AC-14).
	session.resume_token = _resume_token_sent
	session.last_seq = maxi(session.last_seq, env.seq)
	stats.resume_ok += 1
	_go_online()
	session_resumed.emit()


func _on_resume_failed(env: WsEnvelope) -> void:
	if state != State.RESUMING:
		return
	stats.resume_failed += 1
	Log.info("realtime_resume_failed", {"reason": String(env.payload.get("reason", ""))})
	# A new logical connection: the welcome token and the new numbering apply.
	session.resume_token = _welcome_token
	session.last_seq = env.seq
	_set_state(State.RESYNC)
	resync_required.emit()


## Services call this when the REST resync and the room joins are done (F092-AC-05).
func finish_resync() -> void:
	if state == State.RESYNC:
		_go_online()


func _go_online() -> void:
	_was_online = true
	quiet_reconnect = false
	_drain_after_ms = -1
	_set_state(State.ONLINE)


func _on_pong(payload: Dictionary) -> void:
	if not WsEnvelope.is_int(payload.get("client_time")) or not WsEnvelope.is_int(payload.get("server_time")):
		return
	var sent_at: int = int(payload["client_time"])
	var rtt: int = _now() - sent_at
	if rtt < 0 or rtt > 60000:
		return
	stats.add_rtt(rtt)
	_set_offset(int(payload["server_time"]) - (sent_at + rtt / 2), rtt)


## NTP-like offset: median of the 5 samples with the lowest RTT (GAME_NETWORKING §6).
func _set_offset(offset_ms: int, rtt_ms: int) -> void:
	_offset_samples.append([rtt_ms, offset_ms])
	_offset_samples.sort_custom(func(a: Array, b: Array) -> bool: return int(a[0]) < int(b[0]))
	while _offset_samples.size() > OFFSET_SAMPLES:
		_offset_samples.pop_back()
	var offsets: Array[int] = []
	for sample: Array in _offset_samples:
		offsets.append(int(sample[1]))
	offsets.sort()
	server_offset_ms = offsets[offsets.size() / 2]


func _on_failed(env: WsEnvelope) -> void:
	var code: String = String(env.payload["code"])
	if env.ref.is_empty():
		Log.info("realtime_error", {"code": code})
		message_received.emit(env.type, env.payload)
		return
	var details: Dictionary = env.payload["details"] as Dictionary if env.payload.get("details") is Dictionary else {}
	_complete(env.ref, CommandResult.failure(code, WsEnvelope.as_int(env.payload.get("retry_after_ms")), details))


func _complete(ref: String, result: CommandResult) -> void:
	var p: PendingCommand = _pending.get(ref) as PendingCommand
	if p == null:
		return
	_pending.erase(ref)
	p.finish(result)


func _on_degraded(payload: Dictionary) -> void:
	degraded_features.clear()
	for f: Variant in payload.get("features", []) as Array:
		if f is String and String(f).length() <= 32:
			degraded_features.append(String(f))
	service_degraded.emit(degraded_features)


func _dispatch(env: WsEnvelope) -> void:
	var prefix: String = env.type.get_slice(".", 0)
	var list: Array = _handlers.get(prefix, []) as Array
	if list.is_empty():
		Log.debug("realtime_type_ignored", {"type": env.type})
		return
	message_received.emit(env.type, env.payload)
	for handler: Variant in list.duplicate():
		(handler as Callable).call(env.type, env.payload, env)


# ---------- close codes (GAME_NETWORKING §4.4) ----------

func _on_closed(code: int) -> void:
	if not state in CONNECTED_STATES and state != State.CONNECTING:
		return
	Log.info("realtime_closed", {"code": code})
	_fail_pending(CommandResult.CODE_OFFLINE)
	match code:
		CLOSE_TICKET_INVALID:
			_attempt = 0
			_fetch_ticket()
		CLOSE_REVOKED:
			_set_state(State.STOPPED)
			session.clear()
			session_ended_received.emit(_ended_reason, 0)
		CLOSE_SESSION_ENDED:
			_set_state(State.STOPPED)
			session.clear()
			session_ended_received.emit(_ended_reason if not _ended_reason.is_empty() else "curfew", _ended_reopen_ms)
		CLOSE_REPLACED:
			if _replaced:
				_set_state(State.STOPPED)
				session.clear()
				session_replaced_received.emit()
			else:
				# A newer connection of this device took over: nothing to show [Proposed: reconnect quietly].
				quiet_reconnect = true
				_schedule_backoff(0)
		CLOSE_RATE_LIMITED:
			_schedule_backoff(RATE_LIMITED_WAIT_MS)
		CLOSE_SERVICE_RESTART:
			if _drain_after_ms >= 0:
				quiet_reconnect = true
				_attempt = 0
				_deadline_ms = _now() + _drain_after_ms
				_drain_after_ms = -1
				stats.reconnects += 1
				_set_state(State.BACKOFF)
			else:
				_schedule_backoff(0)
		_:
			_schedule_backoff(0)


func _set_state(next: State) -> void:
	if next == state:
		return
	state = next
	Log.debug("realtime_state", {"state": State.keys()[next]})
	connection_state_changed.emit(next)
	var bus: Node = get_node_or_null("/root/EventBus") if is_inside_tree() else null
	if bus != null:
		bus.emit_signal("network_state_changed", next)


func _now() -> int:
	return clock.now_msec() if clock != null else Time.get_ticks_msec()
