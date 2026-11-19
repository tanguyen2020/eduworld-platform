class_name Backoff
extends RefCounted
## REST retry delays (GAME_NETWORKING §2.3, CONFIG_DEFAULTS `client_reconnect_backoff` = 0.5–15 s, jitter ±20 %):
## `delay = min(15 s, 0.5 s × 2^(n−1)) × (1 ± 0.2)`, at most 3 automatic retries. The login queue
## (`429 LOGIN_QUEUE_BUSY`) waits `Retry-After` plus a random 0–50 % (GAME_NETWORKING §3).

const BASE_SEC: float = 0.5
const CAP_SEC: float = 15.0
const JITTER: float = 0.2
const MAX_RETRIES: int = 3
const MAX_RATE_LIMIT_RETRIES: int = 2
const QUEUE_JITTER_MAX: float = 0.5
const DEFAULT_RETRY_AFTER_SEC: float = 1.0


## Delay before retry number `attempt` (1-based). A null `rng` gives the delay without jitter (tests).
static func delay(attempt: int, rng: RandomNumberGenerator = null) -> float:
	var base: float = minf(CAP_SEC, BASE_SEC * pow(2.0, float(maxi(attempt, 1) - 1)))
	var jitter: float = rng.randf_range(-JITTER, JITTER) if rng != null else 0.0
	return base * (1.0 + jitter)


## Login queue wait: `Retry-After` + 0–50 % jitter.
static func queue_delay(retry_after_sec: float, rng: RandomNumberGenerator = null) -> float:
	var wait: float = retry_after_sec if retry_after_sec > 0.0 else DEFAULT_RETRY_AFTER_SEC
	var jitter: float = rng.randf_range(0.0, QUEUE_JITTER_MAX) if rng != null else 0.0
	return wait * (1.0 + jitter)
