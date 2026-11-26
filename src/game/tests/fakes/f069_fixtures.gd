class_name F069Fixtures
extends RefCounted
## TEST-ONLY (AGENTS R17, MOCK data): responses and messages of the F069 routes (progression, wallet, inventory, shop,
## achievements, collections, solo challenges) built on the generated contract fixtures
## (res://tests/fixtures/contract/openapi, tools/gen_fixtures.mjs) with readable sample names for the scene tests and
## the review captures. Item codes are the F070 display resources (res://resources/items). Never shipped.

const ROUTE_PROGRESSION: String = "GET /game/progression"
const ROUTE_HISTORY: String = "GET /game/progression/xp-history"
const ROUTE_LEVELS: String = "GET /game/progression/levels"
const ROUTE_WALLET: String = "GET /game/wallet"
const ROUTE_LEDGER: String = "GET /game/wallet/ledger"
const ROUTE_ITEM_HISTORY: String = "GET /game/inventory/history"
const ROUTE_INVENTORY: String = "GET /game/inventory"
const ROUTE_SHOP: String = "GET /game/shop/items"
const ROUTE_PURCHASE: String = "POST /game/shop/items/*:purchase"
const ROUTE_ACHIEVEMENTS: String = "GET /game/achievements"
const ROUTE_COLLECTIONS: String = "GET /game/collections"
const ROUTE_COLLECTION: String = "GET /game/collections/*"
const ROUTE_CHALLENGES: String = "GET /game/challenges"
const ROUTE_CHALLENGE: String = "GET /game/challenges/*"
const ROUTE_START: String = "POST /game/challenges/*:start"
const ROUTE_ATTEMPT: String = "GET /game/challenge-attempts/*"
const ROUTE_NEXT: String = "POST /game/challenge-attempts/*/activities:next"
const ROUTE_ABANDON: String = "POST /game/challenge-attempts/*:abandon"
const SHOP_CHEAP: String = "06900000-0000-4000-8000-000000000001"
const SHOP_PRICEY: String = "06900000-0000-4000-8000-000000000002"
const SHOP_OWNED: String = "06900000-0000-4000-8000-000000000003"
const SHOP_LIMITED: String = "06900000-0000-4000-8000-000000000004"
const SHOP_SEASON: String = "06900000-0000-4000-8000-000000000005"
const CHALLENGE_TIME: String = "06900000-0000-4000-8000-0000000000c1"
const CHALLENGE_PARTY: String = "06900000-0000-4000-8000-0000000000c2"
const ATTEMPT_ID: String = "06900000-0000-4000-8000-0000000000a1"
const LEARNING_ATTEMPT_1: String = "06900000-0000-4000-8000-0000000000b1"
const LEARNING_ATTEMPT_2: String = "06900000-0000-4000-8000-0000000000b2"
const COLLECTION_ID: String = "06900000-0000-4000-8000-0000000000d1"
const HIDDEN_ACHIEVEMENT: String = "06900000-0000-4000-8000-0000000000e3"
## Server time of the fixtures (`Date` header) and a deadline 3 minutes later.
const SERVER_DATE: String = "Thu, 26 Nov 2026 07:30:00 GMT"
const STARTED_AT: String = "2026-11-26T07:30:00Z"
const DEADLINE_AT: String = "2026-11-26T07:33:00Z"


static func fixture(schema: String) -> Dictionary:
	return AuthHarness.fixture(schema)


static func loc(vi: String, en: String) -> Dictionary:
	return {"vi": vi, "en": en}


static func page(items: Array, next: String = "") -> Dictionary:
	return {"data": items, "page": {"has_more": not next.is_empty(), "next_cursor": next, "limit": 20}}


static func progression(level: int = 3, in_level: int = 40, to_next: Variant = 80, daily_left: int = 120) -> Dictionary:
	var p: Dictionary = fixture("ProgressionStatus")
	p["level"] = level
	p["title"] = {"code": "explorer_kid", "label": loc("Nhà thám hiểm nhí", "Little Explorer")}
	p["xp_total"] = 400 + in_level
	p["xp_in_level"] = in_level
	p["xp_to_next"] = to_next
	p["daily_xp_remaining"] = daily_left
	p["max_level"] = 50
	p["next_level_rewards"] = [{"kind": "currency", "amount": 50, "currency_code": "xu"}]
	return p


static func xp_update(delta: int, in_level: int, to_next: Variant = 80, capped: bool = false) -> Dictionary:
	return {"delta": delta, "xp_total": 400 + in_level, "xp_in_level": in_level, "xp_to_next": to_next,
		"daily_xp_remaining": 0 if capped else 80, "capped": capped,
		"source_label": loc("Hoàn thành bài học", "Finished an activity")}


static func level_up(from_level: int, to_level: int, with_item: bool = true) -> Dictionary:
	var rewards: Array = [{"kind": "currency", "amount": 50, "currency_code": "xu"}]
	if with_item:
		rewards.append({"kind": "item", "item_id": "acc_cap_star", "label": loc("Mũ ngôi sao", "Star cap")})
	return {"from_level": from_level, "to_level": to_level, "title_code": "explorer_kid",
		"title": loc("Nhà thám hiểm nhí", "Little Explorer"), "rewards": rewards}


static func history(count: int = 3, next: String = "") -> Dictionary:
	var codes: Array[String] = ["activity.completed", "quest.side_completed", "exploration.discovery", "unknown.code"]
	var out: Array = []
	for i: int in count:
		var e: Dictionary = fixture("XpLedgerEntry")
		e["amount"] = 20 + i * 5
		e["source_type"] = "activity"
		e["source_code"] = codes[i % codes.size()]
		e["source_label"] = "Bài học vui"
		e["capped"] = i == 2
		e["created_at"] = "2026-11-2%dT08:00:00Z" % (5 - (i % 5))
		out.append(e)
	return page(out, next)


static func levels() -> Dictionary:
	var out: Array = []
	for level: int in range(1, 6):
		out.append({"level": level, "xp_required": 100 + level * 20, "xp_cumulative": level * 100,
			"title_code": "explorer_kid" if level == 3 else "", "rewards": [{"kind": "currency", "amount": level * 10}]})
	return {"curve_version": 1, "max_level": 50, "levels": out}


static func wallet(balance: int = 150) -> Dictionary:
	var w: Dictionary = fixture("Wallet")
	w["currency_code"] = "xu"
	w["balance"] = balance
	w["daily_earn_remaining"] = 40
	return w


static func item(item_id: String, kind: String, rarity: String, vi: String, en: String) -> Dictionary:
	var i: Dictionary = fixture("InventoryItem")
	i["item_id"] = item_id
	i["kind"] = kind
	i["rarity"] = rarity
	i["name"] = loc(vi, en)
	i["equip_slot"] = "head" if kind == "accessory" else kind
	i["quantity"] = 1
	return i


static func inventory_items() -> Array:
	return [
		item("outfit_school_01", "outfit", "common", "Đồng phục học sinh", "School uniform"),
		item("outfit_explorer_01", "outfit", "rare", "Áo nhà thám hiểm", "Explorer jacket"),
		item("outfit_star_01", "outfit", "epic", "Áo ngôi sao", "Star shirt"),
		item("hair_bob_01", "hair", "common", "Tóc ngắn tròn", "Bob hair"),
		item("hair_braids_01", "hair", "uncommon", "Tóc tết", "Braids"),
		item("acc_cap_star", "accessory", "rare", "Mũ ngôi sao", "Star cap"),
		item("acc_glasses_round", "accessory", "uncommon", "Kính tròn", "Round glasses"),
		item("acc_backpack_owl", "accessory", "legendary", "Ba lô cú mèo", "Owl backpack"),
		item("emote_dance", "emote", "common", "Nhảy múa", "Dance"),
	]


## `n` items for the virtualisation / performance tests.
static func many_items(n: int) -> Array:
	var kinds: Array[String] = ["outfit", "hair", "accessory", "emote", "pet", "badge"]
	var rarities: Array[String] = ["common", "uncommon", "rare", "epic", "legendary"]
	var out: Array = []
	for i: int in n:
		out.append(item("item_%03d" % i, kinds[i % kinds.size()], rarities[i % rarities.size()], "Món %d" % i, "Item %d" % i))
	return out


static func shop_item(id: String, item_id: String, kind: String, rarity: String, price: int, vi: String, en: String) -> Dictionary:
	var s: Dictionary = fixture("ShopItem")
	s["shop_item_id"] = id
	s["item_id"] = item_id
	s["kind"] = kind
	s["rarity"] = rarity
	s["price"] = price
	s["currency_code"] = "xu"
	s["name"] = loc(vi, en)
	s["description"] = loc("Một món đồ đáng yêu cho nhân vật của em.", "A cute item for your character.")
	s["purchase_limit"] = 1
	s["purchased_count"] = 0
	s["owned"] = false
	s.erase("available_to")
	s.erase("thumbnail_url")
	return s


static func shop_items() -> Array:
	var limited: Dictionary = shop_item(SHOP_LIMITED, "emote_celebrate", "emote", "uncommon", 60, "Ăn mừng", "Celebrate")
	limited["purchase_limit"] = 2
	limited["purchased_count"] = 1
	var season: Dictionary = shop_item(SHOP_SEASON, "outfit_rain_01", "outfit", "epic", 140, "Áo mưa mùa thu",
			"Autumn raincoat")
	season["available_to"] = "2026-11-30T16:59:59Z"
	var owned: Dictionary = shop_item(SHOP_OWNED, "acc_glasses_round", "accessory", "uncommon", 80, "Kính tròn",
			"Round glasses")
	owned["owned"] = true
	owned["purchased_count"] = 1
	return [
		shop_item(SHOP_CHEAP, "acc_flag_small", "accessory", "common", 120, "Cờ nhỏ", "Little flag"),
		shop_item(SHOP_PRICEY, "outfit_sport_01", "outfit", "rare", 190, "Bộ đồ thể thao", "Sports outfit"),
		owned, limited, season,
	]


static func purchase_result(shop_item_id: String, item_id: String, balance_after: int) -> Dictionary:
	return {"shop_item_id": shop_item_id, "balance_after": balance_after,
		"item": item(item_id, "accessory", "common", "Cờ nhỏ", "Little flag")}


static func problem(code: String, status: int, extra: Dictionary = {}) -> Dictionary:
	var p: Dictionary = {"type": "https://errors.eduworld.vn/" + code.to_lower(), "title": code, "status": status,
		"code": code}
	p.merge(extra, true)
	return p


static func ledger() -> Dictionary:
	var rows: Array = []
	for spec: Array in [[50, 150, "reward"], [-120, 100, "shop_purchase"], [25, 220, "duplicate_conversion"]]:
		var e: Dictionary = fixture("CurrencyLedgerEntry")
		e["currency_code"] = "xu"
		e["delta"] = spec[0]
		e["balance_after"] = spec[1]
		e["reason_code"] = spec[2]
		e["source_label"] = "Nhiệm vụ"
		rows.append(e)
	return page(rows)


static func item_history() -> Dictionary:
	var e: Dictionary = fixture("ItemLedgerEntry")
	e["item_id"] = "acc_flag_small"
	e["item_name"] = "Cờ nhỏ"
	e["kind"] = "accessory"
	e["delta"] = 1
	e["reason_code"] = "shop_purchase"
	return page([e])


static func achievement(id: String, category: String, unlocked: bool, tier: String, current: float, target: float,
		vi: String, en: String) -> Dictionary:
	var a: Dictionary = fixture("AchievementProgress")
	a["achievement_id"] = id
	a["code"] = "ach_%s" % id.right(2)
	a["category"] = category
	a["unlocked"] = unlocked
	a["tier"] = tier
	a["progress_current"] = current
	a["progress_target"] = target
	a["title"] = loc(vi, en)
	a["description"] = loc("Khám phá các khu vực của Vương quốc Số.", "Explore the areas of the Number Kingdom.")
	a["rewards"] = [{"kind": "currency", "amount": 50}, {"kind": "badge", "item_id": "badge_explorer",
		"label": loc("Huy hiệu Nhà thám hiểm", "Explorer badge")}] if unlocked else [{"kind": "xp", "amount": 30}]
	return a


static func achievements() -> Dictionary:
	var hidden: Dictionary = achievement(HIDDEN_ACHIEVEMENT, "hidden", false, "none", 2, 5, "Bí mật của Mây",
			"Cloudy's secret")
	hidden["hidden"] = true
	return page([
		achievement("06900000-0000-4000-8000-0000000000e1", "exploration", false, "none", 7, 10,
			"Nhà thám hiểm Vương quốc Số", "Number Kingdom explorer"),
		achievement("06900000-0000-4000-8000-0000000000e2", "first_time", true, "gold", 1, 1, "Bạn đồng hành",
			"Good companion"),
		hidden,
	])


static func collections() -> Dictionary:
	var c: Dictionary = fixture("CollectionProgress")
	c["collection_id"] = COLLECTION_ID
	c["code"] = "pets_regions"
	c["title"] = loc("Thú cưng các vùng", "Pets of the regions")
	c["type"] = "pet"
	c["owned_count"] = 3
	c["total_count"] = 10
	c.erase("completed_at")
	return {"data": [c]}


static func collection_detail() -> Dictionary:
	var d: Dictionary = collections()["data"][0] as Dictionary
	var items: Array = []
	var names: Array[String] = ["Mèo Mây", "Cún Sao", "Thỏ Lá", "Cáo Lửa", "Rùa Biển", "Cú Đêm", "Gấu Tuyết", "Cá Vàng",
		"Ong Mật", "Nai Rừng"]
	for i: int in 10:
		var owned: bool = i < 3
		var entry: Dictionary = {"item_id": "pet_%02d" % i, "name": names[i], "rarity": ["common", "uncommon", "rare"][i % 3],
			"owned": owned}
		if not owned:
			entry["how_to_obtain"] = "Hoàn thành chuỗi nhiệm vụ Rừng Ngôn Ngữ"
		items.append(entry)
	d["items"] = items
	d["completion_rewards"] = [{"kind": "badge", "label": loc("Huy hiệu Bạn của muôn loài", "Friend of all creatures")}]
	return d


static func badges() -> Dictionary:
	return page([item("badge_explorer", "badge", "rare", "Huy hiệu Nhà thám hiểm", "Explorer badge"),
		item("badge_friend", "badge", "common", "Huy hiệu Bạn tốt", "Good friend badge"),
		item("badge_star", "badge", "epic", "Huy hiệu Ngôi sao", "Star badge"),
		item("badge_math", "badge", "uncommon", "Huy hiệu Toán học", "Math badge")])


static func challenge_summary(id: String, type: String, difficulties: Array, party: bool = false, attempts: int = 3,
		active: String = "") -> Dictionary:
	var s: Dictionary = fixture("GameChallengeSummary")
	s["challenge_id"] = id
	s["challenge_type"] = type
	s["title"] = loc("Đua với thời gian: Phép cộng", "Race the clock: Addition") if not party \
			else loc("Cùng nhóm hạ Rồng Số", "Team up against the Number Dragon")
	s["area_id"] = "challenge_mountain"
	s["available_difficulties"] = difficulties
	s["party_required"] = party
	s["attempts_left"] = attempts
	s.erase("available_to")
	if not active.is_empty():
		s["active_attempt_id"] = active
	return s


static func challenges(grade_band: String = "g1_3", attempts: int = 3, active: String = "") -> Dictionary:
	var levels_open: Array = ["easy", "normal", "hard"] if grade_band == "g1_3" else ["easy", "normal", "hard", "expert", "legendary"]
	return page([challenge_summary(CHALLENGE_TIME, "time", levels_open, false, attempts, active),
		challenge_summary(CHALLENGE_PARTY, "coop", ["normal"], true)])


static func challenge_detail(grade_band: String = "g1_3", attempts: int = 3, cooldown_until: String = "",
		with_locks: bool = false) -> Dictionary:
	var d: Dictionary = fixture("GameChallengeDetail")
	d.merge(challenge_summary(CHALLENGE_TIME, "time", challenges(grade_band, attempts)["data"][0]["available_difficulties"],
			false, attempts), true)
	d["description"] = loc("Trả lời thật nhiều phép cộng trước khi hết giờ!", "Answer as many additions as you can!")
	d["rules_summary"] = loc("Mỗi câu đúng được thêm điểm. Không bị trừ điểm.", "Each right answer adds points. No points lost.")
	d["time_limit_seconds"] = 180
	d["lives"] = 3
	d["first_clear_rewards"] = [{"kind": "xp", "amount": 60}, {"kind": "currency", "amount": 30},
		{"kind": "badge", "label": loc("Huy hiệu Tia chớp", "Lightning badge")}]
	if cooldown_until.is_empty():
		d.erase("cooldown_until")
	else:
		d["cooldown_until"] = cooldown_until
	if with_locks:
		d["difficulties"] = [
			{"difficulty": "easy", "available": true, "reward_multiplier": 1.0},
			{"difficulty": "normal", "available": true, "reward_multiplier": 1.2},
			{"difficulty": "hard", "available": true, "reward_multiplier": 1.5},
			{"difficulty": "expert", "available": false, "locked_reason_code": "grade_not_allowed"},
			{"difficulty": "legendary", "available": false},
		]
	return d


static func attempt(status: String = "in_progress", completed: int = 0, lives: int = 3, rewards: Array = []) -> Dictionary:
	var a: Dictionary = fixture("GameChallengeAttempt")
	a["challenge_attempt_id"] = ATTEMPT_ID
	a["challenge_id"] = CHALLENGE_TIME
	a["difficulty"] = "hard"
	a["status"] = status
	a["score"] = completed * 10
	a["lives_left"] = lives
	a["activities_completed"] = completed
	a["started_at"] = STARTED_AT
	a["deadline_at"] = DEADLINE_AT
	a.erase("party_id")
	a["rewards"] = rewards
	if status == "in_progress":
		a.erase("ended_at")
	return a


static func next_activity(learning_attempt_id: String, completed: int = 0, done: bool = false) -> Dictionary:
	var body: Dictionary = {"challenge_attempt": attempt("succeeded" if done else "in_progress", completed), "done": done}
	if not done:
		var learning: Dictionary = fixture("Attempt")
		learning["attempt_id"] = learning_attempt_id
		body["attempt"] = learning
	return body


## Scripts every F069 route on `http` (scene tests and review captures).
static func script_all(http: FakeHttp, grade_band: String = "g1_3") -> void:
	http.reply(ROUTE_PROGRESSION, 200, progression())
	http.reply(ROUTE_HISTORY, 200, history(4, "cursor-2"))
	http.reply(ROUTE_LEVELS, 200, levels())
	http.reply(ROUTE_WALLET, 200, wallet())
	http.reply(ROUTE_LEDGER, 200, ledger())
	http.reply(ROUTE_ITEM_HISTORY, 200, item_history())
	http.reply(ROUTE_INVENTORY, 200, page(inventory_items()))
	http.reply(ROUTE_SHOP, 200, page(shop_items()))
	http.reply(ROUTE_ACHIEVEMENTS, 200, achievements())
	http.reply(ROUTE_COLLECTION, 200, collection_detail())
	http.reply(ROUTE_COLLECTIONS, 200, collections())
	http.reply(ROUTE_CHALLENGE, 200, challenge_detail(grade_band), {"Date": SERVER_DATE})
	http.reply(ROUTE_CHALLENGES, 200, challenges(grade_band))
