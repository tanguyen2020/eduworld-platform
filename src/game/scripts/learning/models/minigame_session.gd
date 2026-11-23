class_name MiniGameSessionModel
extends RefCounted
## One mini-game play on the client (F073, MINIGAME_ENGINE §5): the `Attempt` (render payloads of the items, status,
## deadline), the `minigame_type`, the pinned configuration version and its display config. Built either from the
## `MiniGameSession` of `startMiniGame` (free play) or from a `minigame` activity attempt (quest step / school
## assignment: `Attempt.render.render_payload = {minigame_type, minigame_config_version_id, config}`). RAM only; the
## clock is the server's: `remaining_ms()` uses `deadline_at` and `server_time` with the local offset taken when the
## session was received, so the countdown keeps running while the network is down (F073-AC-23).

const TYPE_MAX_LENGTH: int = 64

var attempt: AttemptModel = null
var minigame_type: String = ""
var minigame_config_version_id: String = ""
var config: MiniGameConfigModel = MiniGameConfigModel.new()
var deadline_at: String = ""
var server_time: String = ""
## Local unix seconds when the session was received (offset between the server clock and this device).
var received_unix: float = 0.0


## Parses a decoded `MiniGameSession`; null when the attempt or the type is missing.
static func from_dict(data: Dictionary, now_unix: float = 0.0) -> MiniGameSessionModel:
	var raw_attempt: Variant = data.get("attempt")
	if not raw_attempt is Dictionary or not data.get("minigame_type") is String:
		return null
	var a: AttemptModel = AttemptModel.from_dict(raw_attempt as Dictionary, now_unix)
	if a == null:
		return null
	var s: MiniGameSessionModel = MiniGameSessionModel.new()
	s.attempt = a
	s.minigame_type = (data["minigame_type"] as String).left(TYPE_MAX_LENGTH)
	s.minigame_config_version_id = _text(data, "minigame_config_version_id")
	var cfg: Variant = data.get("config")
	s.config = MiniGameConfigModel.from_dict(cfg as Dictionary if cfg is Dictionary else {})
	s.deadline_at = _text(data, "deadline_at") if not _text(data, "deadline_at").is_empty() else a.deadline_at
	s.server_time = _text(data, "server_time") if not _text(data, "server_time").is_empty() else a.server_time
	s.received_unix = a.received_unix
	return s


## A session from a `minigame` activity attempt (ActivityHost → MinigameRenderer). Null without `minigame_type`.
static func from_attempt(a: AttemptModel) -> MiniGameSessionModel:
	if a == null or a.render == null:
		return null
	var payload: Dictionary = a.render.render_payload
	if not payload.get("minigame_type") is String:
		return null
	var s: MiniGameSessionModel = MiniGameSessionModel.new()
	s.attempt = a
	s.minigame_type = (payload["minigame_type"] as String).left(TYPE_MAX_LENGTH)
	s.minigame_config_version_id = _text(payload, "minigame_config_version_id")
	var cfg: Variant = payload.get("config")
	s.config = MiniGameConfigModel.from_dict(cfg as Dictionary if cfg is Dictionary else {})
	s.deadline_at = a.deadline_at
	s.server_time = a.server_time
	s.received_unix = a.received_unix
	return s


## Milliseconds left before `deadline_at` on the server clock at local time `now_unix`; -1 without a deadline.
func remaining_ms(now_unix: float) -> int:
	if deadline_at.is_empty():
		return -1
	var deadline: float = Time.get_unix_time_from_datetime_string(deadline_at)
	var server_now: float = Time.get_unix_time_from_datetime_string(server_time) if not server_time.is_empty() \
			else received_unix
	return maxi(0, int(round((deadline - (server_now + (now_unix - received_unix))) * 1000.0)))


func has_deadline() -> bool:
	return not deadline_at.is_empty()


func items() -> Array[QuestionRenderModel]:
	return attempt.items if attempt != null else [] as Array[QuestionRenderModel]


static func _text(data: Dictionary, key: String) -> String:
	var v: Variant = data.get(key)
	return v as String if v is String else ""
