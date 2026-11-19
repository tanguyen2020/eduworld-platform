class_name TokenStore
extends RefCounted
## Tokens of the current session, RAM only (GODOT_ARCHITECTURE §5.1, ADR-0086, F061-AC-04): this class has no API
## that writes to disk. The refresh token is copied to `SessionStore` (SecureStorage) by `AuthService`; the access
## token never leaves RAM. Expiry uses the monotonic clock (research R3).

## Refresh when the access token has less than this left (GAME_NETWORKING §3).
const REFRESH_MARGIN_MSEC: int = 60_000

var access_token: String = ""
var refresh_token: String = ""
var access_expires_at_msec: int = 0
var refresh_expires_at_msec: int = 0
## Tenant of the session (`bootstrap.tenant_id`) → `X-Tenant-Id` (ADR-0065). Never changes within a session.
var tenant_id: String = ""


## Applies a `GameTokenSet` (also the top level of `GameTokenRefreshResponse`). Returns false and keeps the
## current tokens when the set is malformed.
func apply(token_set: Dictionary, now_msec: int) -> bool:
	var access: Variant = token_set.get("access_token")
	var refresh: Variant = token_set.get("refresh_token")
	var expires_in: Variant = token_set.get("expires_in")
	if not (access is String and refresh is String and (expires_in is int or expires_in is float)):
		return false
	if (access as String).is_empty() or (refresh as String).is_empty():
		return false
	access_token = access as String
	refresh_token = refresh as String
	access_expires_at_msec = now_msec + int(expires_in) * 1000
	var refresh_in: Variant = token_set.get("refresh_expires_in", 0)
	refresh_expires_at_msec = now_msec + int(refresh_in) * 1000 if (refresh_in is int or refresh_in is float) else 0
	return true


func has_access() -> bool:
	return not access_token.is_empty()


## True when there is no access token or it expires within REFRESH_MARGIN_MSEC.
func access_expiring(now_msec: int) -> bool:
	return access_token.is_empty() or access_expires_at_msec - now_msec < REFRESH_MARGIN_MSEC


func clear() -> void:
	access_token = ""
	refresh_token = ""
	access_expires_at_msec = 0
	refresh_expires_at_msec = 0
	tenant_id = ""


func _to_string() -> String:
	return "TokenStore([REDACTED])"
