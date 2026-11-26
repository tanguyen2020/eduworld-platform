extends RefCounted
## Recipes of "Núi Thử Thách" (Challenge Mountain, `challenge_mountain`; F094, WORLD_DESIGN §2.2 chapter 6 — boss Tinh
## Thể Tri Thức, §4.4 "challenge các loại và boss tổng hợp", §11; CHALLENGE_BOSS_SYSTEM "boss mất năng lượng, không
## máu"; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## A cool alpine plateau: the Crystal Peak behind the wind gate — a rocky spire with a stone stair and the floating
## Crystal of Knowledge in two light rings; snowy mountains on the horizon; the summit lookout (wooden deck, brass
## telescope, pennant) beside `discovery.mountain.summit_view`; the crystal cluster on a rock beside
## `collectible.mountain.crystal_shard`; the challenge circle (a round stone dais ringed by crystal pillars — the place
## of the friendly challenges, no weapons); stone stairs, cairns, pennants, pines and a mountain hut. No text (R2);
## colours are art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "mountain"
const THEME_OVERRIDES: Dictionary = {"lawn_alt": "hill", "path": "stone", "paving": "stone", "paving_dark": "blossom_violet",
	"curb": "stone_dark", "hedge": "stone_dark", "hedge_top": "stone", "far": "hill_far"}
const PLAZA: Vector3 = Vector3(0, 0, 5)
const PLAZA_RADIUS: float = 5.5
const SUMMIT: Vector3 = Vector3(0, 0, -18)
const LOOKOUT: Vector3 = Vector3(4.6, 0, -17.4)
const CRYSTALS: Vector3 = Vector3(12.8, 0, -12.4)
const CIRCLE: Vector3 = Vector3(-11.0, 0, -10.0)
const CIRCLE_RADIUS: float = 3.4
const HUT: Array = [Vector3(-17.5, 0, 15.5), 90.0, Vector2(5.0, 4.0)]
const LAYOUT: Dictionary = {
	"plaza": [PLAZA, PLAZA_RADIUS],
	"lanes": [[Vector3(0, 0, 0.5), Vector2(1.8, 23.5)], [Vector3(-1.0, 0, -10.0), Vector2(12.0, 1.5)]],
	"pads": [[Vector3(10, 0, -10), 3.0], [CIRCLE, CIRCLE_RADIUS + 0.4], [Vector3(0, 0, -19), 3.2]],
}
const BLOCKED: Array = [
	[LOOKOUT, Vector2(2.2, 2.2)],
	[CRYSTALS, Vector2(2.0, 2.0)],
	[CIRCLE, Vector2(4.0, 4.0)],
	[Vector3(-17.5, 0, 15.5), Vector2(3.2, 3.2)],
	[Vector3(3.0, 0, 11.0), Vector2(1.2, 1.2)],
	[Vector3(0, 0, -31), Vector2(12.0, 6.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_mountain_terrain_a": terrain, "env/env_mountain_far_a": far, "bld/bld_mountain_set_a": buildings,
		"prop/prop_mountain_set_a": props, "prop/prop_mountain_lookout_a": summit_lookout,
		"prop/prop_mountain_crystals_a": crystal_cluster, "prop/prop_mountain_challenge_circle_a": challenge_circle,
		"veg/veg_pine_a": ZoneKit.pine,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.lawn(m, th)
	ZoneKit.lanes(m, LAYOUT, "stone")
	# stair steps of the stone lane
	var z: float = -23.0
	while z < 23.5:
		if absf(z - PLAZA.z) > PLAZA_RADIUS + 0.3:
			ZoneKit.path_strip(m, Vector3(-1.8, 0, z), Vector3(1.8, 0, z + 0.12), "stone_dark", 0.05)
		z += 1.2
	ZoneKit.plaza(m, th, PLAZA, PLAZA_RADIUS)
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(1.4, 1.4, 1.0)), PLAZA + Vector3(0, 0.09, 0)))
	m.extrude(HubArt._icon_points("challenge_mountain"), 0.01, "stone_dark")
	m.pop()
	ZoneKit.pad(m, th, Vector3(10, 0, -10))
	ZoneKit.glow_ring(m, Vector3(10, 0, -10), 3.0)
	ZoneKit.pad(m, th, Vector3(0, 0, -19), 3.2)
	ZoneKit.glow_ring(m, SUMMIT, 2.6)
	# snow patches on the meadow
	for p: Vector3 in [Vector3(-19.0, 0, -18.0), Vector3(18.5, 0, 3.0), Vector3(-6.0, 0, 19.5), Vector3(19.0, 0, 20.0)]:
		HubArt._disk(m, p + Vector3.UP * 0.02, 1.6, 10, "eye_light")
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, a rise and a ring of snow-capped rocky mountains.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9461
	ZoneKit.far(m, theme(), rng, false)
	for side: int in 4:
		m.push_at(Vector3.ZERO, 90.0 * side)
		m.quad(Vector3(-32, 0, -32), Vector3(32, 0, -32), Vector3(70, 3.0, -70), Vector3(-70, 3.0, -70), "hill")
		m.pop()
	for i: int in 14:
		var a: float = TAU * (float(i) + rng.randf_range(-0.25, 0.25)) / 14.0
		var r: float = rng.randf_range(85.0, 120.0)
		var h: float = rng.randf_range(26.0, 46.0)
		var w: float = rng.randf_range(16.0, 24.0)
		var c: Vector3 = Vector3(cos(a) * r, -2.0, sin(a) * r)
		m.cylinder(c, w, w * 0.18, h, 9, "stone_dark" if i % 2 == 0 else "hill_far", "", false, rng.randf())
		m.cylinder(c + Vector3.UP * h * 0.68, w * 0.456, w * 0.19, h * 0.33, 9, "eye_light", "eye_light", true,
				rng.randf())
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_crystal_peak(m)
	m.push_at(HUT[0] as Vector3, float(HUT[1]))
	ZoneKit.house(m, HUT[2] as Vector2, "wood", "wood_dark", "stone_dark", "cloth_red")
	m.pop()
	return m.build()


## Crystal Peak: a jagged rocky spire (three faceted rock prisms) with snow on the shoulders, a stone stair to a ledge
## and the floating Crystal of Knowledge (a big violet crystal with pale satellites) inside two light rings; pennants
## at the foot.
static func _crystal_peak(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP + Vector3(0, 0, -2.0))
	m.box(Vector3(0, 0.3, 0.6), Vector3(13.0, 0.6, 7.0), "stone", "stone")
	# faceted rock prisms: [base, bottom radius, top radius, height, colour, phase]
	var rocks: Array = [[Vector3(0, 0.6, -0.8), 4.4, 1.3, 10.0, "stone", 0.1], [Vector3(-3.9, 0.6, 0.2), 2.6, 0.7, 6.4, "stone_dark", 0.4],
		[Vector3(3.7, 0.6, 0.0), 2.4, 0.6, 5.6, "stone_dark", 0.7]]
	for r: Array in rocks:
		var base: Vector3 = r[0] as Vector3
		var r0: float = float(r[1])
		var r1: float = float(r[2])
		var h: float = float(r[3])
		m.cylinder(base, r0, r1, h, 7, String(r[4]), "stone", true, float(r[5]))
		# snow cap on the upper third
		var rs: float = lerpf(r0, r1, 0.68) * 1.04
		m.cylinder(base + Vector3.UP * h * 0.68, rs, r1 * 1.04, h * 0.33, 7, "eye_light", "eye_light", true, float(r[5]))
	# stair to a ledge on the front
	for k: int in 7:
		m.box(Vector3(0, 0.75 + k * 0.4, 4.0 - k * 0.42), Vector3(2.2, 0.3, 0.5), "stone_dark", "stone")
	m.box(Vector3(0, 3.4, 1.2), Vector3(3.0, 0.3, 1.6), "stone", "stone")
	# the Crystal of Knowledge and its rings
	var c: Vector3 = Vector3(0, 13.2, -0.8)
	ZoneKit.crystal(m, c + Vector3.DOWN * 2.0, 1.4, 4.4, 6, "blossom_violet", 0.42)
	for k: int in 4:
		var a: float = TAU * float(k) / 4.0 + 0.4
		ZoneKit.crystal(m, c + Vector3(cos(a) * 2.6, -0.8 + 0.4 * k, sin(a) * 2.6), 0.4, 1.5, 6, "wind" if k % 2 == 0 else "spark")
	for rr: Array in [[2.2, 0.35], [2.9, -0.45]]:
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5 + float(rr[1])), c))
		HubArt._torus_ring(m, Vector3.ZERO, float(rr[0]), 0.08, 28, 5, "spark")
		m.pop()
	for x: float in [-5.6, 5.6]:
		ZoneKit.pennant(m, Vector3(x, 0.6, 3.6), "blossom_violet" if x < 0.0 else "cloth_blue", 3.8, -90.0 if x < 0.0 else 90.0)
	m.pop()


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Summit lookout (beside discovery.mountain.summit_view): a wooden deck with a railing, a brass telescope on a tripod
## looking north over the peaks, a pennant and a little bench.
static func summit_lookout() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.2, 0), Vector3(3.2, 0.4, 3.2), "wood", "wood")
	for k: int in 6:
		m.box(Vector3(-1.35 + k * 0.54, 0.41, 0), Vector3(0.04, 0.02, 3.1), "wood_dark")
	for x: float in [-1.5, -0.5, 0.5, 1.5]:
		for z: float in [-1.5]:
			m.cylinder(Vector3(x, 0.4, z), 0.06, 0.06, 0.9, 6, "wood_dark")
	for x: float in [-1.5, 1.5]:
		for z: float in [-0.5, 0.5]:
			m.cylinder(Vector3(x, 0.4, z), 0.06, 0.06, 0.9, 6, "wood_dark")
	m.box(Vector3(0, 1.3, -1.5), Vector3(3.1, 0.1, 0.1), "wood")
	for x: float in [-1.5, 1.5]:
		m.box(Vector3(x, 1.3, -0.5), Vector3(0.1, 0.1, 2.1), "wood")
	# telescope on a tripod
	var t: Vector3 = Vector3(0.3, 0.4, -0.7)
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0
		HubArt._limb(m, t + Vector3(cos(a) * 0.4, 0, sin(a) * 0.4), t + Vector3(0, 1.1, 0), 0.03, 0.03, "wood_dark")
	m.push(Transform3D(Basis(Vector3.RIGHT, -1.25) * Basis.IDENTITY, t + Vector3(0, 1.15, 0)))
	m.cylinder(Vector3(0, -0.5, 0), 0.08, 0.12, 1.1, 10, "bronze")
	m.cylinder(Vector3(0, 0.6, 0), 0.14, 0.14, 0.12, 10, "bronze_patina")
	m.pop()
	ZoneKit.pennant(m, Vector3(1.3, 0.4, 1.2), "cloth_red", 2.8, 0.0)
	ZoneKit.bench(m, Vector3(-0.7, 0.4, 0.9), 0.0)
	# steps down to the lane side
	m.box(Vector3(-1.9, 0.12, 0.4), Vector3(0.6, 0.24, 1.6), "stone", "stone")
	return m.build()


## Crystal cluster on a rock (beside collectible.mountain.crystal_shard): violet, pale and gold crystals with sparkles.
static func crystal_cluster() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.push_at(Vector3.ZERO, 0.0, Vector3(1.45, 1.45, 1.45))
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9465
	m.blob(Vector3(0, 0.35, 0), Vector3(1.5, 0.7, 1.3), "stone_dark", 1, 0.1, rng)
	var spec: Array = [[Vector3(0, 0.6, 0), 0.45, 2.4, "blossom_violet", 0.0], [Vector3(0.6, 0.5, 0.3), 0.3, 1.5, "wind", 0.35],
		[Vector3(-0.55, 0.5, 0.25), 0.32, 1.7, "blossom_violet", -0.3], [Vector3(0.2, 0.5, -0.6), 0.28, 1.3, "spark", -0.25],
		[Vector3(-0.3, 0.45, -0.5), 0.22, 1.0, "wind", 0.3]]
	for s: Array in spec:
		m.push(Transform3D(Basis(Vector3.BACK, float(s[4])) * Basis(Vector3.RIGHT, float(s[4]) * 0.6), s[0] as Vector3))
		ZoneKit.crystal(m, Vector3.ZERO, float(s[1]), float(s[2]), 6, String(s[3]), 0.3)
		m.pop()
	for k: int in 4:
		var a: float = TAU * float(k) / 4.0
		m.blob(Vector3(cos(a) * 1.1, 1.8 + 0.3 * k, sin(a) * 1.0), Vector3(0.07, 0.07, 0.07), "spark", 0)
	m.pop()
	return m.build()


## Challenge circle: a low round stone dais with a violet ring and a star in the middle, ringed by six crystal pillars
## (gaps east and west for the path).
static func challenge_circle() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var r: float = CIRCLE_RADIUS
	m.cylinder(Vector3.ZERO, r + 0.2, r + 0.1, 0.14, 28, "stone", "stone")
	HubArt._annulus(m, Vector3.UP * 0.145, r - 0.9, r - 0.5, 28, "blossom_violet")
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(0.9, 0.9, 1.0)), Vector3.UP * 0.15))
	m.extrude(HubArt._star(1.0, 0.45), 0.02, "lantern_glow")
	m.pop()
	for k: int in 6:
		var a: float = deg_to_rad(30.0 + 60.0 * float(k))
		var p: Vector3 = Vector3(cos(a), 0, sin(a)) * r
		m.cylinder(p, 0.42, 0.36, 0.5, 8, "stone_dark", "stone")
		ZoneKit.crystal(m, p + Vector3.UP * 0.4, 0.3, 1.9, 6, "blossom_violet" if k % 2 == 0 else "wind", 0.3)
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9463
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = PLAZA + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (PLAZA_RADIUS - 0.6)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "wind")
	for b: Array in [[Vector3(-4.2, 0.08, 10.0), 40.0], [Vector3(4.2, 0.08, 10.0), -40.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	ZoneKit.signpost(m, Vector3(3.0, 0, 11.0), [[Vector3(0, 0, -20), "home", "roof"], [SUMMIT, "peak", "stone_dark"],
		[Vector3(10, 0, -10), "star", "blossom_violet"], [CIRCLE, "star", "cloth_red"]])
	# pennants along the stair lane in challenge colours
	var colours: PackedStringArray = ["cloth_red", "cloth_blue", "wall_yellow", "cloth_green"]
	var i: int = 0
	for z: float in [-3.0, -14.0]:
		for s: float in [-1.0, 1.0]:
			ZoneKit.pennant(m, Vector3(s * 2.6, 0, z), colours[i % colours.size()], 3.0, -90.0 if s < 0.0 else 90.0)
			i += 1
	for p: Vector3 in [Vector3(-6.0, 0, -15.0), Vector3(7.5, 0, -4.0), Vector3(-17.0, 0, 2.0), Vector3(16.5, 0, 9.0), Vector3(-3.6, 0, -21.6),
			Vector3(3.6, 0, -21.6)]:
		ZoneKit.cairn(m, p, rng, 4 if p.z > -20.0 else 5)
	for p: Vector3 in [Vector3(-21.0, 0, -8.0), Vector3(21.0, 0, -15.0), Vector3(20.5, 0, 14.0), Vector3(-21.5, 0, 21.0), Vector3(8.0, 0, 20.5)]:
		ZoneKit.rock(m, p, Vector3(rng.randf_range(0.9, 1.4), rng.randf_range(0.6, 1.0), rng.randf_range(0.8, 1.2)), "stone_dark", rng)
	# small crystals sprouting from the meadow (hint of the zone's treasure)
	for p: Vector3 in [Vector3(-8.0, 0, 3.0), Vector3(8.5, 0, 14.0), Vector3(-14.0, 0, -18.0), Vector3(16.0, 0, -20.0)]:
		ZoneKit.crystal(m, p, 0.25, 0.9, 6, "blossom_violet", 0.3)
		ZoneKit.crystal(m, p + Vector3(0.3, 0, 0.2), 0.16, 0.6, 6, "wind", 0.3)
	return m.build()


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"pine": "veg_pine_a", "bush": "veg_bush_a", "flower": "veg_flower_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"pine":
			return ZoneKit.scatter_in(9470, 16, 26, 1.6, LAYOUT, BLOCKED, 0.8, 1.3)
		"bush":
			return ZoneKit.scatter_in(9473, 24, 0, 1.0, LAYOUT, BLOCKED, 0.7, 1.1)
		"flower":
			return ZoneKit.scatter_in(9474, 120, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter_in(9475, 240, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.4)
	return []
