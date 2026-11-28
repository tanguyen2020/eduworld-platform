extends GdUnitTestSuite
## T025 (F111-AC-16, F111-AC-17; EVENT_SEASON_SYSTEM §7, ADR-0049): `getSeasonTrack` fills the "Mùa" tab, `404` means
## "Mùa mới sắp bắt đầu"; "Nhận" of a reached tier is `claimSeasonTier` with one Idempotency-Key kept when retried, the
## tier reads claimed only after the server answered and the rewards are the server's; `409 SEASON_TIER_NOT_REACHED` reads
## the track again; `season.track.updated` replaces points / tier / the points left today; "Mùa trước" from
## `listSeasonTrackHistory`.

var s: F111SceneHarness = null
var season: SeasonTrackService = null
var rewards: Array = []


func before_test() -> void:
	s = F111SceneHarness.new()
	await s.start("g6_7")
	season = s.services.season
	rewards.clear()
	season.reward_claimed.connect(func(list: Array) -> void: rewards.append(list))


func after_test() -> void:
	s.dispose()


func test_load_and_no_season() -> void:
	var r: ApiResult = await season.load_track()
	assert_bool(r.ok).is_true()
	assert_int(season.store.track.tiers.size()).is_equal(30)
	assert_bool(season.store.missing).is_false()
	s.h.http.clear(F111Fixtures.ROUTE_SEASON).reply(F111Fixtures.ROUTE_SEASON, 404, AuthHarness.problem(404, "NOT_FOUND"))
	await season.load_track()
	assert_bool(season.store.missing).is_true()
	assert_object(season.store.track).is_null()


func test_claim_tier_keeps_its_key_and_marks_only_after_the_server() -> void:
	await season.load_track()
	s.h.http.clear(F111Fixtures.ROUTE_TIER_CLAIM).fail(F111Fixtures.ROUTE_TIER_CLAIM)
	await season.claim_tier(2)
	assert_bool(season.store.track.tier_model(2).claimed).is_false()
	var key: String = String(s.h.http.last(F111Fixtures.ROUTE_TIER_CLAIM).headers["idempotency-key"])
	s.h.http.clear(F111Fixtures.ROUTE_TIER_CLAIM).reply(F111Fixtures.ROUTE_TIER_CLAIM, 200,
			F111Fixtures.claim_result([F111Fixtures.reward_xp(22)]))
	await season.claim_tier(2)
	assert_str(String(s.h.http.last(F111Fixtures.ROUTE_TIER_CLAIM).headers["idempotency-key"])).is_equal(key)
	assert_str(s.h.http.last(F111Fixtures.ROUTE_TIER_CLAIM).path).ends_with("/tiers/2:claim")
	assert_bool(season.store.track.tier_model(2).claimed).is_true()
	assert_int(rewards.size()).is_equal(1)


func test_not_reached_reads_the_track_again() -> void:
	await season.load_track()
	s.h.http.clear(F111Fixtures.ROUTE_TIER_CLAIM).reply(F111Fixtures.ROUTE_TIER_CLAIM, 409,
			AuthHarness.problem(409, "SEASON_TIER_NOT_REACHED"))
	var loads: int = s.h.http.count(F111Fixtures.ROUTE_SEASON)
	await season.claim_tier(9)
	assert_int(s.h.http.count(F111Fixtures.ROUTE_SEASON)).is_greater(loads)
	assert_bool(season.store.track.tier_model(9).claimed).is_false()


func test_realtime_update_and_history() -> void:
	await season.load_track()
	var msg: Dictionary = F111Fixtures.ws("season.track.updated", {"season_id": F111Fixtures.SEASON_ID, "delta": 80,
		"points": 330, "tier": 4, "tier_reached": true, "daily_points_remaining": 0})
	s.push("season.track.updated", msg)
	s.push("season.track.updated", msg)
	assert_int(season.store.track.points).is_equal(330)
	assert_int(season.store.track.tier).is_equal(4)
	assert_bool(season.store.track.tier_model(4).reached).is_true()
	assert_int(season.store.track.daily_points_remaining).is_equal(0)
	var r: ApiResult = await season.load_history()
	assert_bool(r.ok).is_true()
	assert_int(season.store.history.size()).is_equal(1)
