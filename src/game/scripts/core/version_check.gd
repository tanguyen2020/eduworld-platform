class_name VersionCheck
extends RefCounted
## VersionCheck step (GODOT_ARCHITECTURE §5, F061-AC-01, FR-GAME-008): `GET /game/client-config?platform=&version=`
## (public: no token, no `X-Tenant-Id`) before the login form. `update_required` or a client version below
## `min_client_version` → ForceUpdate; `maintenance.active` → Maintenance (checked again periodically).
## When client-config cannot be fetched after the GET retries, the flow continues [Proposed]: the server still
## enforces the version at login/resume with `426 CLIENT_VERSION_UNSUPPORTED` (F061-AC-27).

enum Outcome { OK, FORCE_UPDATE, MAINTENANCE, UNAVAILABLE }

const PATH: String = "/game/client-config"

var config: ClientConfig = null


## Coroutine: fetches client-config and evaluates it.
func run(api: RestApiClient, platform: String, client_version: String) -> Outcome:
	var req: ApiRequest = ApiRequest.make_get(PATH, {"platform": platform, "version": client_version}).as_public()
	var result: ApiResult = await api.send(req)
	if not result.ok:
		config = null
		return Outcome.UNAVAILABLE
	config = ClientConfig.from_dict(result.body())
	if config == null:
		return Outcome.UNAVAILABLE
	return evaluate(config, client_version)


static func evaluate(cfg: ClientConfig, client_version: String) -> Outcome:
	if cfg.update_required or compare_versions(client_version, cfg.min_client_version) < 0:
		return Outcome.FORCE_UPDATE
	if cfg.maintenance_active:
		return Outcome.MAINTENANCE
	return Outcome.OK


## SemVer precedence of `major.minor.patch[-pre]` (build metadata ignored): -1, 0 or 1. A pre-release is lower
## than its release; pre-release identifiers compare as text. Unparsable parts count as 0.
static func compare_versions(a: String, b: String) -> int:
	var pa: Array = _parse(a)
	var pb: Array = _parse(b)
	for i: int in 3:
		if int(pa[i]) != int(pb[i]):
			return -1 if int(pa[i]) < int(pb[i]) else 1
	var pre_a: String = String(pa[3])
	var pre_b: String = String(pb[3])
	if pre_a == pre_b:
		return 0
	if pre_a.is_empty():
		return 1
	if pre_b.is_empty():
		return -1
	return -1 if pre_a < pre_b else 1


static func _parse(version: String) -> Array:
	var core: String = version.strip_edges().split("+")[0]
	var pre: String = ""
	var dash: int = core.find("-")
	if dash >= 0:
		pre = core.substr(dash + 1)
		core = core.substr(0, dash)
	var nums: PackedStringArray = core.split(".")
	var out: Array = []
	for i: int in 3:
		out.append(int(nums[i]) if i < nums.size() and nums[i].is_valid_int() else 0)
	out.append(pre)
	return out
