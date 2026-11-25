extends RefCounted
## Recipes of "Đảo Khoa Học" (Science Island, `learning_zone.science`; F093, WORLD_DESIGN §2.2 chapter 3 "phòng thí
## nghiệm, vòng đời sinh vật, thời tiết", §4.2, §11; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party,
## nothing downloaded.
## An island in the sea (sand beach, palms, islets, a friendly smoking volcano on the horizon); set pieces: the glass
## greenhouse lab with a telescope dome, solar panels and a wind turbine behind the wind gate, the weather station
## with an anemometer, a wind vane, a louvred screen and a rain gauge (discovery.science.weather_station), the rocky
## tide pool with starfish and shells (discovery.science.tide_pool), the Fossil Gate of two standing slabs with leaf
## and ammonite imprints over the lane (collectible.science.leaf_fossil), a plant life-cycle row of pots, a beehive and
## a telescope. Colours are art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "science"
const THEME_OVERRIDES: Dictionary = {"paving_dark": "subject_science", "hedge": "leaf_dark", "hedge_top": "leaf_light",
	"far": "water"}
const STATION_CENTER: Vector3 = Vector3(16.2, 0, -6)
const POOL_CENTER: Vector3 = Vector3(-16.4, 0, -6)
const POOL_RADIUS: float = 2.3
const SLAB_X: float = 2.9
const ISLAND_RADIUS: float = 52.0
const BLOCKED: Array = [
	[STATION_CENTER, Vector2(2.6, 2.6)],
	[POOL_CENTER, Vector2(3.2, 3.2)],
	[Vector3(-18.0, 0, 16.0), Vector2(3.2, 3.2)],
	[Vector3(8.5, 0, 12.5), Vector2(3.6, 1.4)],
	[Vector3(0, 0, -31), Vector2(12.0, 5.0)],
]
const HOUSE: Array = [Vector3(-18.0, 0, 16.0), 90.0, Vector2(5.0, 4.0)]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_science_terrain_a": terrain, "env/env_science_far_a": far, "bld/bld_science_set_a": buildings, "prop/prop_science_set_a": props,
		"prop/prop_science_weather_a": weather_station, "prop/prop_science_tide_pool_a": tide_pool,
		"prop/prop_science_fossil_gate_a": fossil_gate, "veg/veg_palm_coconut_a": coconut_palm,
	}


# ---------- terrain ----------

## Island: the shared lawn and paths, a sand beach sloping into the sea, a deeper sea ring, islets and a volcano.
static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.terrain(m, th)
	ZoneKit.path_strip(m, Vector3(15.0, 0, -7.2), Vector3(17.0, 0, -4.8), "path")
	_beach(m)
	ZoneKit.boundary(m, th)
	return m.build()


static func _beach(m: LowPoly) -> void:
	var segs: int = 40
	for i: int in segs:
		var a0: float = TAU * float(i) / float(segs)
		var a1: float = TAU * float(i + 1) / float(segs)
		var d0: Vector3 = Vector3(cos(a0), 0, sin(a0))
		var d1: Vector3 = Vector3(cos(a1), 0, sin(a1))
		# inner edge: the 64 m lawn square; then sand down to the waterline and the shallow sea
		var i0: Vector3 = d0 * (32.0 / maxf(absf(d0.x), absf(d0.z)))
		var i1: Vector3 = d1 * (32.0 / maxf(absf(d1.x), absf(d1.z)))
		var o0: Vector3 = d0 * ISLAND_RADIUS + Vector3.DOWN * 0.03
		var o1: Vector3 = d1 * ISLAND_RADIUS + Vector3.DOWN * 0.03
		m.quad(i0 + Vector3.UP * 0.002, i1 + Vector3.UP * 0.002, o1, o0, "path")
		var w0: Vector3 = d0 * (ISLAND_RADIUS + 9.0) + Vector3.DOWN * 0.04
		var w1: Vector3 = d1 * (ISLAND_RADIUS + 9.0) + Vector3.DOWN * 0.04
		m.quad(o0 + Vector3.UP * 0.005, o1 + Vector3.UP * 0.005, w1, w0, "water")
		var f0: Vector3 = d0 * (ISLAND_RADIUS + 0.9) + Vector3.UP * 0.0
		var f1: Vector3 = d1 * (ISLAND_RADIUS + 0.9) + Vector3.UP * 0.0
		m.quad(o0 + Vector3.UP * 0.01, o1 + Vector3.UP * 0.01, f1, f0, "flower_white")


## Far scenery (no outline, no shadow): the sea with a deeper ring, islets and the friendly volcano.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9369
	ZoneKit.far(m, theme(), rng, false)
	HubArt._annulus(m, Vector3(0, -0.045, 0), 70.0, 168.0, 24, "water_deep")
	for i: int in 5:
		var a: float = TAU * (float(i) + 0.3) / 5.0 + 0.4
		var r: float = rng.randf_range(80.0, 110.0)
		var c: Vector3 = Vector3(cos(a) * r, -0.5, sin(a) * r)
		HubArt._disk(m, c + Vector3.UP * 0.47, rng.randf_range(9.0, 13.0), 12, "path")
		m.blob(c, Vector3(rng.randf_range(6.0, 9.0), rng.randf_range(3.0, 6.0), rng.randf_range(6.0, 8.0)), "hill", 2, 0.04, rng)
	# friendly volcano on the horizon (no lava: a warm glow and a soft smoke puff)
	var v: Vector3 = Vector3(78, -1.0, -135)
	m.blob(v + Vector3.UP * 1.0, Vector3(24.0, 5.0, 20.0), "hill", 2, 0.04, rng)
	m.cylinder(v, 17.0, 4.0, 15.0, 16, "soil", "soil", true)
	m.cylinder(v + Vector3.UP * 14.4, 3.6, 3.6, 0.5, 16, "lantern_glow", "lantern_glow", true)
	for k: int in 2:
		m.blob(v + Vector3(k * 2.5, 18.5 + k * 3.0, -k * 1.0), Vector3(2.6, 1.8, 2.6) * (1.0 + k * 0.35), "flower_white", 1, 0.05, rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_greenhouse_lab(m)
	m.push_at(HOUSE[0] as Vector3, float(HOUSE[1]))
	ZoneKit.house(m, HOUSE[2] as Vector2, "subject_science", "leaf_dark", "subject_science", "lantern_glow")
	m.pop()
	return m.build()


## Glass greenhouse lab: a big glass dome with green ribs on a cream base and a porch, a white observatory with a
## telescope slot, solar panels and a three-bladed wind turbine.
static func _greenhouse_lab(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 0.3, 0), Vector3(18.0, 0.6, 7.0), "stone", "stone")
	m.cylinder(Vector3(0, 0.6, 0), 4.6, 4.6, 0.9, 20, "wall", "", false)
	m.blob(Vector3(0, 1.5, 0), Vector3(4.5, 4.2, 4.5), "wind", 2)
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		var prev: Vector3 = Vector3.ZERO
		for j: int in 7:
			var t: float = PI * 0.5 * float(j) / 6.0
			var p: Vector3 = Vector3(cos(a) * cos(t) * 4.56, 1.5 + sin(t) * 4.26, sin(a) * cos(t) * 4.56)
			if j > 0:
				HubArt._limb(m, prev, p, 0.09, 0.09, "subject_science")
			prev = p
	for y: float in [3.2, 4.8]:
		var rr: float = 4.56 * cos(asin(clampf((y - 1.5) / 4.26, -1.0, 1.0)))
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, y, 0)))
		HubArt._torus_ring(m, Vector3.ZERO, rr, 0.08, 24, 4, "subject_science")
		m.pop()
	m.blob(Vector3(0, 5.9, 0), Vector3(0.5, 0.35, 0.5), "subject_science", 1)
	# porch
	m.box(Vector3(0, 1.6, 4.6), Vector3(2.6, 2.0, 1.6), "wall")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.6, 5.42)))
	m.extrude(ZoneKit.arch(0.6, 1.7, 8), 0.08, "wood_dark")
	m.pop()
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, 2.65, 4.6)))
	m.cylinder(Vector3(0, -0.8, 0), 1.3, 1.3, 1.6, 12, "subject_science", "", false)
	m.pop()
	# plants peeking under the glass
	for p: Vector3 in [Vector3(-2.0, 2.0, 1.5), Vector3(1.6, 2.2, 1.8), Vector3(0.2, 2.6, -1.0)]:
		m.blob(p, Vector3(1.0, 0.9, 1.0), "leaf", 1)
	# observatory
	m.push_at(Vector3(-7.2, 0.6, 0.2), 0.0)
	m.cylinder(Vector3.ZERO, 1.9, 1.9, 3.2, 16, "flower_white")
	m.cylinder(Vector3(0, 0.3, 0), 1.95, 1.95, 0.3, 16, "subject_science", "", false)
	m.blob(Vector3(0, 3.2, 0), Vector3(1.95, 1.7, 1.95), "metal", 1)
	m.push(Transform3D(Basis(Vector3.RIGHT, -0.5), Vector3(0, 3.6, 0.9)))
	m.box(Vector3.ZERO, Vector3(0.55, 1.6, 0.4), "metal_dark")
	m.pop()
	m.push(Transform3D(Basis(Vector3.RIGHT, -0.9), Vector3(0, 4.1, 1.0)))
	m.cylinder(Vector3.ZERO, 0.2, 0.26, 1.6, 10, "subject_science")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.0, 1.86)))
	m.extrude(ZoneKit.arch(0.5, 1.7, 8), 0.08, "wood_dark")
	m.pop()
	m.pop()
	# solar panels
	for x: float in [5.6, 7.6]:
		m.push(Transform3D(Basis(Vector3.RIGHT, -0.6), Vector3(x, 1.5, 1.4)))
		m.box(Vector3.ZERO, Vector3(1.8, 0.08, 1.3), "metal_dark")
		m.box(Vector3(0, 0.05, 0), Vector3(1.6, 0.04, 1.1), "cloth_blue")
		m.pop()
		m.cylinder(Vector3(x, 0.6, 1.4), 0.08, 0.08, 0.9, 8, "metal")
	# wind turbine
	var hub: Vector3 = Vector3(9.4, 9.0, -1.0)
	m.cylinder(Vector3(9.4, 0.6, -1.0), 0.35, 0.2, 8.4, 12, "flower_white")
	m.blob(hub, Vector3(0.45, 0.4, 0.75), "flower_white", 1)
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0 + 0.3
		m.push(Transform3D(Basis(Vector3.BACK, a), hub + Vector3(0, 0, 0.55)))
		m.box(Vector3(0, 1.8, 0), Vector3(0.42, 3.4, 0.08), "flower_white")
		m.box(Vector3(0, 3.3, 0.01), Vector3(0.36, 0.4, 0.08), "subject_science")
		m.pop()
	m.pop()


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Weather station (discovery.science.weather_station): a lattice mast with an anemometer and a wind vane, a white
## louvred instrument screen on legs, a rain gauge and a small solar panel on a stone pad.
static func weather_station() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.1, 0), Vector3(3.6, 0.2, 3.6), "stone", "stone")
	var legs: Array[Vector3] = [Vector3(-0.45, 0, -0.45), Vector3(0.45, 0, -0.45), Vector3(0.45, 0, 0.45), Vector3(-0.45, 0, 0.45)]
	var top: float = 4.2
	for i: int in 4:
		var a: Vector3 = legs[i] + Vector3(0.9 * signf(legs[i].x) * 0.3, 0.2, 0.9 * signf(legs[i].z) * 0.3) + Vector3(-0.6, 0, -0.6)
		var b: Vector3 = legs[i] * 0.4 + Vector3(-0.6, top, -0.6)
		HubArt._limb(m, a, b, 0.06, 0.05, "metal")
		var n: Vector3 = legs[(i + 1) % 4]
		var an: Vector3 = n + Vector3(0.9 * signf(n.x) * 0.3, 0.2, 0.9 * signf(n.z) * 0.3) + Vector3(-0.6, 0, -0.6)
		var bn: Vector3 = n * 0.4 + Vector3(-0.6, top, -0.6)
		for t: float in [0.25, 0.55]:
			HubArt._limb(m, a.lerp(b, t), an.lerp(bn, t + 0.2), 0.03, 0.03, "metal_dark")
	var mast: Vector3 = Vector3(-0.6, top, -0.6)
	m.cylinder(mast, 0.05, 0.05, 1.0, 8, "metal_dark")
	# anemometer: three arms with cups
	m.cylinder(mast + Vector3.UP * 1.0, 0.1, 0.1, 0.12, 8, "cloth_red")
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		HubArt._limb(m, mast + Vector3.UP * 1.06, mast + Vector3.UP * 1.06 + d * 0.55, 0.025, 0.025, "metal")
		m.blob(mast + Vector3.UP * 1.06 + d * 0.6, Vector3(0.12, 0.1, 0.12), "flower_white", 0)
	# wind vane with an arrow
	m.cylinder(mast + Vector3.UP * 0.55, 0.03, 0.03, 0.2, 6, "metal_dark")
	m.push(Transform3D(Basis(Vector3.UP, 0.6), mast + Vector3.UP * 0.7))
	m.extrude(PackedVector2Array([Vector2(-0.6, -0.03), Vector2(0.3, -0.03), Vector2(0.3, -0.12), Vector2(0.55, 0.0),
		Vector2(0.3, 0.12), Vector2(0.3, 0.03), Vector2(-0.6, 0.03), Vector2(-0.75, 0.16), Vector2(-0.75, -0.16)]), 0.04, "cloth_red")
	m.pop()
	# louvred screen (Stevenson screen)
	var sc: Vector3 = Vector3(0.75, 0, 0.6)
	for x: float in [-0.35, 0.35]:
		for z: float in [-0.3, 0.3]:
			m.cylinder(sc + Vector3(x, 0.2, z), 0.04, 0.04, 1.0, 6, "flower_white")
	m.box(sc + Vector3(0, 1.55, 0), Vector3(0.9, 0.75, 0.75), "flower_white")
	for k: int in 4:
		m.box(sc + Vector3(0, 1.3 + k * 0.16, 0.39), Vector3(0.8, 0.05, 0.04), "metal")
	m.push_at(sc + Vector3(0, 2.0, 0), 90.0)
	m.tiled_roof(Vector3.ZERO, 1.1, 1.0, 0.25, 0.12, 0.05, "flower_white", "subject_science")
	m.pop()
	# rain gauge and solar panel
	m.cylinder(Vector3(1.2, 0.2, -0.9), 0.18, 0.18, 0.8, 12, "metal", "metal_dark")
	m.cylinder(Vector3(1.2, 1.0, -0.9), 0.24, 0.18, 0.15, 12, "metal_dark", "", false)
	m.push(Transform3D(Basis(Vector3.RIGHT, -0.7), Vector3(-0.6, 2.3, 0.15)))
	m.box(Vector3.ZERO, Vector3(0.9, 0.06, 0.6), "metal_dark")
	m.box(Vector3(0, 0.04, 0), Vector3(0.8, 0.03, 0.5), "cloth_blue")
	m.pop()
	return m.build()


## Rocky tide pool (discovery.science.tide_pool at its east rim): a ring of rounded rocks around shallow water with
## starfish, shells, sea anemones and pebbles.
static func tide_pool() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9362
	HubArt._disk(m, Vector3(0, 0.03, 0), POOL_RADIUS + 0.9, 18, "path")
	HubArt._disk(m, Vector3(0, 0.12, 0), POOL_RADIUS, 18, "water")
	HubArt._annulus(m, Vector3(0, 0.125, 0), POOL_RADIUS * 0.45, POOL_RADIUS * 0.75, 18, "water_deep")
	for k: int in 11:
		var a: float = TAU * float(k) / 11.0 + rng.randf_range(-0.1, 0.1)
		var r: float = POOL_RADIUS + rng.randf_range(0.15, 0.45)
		m.blob(Vector3(cos(a) * r, 0.25, sin(a) * r), Vector3(rng.randf_range(0.5, 0.75), rng.randf_range(0.35, 0.7), rng.randf_range(0.45, 0.65)),
				"stone_dark" if k % 3 == 0 else "stone", 1, 0.1, rng)
	m.blob(Vector3(-1.8, 0.6, -1.5), Vector3(1.0, 0.9, 0.9), "stone", 1, 0.08, rng)
	m.blob(Vector3(-0.8, 0.45, -2.3), Vector3(0.8, 0.6, 0.7), "stone_dark", 1, 0.08, rng)
	# starfish lying in the water and on the rocks
	for s: Array in [[Vector3(0.6, 0.14, 0.4), "cloth_red", 0.4], [Vector3(-0.7, 0.14, -0.3), "subject_vietnamese", 1.2],
			[Vector3(1.9, 0.62, -1.0), "flower_pink", 0.2]]:
		m.push(Transform3D(Basis(Vector3.UP, float(s[2])) * Basis(Vector3.RIGHT, -PI * 0.5), s[0] as Vector3))
		m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.3, 0.3, 1.0)), Vector3.ZERO))
		m.extrude(HubArt._star(1.0, 0.45), 0.08, String(s[1]))
		m.pop()
		m.pop()
	# shells and anemones
	for p: Vector3 in [Vector3(0.0, 0.16, -0.9), Vector3(-1.2, 0.16, 0.8), Vector3(2.6, 0.06, 1.2)]:
		m.lathe(p, PackedVector2Array([Vector2(0.16, 0.0), Vector2(0.12, 0.08), Vector2(0.06, 0.16), Vector2(0.0, 0.2)]), 8, "flower_white")
	for p: Vector3 in [Vector3(0.9, 0.12, -0.6), Vector3(-0.2, 0.12, 1.0)]:
		m.cylinder(p, 0.12, 0.1, 0.22, 8, "flower_pink")
		for k: int in 5:
			var a: float = TAU * float(k) / 5.0
			m.blob(p + Vector3(cos(a) * 0.1, 0.28, sin(a) * 0.1), Vector3(0.05, 0.08, 0.05), "lantern_glow", 0)
	return m.build()


## Fossil Gate over the lane (collectible.science.leaf_fossil under it): two standing stone slabs and a capstone with
## leaf and ammonite imprints.
static func fossil_gate() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * SLAB_X, 0.15, 0), Vector3(1.2, 0.3, 1.4), "stone_dark", "stone_dark")
		m.push(Transform3D(Basis(Vector3.BACK, s * 0.05), Vector3(s * SLAB_X, 1.75, 0)))
		m.box(Vector3.ZERO, Vector3(0.8, 3.2, 1.1), "stone")
		m.pop()
	m.box(Vector3(0, 3.55, 0), Vector3(2.0 * SLAB_X + 1.6, 0.6, 1.3), "stone", "stone")
	# imprints: a leaf on each slab, an ammonite spiral on the capstone (front face)
	for s: float in [-1.0, 1.0]:
		m.push(Transform3D(Basis(Vector3.BACK, s * 0.3).scaled(Vector3(0.42, 0.42, 1.0)), Vector3(s * SLAB_X, 2.0, 0.56)))
		m.extrude(HubArt._icon_points("practice_zone"), 0.04, "stone_dark")
		m.pop()
		m.push(Transform3D(Basis(Vector3.BACK, s * 0.3), Vector3(s * SLAB_X, 2.0, 0.59)))
		m.box(Vector3.ZERO, Vector3(0.03, 0.75, 0.02), "stone")
		m.pop()
	var prev: Vector3 = Vector3(0, 3.55, 0.66)
	for i: int in 22:
		var a: float = float(i) * 0.55
		var r: float = 0.05 + float(i) * 0.018
		var p: Vector3 = Vector3(cos(a) * r, 3.55 + sin(a) * r * 0.8, 0.66)
		if i > 0:
			HubArt._limb(m, prev, p, 0.03, 0.03, "stone_dark")
		prev = p
	# moss and little flowers on the capstone
	m.blob(Vector3(-2.4, 3.9, 0.1), Vector3(0.8, 0.22, 0.6), "leaf", 1)
	m.blob(Vector3(2.0, 3.88, -0.2), Vector3(0.6, 0.2, 0.5), "leaf_light", 1)
	m.blob(Vector3(-2.2, 4.08, 0.2), Vector3(0.1, 0.08, 0.1), "flower_white", 0)
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9363
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = ZoneKit.PLAZA_CENTER + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (ZoneKit.PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "metal_dark", "subject_science")
	for z: float in [-10.0, -21.5]:
		for s: float in [-1.0, 1.0]:
			ZoneKit.lamp(m, Vector3(s * 3.1, 0, z), "metal_dark", "subject_science")
	for b: Array in [[Vector3(-4.6, 0.08, 6.6), 50.0], [Vector3(4.6, 0.08, 6.6), -50.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	_life_cycle(m, Vector3(8.5, 0, 12.5), ZoneKit.yaw_to(Vector3(8.5, 0, 12.5), ZoneKit.PLAZA_CENTER))
	_telescope(m, Vector3(-5.4, 0.0, -1.6), ZoneKit.yaw_to(Vector3(-5.4, 0, -1.6), Vector3(-20, 0, -30)))
	_beehive(m, Vector3(5.8, 0, -2.2))
	ZoneKit.flower_bed(m, Vector3(-7.5, 0, -12.0), 1.4, PackedStringArray(["lantern_glow", "flower_pink", "flower_white"]), rng)
	ZoneKit.flower_bed(m, Vector3(7.5, 0, -12.0), 1.4, PackedStringArray(["subject_science", "lantern_glow", "flower_white"]), rng)
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, 21.5), Vector3(21.8, 0, -21.6), Vector3(-9.0, 0, -21.8)]:
		m.blob(p + Vector3(0, 0.2, 0), Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)),
				"stone_dark", 0, 0.15, rng)
	# beach rocks and a little boat on the sand outside the ring
	for i: int in 10:
		var a: float = TAU * float(i) / 10.0 + 0.2
		var p: Vector3 = Vector3(cos(a), 0, sin(a)) * rng.randf_range(36.0, 42.0)
		m.blob(p + Vector3(0, 0.15, 0), Vector3(rng.randf_range(0.8, 1.4), rng.randf_range(0.4, 0.8), rng.randf_range(0.7, 1.2)),
				"stone", 0, 0.12, rng)
	return m.build()


## Plant life cycle: five pots in a row — seed, sprout, young plant, bud, sunflower — growing taller (no text).
static func _life_cycle(m: LowPoly, c: Vector3, yaw: float) -> void:
	m.push_at(c, yaw)
	m.box(Vector3(0, 0.2, 0), Vector3(6.6, 0.4, 1.1), "wood", "wood_dark")
	for i: int in 5:
		var p: Vector3 = Vector3(-2.6 + i * 1.3, 0.4, 0)
		m.cylinder(p, 0.32, 0.4, 0.5, 10, "roof", "soil")
		var top: Vector3 = p + Vector3.UP * 0.5
		match i:
			0:
				m.blob(top + Vector3.UP * 0.06, Vector3(0.1, 0.07, 0.08), "trunk", 0)
			1:
				m.cylinder(top, 0.03, 0.03, 0.25, 6, "leaf")
				m.blob(top + Vector3(0.08, 0.27, 0), Vector3(0.1, 0.04, 0.06), "leaf_light", 0)
			2:
				m.cylinder(top, 0.04, 0.035, 0.6, 6, "leaf")
				for s: float in [-1.0, 1.0]:
					m.blob(top + Vector3(s * 0.15, 0.4, 0), Vector3(0.16, 0.06, 0.1), "leaf_light", 0)
			3:
				m.cylinder(top, 0.05, 0.04, 1.0, 6, "leaf")
				m.blob(top + Vector3(0, 1.05, 0), Vector3(0.14, 0.16, 0.14), "leaf_dark", 0)
				for s: float in [-1.0, 1.0]:
					m.blob(top + Vector3(s * 0.18, 0.55, 0), Vector3(0.18, 0.06, 0.12), "leaf", 0)
			_:
				m.cylinder(top, 0.06, 0.05, 1.4, 6, "leaf")
				m.push(Transform3D(Basis(Vector3.RIGHT, -0.3), top + Vector3(0, 1.5, 0.05)))
				m.extrude(HubArt._star(0.45, 0.6), 0.06, "wall_yellow")
				m.extrude(ZoneKit.circle(0.2, 10), 0.1, "trunk")
				m.pop()
				for s: float in [-1.0, 1.0]:
					m.blob(top + Vector3(s * 0.22, 0.7, 0), Vector3(0.22, 0.07, 0.14), "leaf", 0)
	m.pop()


static func _telescope(m: LowPoly, p: Vector3, yaw: float) -> void:
	m.push_at(p, yaw)
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0
		HubArt._limb(m, Vector3(cos(a) * 0.5, 0, sin(a) * 0.5), Vector3(0, 1.2, 0), 0.04, 0.03, "wood_dark")
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5 - 0.6), Vector3(0, 1.3, 0)))
	m.cylinder(Vector3(0, -0.5, 0), 0.12, 0.16, 1.4, 10, "subject_science", "flower_white")
	m.cylinder(Vector3(0, -0.6, 0), 0.08, 0.08, 0.15, 8, "metal_dark")
	m.pop()
	m.pop()


static func _beehive(m: LowPoly, p: Vector3) -> void:
	m.box(p + Vector3(0, 0.3, 0), Vector3(0.7, 0.6, 0.6), "wood_dark")
	m.lathe(p + Vector3.UP * 0.6, PackedVector2Array([Vector2(0.38, 0.0), Vector2(0.45, 0.25), Vector2(0.38, 0.55), Vector2(0.2, 0.8),
		Vector2(0.0, 0.9)]), 10, "rice_ripe")
	for y: float in [0.85, 1.1]:
		HubArt._ring(m, p + Vector3.UP * y, 0.4, 0.46, 0.05, 10, "basket")
	m.box(p + Vector3(0, 0.78, 0.4), Vector3(0.14, 0.1, 0.04), "eye")
	for k: int in 3:
		m.blob(p + Vector3(0.5 + k * 0.2, 1.3 + k * 0.15, 0.3), Vector3(0.07, 0.06, 0.07), "wall_yellow", 0)


# ---------- vegetation ----------

## Coconut palm of the island: a curved ringed trunk, drooping fronds as thin closed leaves (the inverted-hull
## outline needs a volume, F093) and coconuts. Repeated: base detail, ≤ 2k.
static func coconut_palm() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.min_sides = 5  # scattered by the dozen (PERFORMANCE_BUDGET §7)
	var prev: Vector3 = Vector3.ZERO
	var segs: int = 4
	for i: int in segs:
		var t: float = float(i + 1) / float(segs)
		var p: Vector3 = Vector3(0.6 * t * t, 4.8 * t, 0)
		HubArt._limb(m, prev, p, 0.2 - 0.025 * i, 0.175 - 0.025 * i, "trunk" if i % 2 == 0 else "wood")
		prev = p
	var top: Vector3 = prev
	for k: int in 6:
		var a: float = TAU * float(k) / 6.0 + 0.2
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		var mid: Vector3 = top + d * 1.05 + Vector3.UP * 0.1
		var tip: Vector3 = top + d * 1.9 + Vector3.DOWN * 0.6
		for seg: Array in ([[top, mid], [mid, tip]] if k % 2 == 0 else [[top, tip]]):
			var a0: Vector3 = seg[0] as Vector3
			var a1: Vector3 = seg[1] as Vector3
			var c: Vector3 = (a0 + a1) * 0.5
			var along: Vector3 = (a1 - a0).normalized()
			var side: Vector3 = along.cross(Vector3.UP).normalized()
			var up: Vector3 = side.cross(along)
			m.push(Transform3D(Basis(side, up, along), c))
			m.blob(Vector3.ZERO, Vector3(0.3, 0.05, (a1 - a0).length() * 0.55), "leaf" if k % 2 == 0 else "leaf_dark", 0)
			m.pop()
	for k: int in 2:
		var a: float = TAU * float(k) / 2.0
		m.blob(top + Vector3(cos(a) * 0.18, -0.2, sin(a) * 0.18), Vector3(0.15, 0.17, 0.15), "wood_dark", 0)
	return m.build()


const SCATTER: Dictionary = {
	"palm": "veg_palm_coconut_a", "tree": "veg_tree_round_a", "bush": "veg_bush_a", "flower": "veg_flower_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"palm":
			var out: Array[Transform3D] = ZoneKit.scatter(9370, 3, 0, 1.6, BLOCKED, 0.9, 1.2)
			var rng: RandomNumberGenerator = RandomNumberGenerator.new()
			rng.seed = 9371
			for i: int in 18:
				var a: float = TAU * float(i) / 18.0 + rng.randf_range(-0.08, 0.08)
				var p: Vector3 = Vector3(cos(a), 0, sin(a)) * rng.randf_range(33.0, 40.0)
				if absf(p.x) < 12.0 and p.z < -24.0:
					continue
				out.append(HubArt._xf(p, rng, 0.9, 1.25))
			return out
		"tree":
			return ZoneKit.scatter(9372, 6, 6, 1.6, BLOCKED, 0.85, 1.2)
		"bush":
			return ZoneKit.scatter(9373, 32, 0, 1.0, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter(9374, 80, 0, 0.3, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter(9375, 260, 0, 0.3, BLOCKED, 0.8, 1.4)
	return []
