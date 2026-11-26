extends RefCounted
## Recipes of "Vương quốc Lịch Sử" (History Kingdom, `learning_zone.history`; F093, WORLD_DESIGN §2.2 chapter 4 "thư
## viện cổ, bản đồ cổ", §4.2, §11; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## Vietnamese heritage set pieces, kept friendly and without any weapon (WORLD_DESIGN §2.1): the citadel gate with a
## five-phoenix pavilion and yellow tiled roofs behind the wind gate (inspired by Ngọ Môn, Huế), a Đông Sơn bronze drum
## with its star and rings on a stone platform (discovery.history.bronze_drum), a map pavilion with an unrolled old
## map, a compass rose and a globe (discovery.history.old_map — abstract land shapes, no names), the Coin Gate: a
## giant ancient coin with a square hole the lane passes through (collectible.history.ancient_coin), the One Pillar
## Pagoda in a lotus pond (inspired by Chùa Một Cột), an hourglass (the chapter's Hourglass Keeper), red lanterns and
## banners, a red-brick citadel wall. Colours are art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "history"
const THEME_OVERRIDES: Dictionary = {"path": "paving_dark", "paving_dark": "roof_dark", "hedge": "roof_dark",
	"hedge_top": "stone"}
const DRUM_CENTER: Vector3 = Vector3(-15.8, 0, -6)
const MAP_CENTER: Vector3 = Vector3(16.0, 0, -6)
const COIN_RADIUS: float = 2.9
const COIN_HOLE: float = 1.25
const COIN_CENTER_Y: float = 1.25
const PAGODA_POND: Vector3 = Vector3(-14.0, 0, 15.0)
const PAGODA_POND_HALF: float = 3.0
const HOUSE: Array = [Vector3(18.0, 0, -16.0), -90.0, Vector2(5.0, 4.0)]
const BLOCKED: Array = [
	[DRUM_CENTER, Vector2(2.6, 2.6)],
	[MAP_CENTER, Vector2(2.6, 2.6)],
	[PAGODA_POND, Vector2(3.8, 3.8)],
	[Vector3(18.0, 0, -16.0), Vector2(3.2, 3.2)],
	[Vector3(0, 0, -31), Vector2(11.0, 5.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_history_terrain_a": terrain, "env/env_history_far_a": far, "bld/bld_history_set_a": buildings, "prop/prop_history_set_a": props,
		"prop/prop_history_drum_a": bronze_drum, "prop/prop_history_map_a": map_pavilion, "prop/prop_history_coin_gate_a": coin_gate,
		"bld/bld_history_pagoda_a": one_pillar_pagoda,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9381
	var th: Dictionary = theme()
	ZoneKit.terrain(m, th)
	ZoneKit.path_strip(m, Vector3(15.0, 0, -7.2), Vector3(16.5, 0, -4.8), "paving_dark")
	ZoneKit.path_strip(m, Vector3(-16.5, 0, -7.2), Vector3(-15.0, 0, -4.8), "paving_dark")
	# stone edging of the brick lane
	for s: float in [-1.0, 1.0]:
		ZoneKit.path_strip(m, Vector3(s * 2.0 - 0.15, 0, -23.0), Vector3(s * 2.0 + 0.15, 0, -2.6), "stone", 0.05)
		ZoneKit.path_strip(m, Vector3(s * 2.0 - 0.15, 0, 10.6), Vector3(s * 2.0 + 0.15, 0, 24.0), "stone", 0.05)
	# lotus pond of the One Pillar Pagoda
	var c: Vector3 = PAGODA_POND
	var h: float = PAGODA_POND_HALF
	m.quad(c + Vector3(-h, 0.06, -h), c + Vector3(-h, 0.06, h), c + Vector3(h, 0.06, h), c + Vector3(h, 0.06, -h), "water")
	for s: float in [-1.0, 1.0]:
		m.box(c + Vector3(s * (h + 0.25), 0.2, 0), Vector3(0.5, 0.4, 2.0 * h + 1.0), "stone", "stone")
		m.box(c + Vector3(0, 0.2, s * (h + 0.25)), Vector3(2.0 * h, 0.4, 0.5), "stone", "stone")
	for p: Vector3 in [Vector3(-1.8, 0, -1.6), Vector3(1.9, 0, -1.2), Vector3(-1.6, 0, 1.8), Vector3(2.0, 0, 1.9), Vector3(0.8, 0, 2.3)]:
		HubArt._disk(m, c + p + Vector3.UP * 0.07, rng.randf_range(0.3, 0.45), 8, "leaf")
		HubArt._lotus(m, c + p + Vector3.UP * 0.09)
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9389
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_citadel_gate(m)
	m.push_at(HOUSE[0] as Vector3, float(HOUSE[1]))
	ZoneKit.house(m, HOUSE[2] as Vector2, "roof", "roof_dark", "roof_dark", "lantern")
	m.pop()
	return m.build()


## Citadel gate (inspired by Ngọ Môn): a long stone-and-brick base with three arched gateways, a red-columned wooden
## pavilion with a two-tier yellow tiled roof on top, side wings and a flag.
static func _citadel_gate(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 2.0, 0), Vector3(17.0, 4.0, 5.0), "roof_dark")
	m.box(Vector3(0, 0.4, 0), Vector3(17.4, 0.8, 5.4), "stone_dark")
	m.box(Vector3(0, 4.1, 0), Vector3(17.4, 0.3, 5.4), "stone")
	for x: float in [-5.0, 0.0, 5.0]:
		var hw: float = 1.15 if x == 0.0 else 0.85
		m.push(Transform3D(Basis.IDENTITY, Vector3(x, 0.0, 2.52)))
		m.extrude(ZoneKit.arch(hw + 0.25, 3.2 if x == 0.0 else 2.6, 10), 0.08, "stone")
		m.pop()
		m.push(Transform3D(Basis.IDENTITY, Vector3(x, 0.0, 2.58)))
		m.extrude(ZoneKit.arch(hw, 2.95 if x == 0.0 else 2.35, 10), 0.08, "eye")
		m.pop()
	# merlons along the top
	for k: int in 9:
		m.box(Vector3(-8.0 + k * 2.0, 4.5, 2.4), Vector3(0.9, 0.5, 0.4), "roof_dark", "stone")
	# five-phoenix pavilion: red columns, wooden walls, two yellow roofs
	m.box(Vector3(0, 4.45, -0.2), Vector3(9.0, 0.4, 3.8), "stone")
	for x: float in [-3.8, -1.9, 0.0, 1.9, 3.8]:
		for z: float in [-1.6, 1.3]:
			m.cylinder(Vector3(x, 4.65, z), 0.16, 0.15, 2.3, 8, "lantern")
	m.box(Vector3(0, 5.75, -0.4), Vector3(7.4, 2.0, 2.0), "wood")
	for x: float in [-2.6, 0.0, 2.6]:
		m.box(Vector3(x, 5.8, 0.62), Vector3(1.3, 1.2, 0.08), "wall_yellow")
	m.tiled_roof(Vector3(0, 6.95, -0.2), 8.6, 3.8, 1.4, 0.8, 0.45, "wall_yellow", "roof_dark")
	m.box(Vector3(0, 8.0, -0.2), Vector3(4.0, 0.8, 2.0), "wood")
	m.tiled_roof(Vector3(0, 8.4, -0.2), 4.6, 2.4, 1.2, 0.5, 0.35, "wall_yellow", "roof_dark")
	m.blob(Vector3(0, 9.75, -0.2), Vector3(0.32, 0.32, 0.32), "lantern_glow", 1)
	# side wings and the flag on the left
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * 7.4, 4.9, -0.4), Vector3(2.4, 1.2, 2.4), "wood")
		m.push_at(Vector3(s * 7.4, 5.5, -0.4), 90.0)
		m.tiled_roof(Vector3.ZERO, 3.0, 2.8, 1.0, 0.4, 0.3, "wall_yellow", "roof_dark")
		m.pop()
	m.cylinder(Vector3(-8.6, 4.25, 1.8), 0.08, 0.06, 5.0, 8, "wood_dark")
	m.push(Transform3D(Basis.IDENTITY, Vector3(-8.55, 8.4, 1.8)))
	m.extrude(PackedVector2Array([Vector2(0, 0.5), Vector2(1.8, 0.45), Vector2(1.5, 0.1), Vector2(1.8, -0.25), Vector2(0, -0.3)]), 0.04,
			"wall_yellow", "lantern")
	m.pop()
	m.pop()


## One Pillar Pagoda (inspired by Chùa Một Cột) in the middle of its lotus pond: a stone pillar carrying a small
## square red-and-wood shrine with a curled tiled roof, and a little stair bridge from the pond edge.
static func one_pillar_pagoda() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.cylinder(Vector3.ZERO, 0.55, 0.5, 2.3, 12, "stone")
	m.cylinder(Vector3(0, 2.1, 0), 1.0, 0.4, 0.5, 12, "stone", "stone", true)
	m.box(Vector3(0, 2.7, 0), Vector3(2.6, 0.2, 2.6), "wood_dark")
	for x: float in [-1.15, 1.15]:
		for z: float in [-1.15, 1.15]:
			m.cylinder(Vector3(x, 2.8, z), 0.09, 0.08, 1.5, 8, "lantern")
	m.box(Vector3(0, 3.5, 0), Vector3(1.9, 1.3, 1.9), "wood")
	m.box(Vector3(0, 3.35, 0.96), Vector3(0.8, 1.0, 0.06), "lantern")
	for side: int in 4:
		m.push_at(Vector3.ZERO, 90.0 * side)
		m.box(Vector3(0, 2.95, 1.28), Vector3(2.5, 0.06, 0.06), "wood_dark")
		for k: int in 5:
			m.box(Vector3(-1.0 + k * 0.5, 2.88, 1.28), Vector3(0.05, 0.2, 0.05), "wood_dark")
		m.pop()
	m.tiled_roof(Vector3(0, 4.25, 0), 2.9, 2.9, 1.3, 0.55, 0.45)
	m.blob(Vector3(0, 5.6, 0), Vector3(0.2, 0.2, 0.2), "lantern_glow", 1)
	# stair bridge to the south edge of the pond
	for k: int in 6:
		m.box(Vector3(0, 0.25 + k * 0.42, 3.1 - k * 0.38), Vector3(1.1, 0.14, 0.45), "stone", "stone")
	for s: float in [-1.0, 1.0]:
		HubArt._limb(m, Vector3(s * 0.55, 0.6, 3.2), Vector3(s * 0.55, 3.0, 1.3), 0.05, 0.05, "lantern")
	return m.build()


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Đông Sơn bronze drum on a round stone platform (discovery.history.bronze_drum): flared tympanum with a twelve-point
## star, concentric rings and four little frog statuettes, bulging shoulder, waist and foot, two pairs of handles.
static func bronze_drum() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.cylinder(Vector3.ZERO, 1.9, 1.85, 0.4, 20, "stone", "stone")
	var base: Vector3 = Vector3(0, 0.4, 0)
	m.lathe(base, PackedVector2Array([Vector2(1.05, 0.0), Vector2(0.95, 0.12), Vector2(0.82, 0.5), Vector2(0.86, 0.75), Vector2(1.08, 1.05),
		Vector2(1.18, 1.25), Vector2(1.2, 1.32)]), 20, "bronze")
	HubArt._disk(m, base + Vector3.UP * 1.32, 1.2, 20, "bronze")
	var top: Vector3 = base + Vector3.UP * 1.33
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5), top))
	m.extrude(HubArt._star(0.42, 0.42), 0.04, "lantern_glow")
	m.pop()
	for r: float in [0.6, 0.85, 1.08]:
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), top))
		HubArt._torus_ring(m, Vector3.ZERO, r, 0.025, 24, 3, "bronze_patina")
		m.pop()
	for k: int in 4:
		var a: float = TAU * float(k) / 4.0 + PI * 0.25
		var p: Vector3 = top + Vector3(cos(a) * 1.0, 0.08, sin(a) * 1.0)
		m.blob(p, Vector3(0.12, 0.09, 0.15), "bronze_patina", 1)
		m.blob(p + Vector3(0, 0.08, 0.06), Vector3(0.07, 0.06, 0.07), "bronze_patina", 0)
	for r: float in [0.25, 0.75]:
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), base + Vector3.UP * r))
		HubArt._torus_ring(m, Vector3.ZERO, 0.88 if r < 0.5 else 0.9, 0.03, 24, 3, "bronze_patina")
		m.pop()
	for s: float in [-1.0, 1.0]:
		for dz: float in [-0.25, 0.25]:
			var p0: Vector3 = base + Vector3(s * 1.0, 1.0, dz)
			var p1: Vector3 = base + Vector3(s * 1.22, 0.8, dz)
			var p2: Vector3 = base + Vector3(s * 0.88, 0.55, dz)
			HubArt._limb(m, p0, p1, 0.04, 0.04, "bronze_patina")
			HubArt._limb(m, p1, p2, 0.04, 0.04, "bronze_patina")
	for x: float in [-1.55, 1.55]:
		HubArt._stone_lantern(m, Vector3(x, 0.0, -1.2))
	return m.build()


## Map pavilion (discovery.history.old_map in front of it): four red columns under a small yellow tiled roof, a
## table with an unrolled old map (abstract green land and blue sea shapes, a compass rose, no names) and a globe.
static func map_pavilion() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.15, 0), Vector3(3.6, 0.3, 3.6), "stone", "stone")
	for x: float in [-1.45, 1.45]:
		for z: float in [-1.45, 1.45]:
			m.cylinder(Vector3(x, 0.3, z), 0.13, 0.12, 2.6, 8, "lantern")
	m.box(Vector3(0, 2.95, 0), Vector3(3.4, 0.2, 3.4), "wood_dark")
	m.tiled_roof(Vector3(0, 3.05, 0), 3.6, 3.6, 1.4, 0.55, 0.4, "wall_yellow", "roof_dark")
	# table and map
	m.box(Vector3(0, 0.95, 0.2), Vector3(1.9, 0.12, 1.2), "wood_dark")
	for x: float in [-0.8, 0.8]:
		for z: float in [-0.3, 0.7]:
			m.box(Vector3(x, 0.6, z), Vector3(0.1, 0.6, 0.1), "wood_dark")
	m.push(Transform3D(Basis(Vector3.RIGHT, -0.25), Vector3(0, 1.08, 0.2)))
	m.box(Vector3.ZERO, Vector3(1.6, 0.03, 1.0), "paving")
	m.box(Vector3(0, 0.02, 0), Vector3(1.4, 0.02, 0.82), "water")
	for land: Array in [[Vector3(-0.35, 0.035, -0.1), Vector2(0.45, 0.32), 0.3], [Vector3(0.3, 0.035, 0.15), Vector2(0.35, 0.25), -0.4],
			[Vector3(0.45, 0.035, -0.25), Vector2(0.2, 0.15), 0.1]]:
		m.push(Transform3D(Basis(Vector3.UP, float(land[2])), land[0] as Vector3))
		var sz: Vector2 = land[1] as Vector2
		m.box(Vector3.ZERO, Vector3(sz.x, 0.02, sz.y), "leaf_light")
		m.pop()
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5), Vector3(-0.55, 0.045, 0.28)))
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.12, 0.12, 1.0)), Vector3.ZERO))
	m.extrude(HubArt._star(1.0, 0.35), 0.02, "cloth_red")
	m.pop()
	m.pop()
	for x: float in [-0.8, 0.8]:
		m.push(Transform3D(Basis(Vector3.BACK, PI * 0.5), Vector3(x, 0.05, 0)))
		m.cylinder(Vector3(0, -0.04, 0), 0.06, 0.06, 0.08, 8, "wood")
		m.pop()
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(x, 0.06, -0.5)))
		m.cylinder(Vector3.ZERO, 0.06, 0.06, 1.0, 8, "paving_dark")
		m.pop()
	m.pop()
	# globe on a stand
	var g: Vector3 = Vector3(1.15, 0.3, -0.9)
	m.cylinder(g, 0.25, 0.2, 0.08, 10, "wood_dark")
	m.cylinder(g, 0.04, 0.04, 0.85, 6, "wood_dark")
	m.blob(g + Vector3.UP * 1.15, Vector3(0.3, 0.3, 0.3), "water", 1)
	m.blob(g + Vector3(0.12, 1.25, 0.18), Vector3(0.16, 0.12, 0.1), "leaf_light", 0)
	m.push(Transform3D(Basis(Vector3.BACK, 0.4) * Basis(Vector3.UP, PI * 0.5), g + Vector3.UP * 1.15))
	HubArt._torus_ring(m, Vector3.ZERO, 0.36, 0.02, 16, 3, "bronze")
	m.pop()
	return m.build()


## Coin Gate: a giant ancient coin (round with a square hole) set upright and half sunk in the lane so players walk
## through the hole (collectible.history.ancient_coin in it); a raised rim and four studs (no characters, R2), stone
## blocks holding it.
static func coin_gate() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var r: float = COIN_RADIUS
	var h: float = COIN_HOLE
	var cy: float = COIN_CENTER_Y
	for q: int in 4:
		var pts: PackedVector2Array = PackedVector2Array()
		var a0: float = PI * 0.25 + PI * 0.5 * q
		var corner_a: Vector2 = Vector2(cos(a0), sin(a0)) * h * sqrt(2.0)
		var corner_b: Vector2 = Vector2(cos(a0 + PI * 0.5), sin(a0 + PI * 0.5)) * h * sqrt(2.0)
		pts.append(corner_a)
		for i: int in 7:
			var a: float = a0 + PI * 0.5 * float(i) / 6.0
			pts.append(Vector2(cos(a), sin(a)) * r)
		pts.append(corner_b)
		# keep only the part above the ground (y > -cy): clip the lower quadrant to the ground line
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, cy, 0)))
		m.extrude(_clip_above(pts, -cy - 0.3), 0.4, "bronze", "bronze_patina")
		m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, cy, 0)))
	HubArt._torus_ring(m, Vector3(0, 0, 0.2), r - 0.12, 0.08, 28, 4, "bronze_patina")
	m.pop()
	for k: int in 4:
		var a: float = PI * 0.5 * k
		var p: Vector3 = Vector3(cos(a), sin(a), 0) * (h + (r - h) * 0.55) + Vector3(0, cy, 0.22)
		if p.y > 0.2:
			m.blob(p, Vector3(0.22, 0.22, 0.08), "bronze_patina", 1)
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * (r - 0.3), 0.35, 0), Vector3(1.6, 0.7, 1.4), "stone", "stone")
	return m.build()


## Part of a polygon above the horizontal line y = `y0` (Sutherland–Hodgman against one edge).
static func _clip_above(pts: PackedVector2Array, y0: float) -> PackedVector2Array:
	var out: PackedVector2Array = PackedVector2Array()
	for i: int in pts.size():
		var a: Vector2 = pts[i]
		var b: Vector2 = pts[(i + 1) % pts.size()]
		var a_in: bool = a.y >= y0
		var b_in: bool = b.y >= y0
		if a_in:
			out.append(a)
		if a_in != b_in:
			var t: float = (y0 - a.y) / (b.y - a.y)
			out.append(a.lerp(b, t))
	return out


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9383
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = ZoneKit.PLAZA_CENTER + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (ZoneKit.PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "lantern")
	for b: Array in [[Vector3(-4.6, 0.08, 6.6), 50.0], [Vector3(4.6, 0.08, 6.6), -50.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	# banners along the lane (cờ đuôi nheo) in gold and the subject colour (F094, F093-2)
	for z: float in [-9.0, -18.0]:
		for s: float in [-1.0, 1.0]:
			_banner(m, Vector3(s * 3.2, 0, z), "wall_yellow" if s < 0.0 else "subject_history")
	_hourglass(m, Vector3(5.8, 0, -2.2))
	for p: Vector3 in [Vector3(-3.6, 0, -21.8), Vector3(3.6, 0, -21.8)]:
		HubArt._stone_lantern(m, p)
	ZoneKit.flower_bed(m, Vector3(-7.5, 0, -12.0), 1.4, PackedStringArray(["lantern", "lantern_glow", "flower_white"]), rng)
	ZoneKit.flower_bed(m, Vector3(7.5, 0, -12.0), 1.4, PackedStringArray(["wall_yellow", "flower_pink", "flower_white"]), rng)
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, 21.5), Vector3(21.8, 0, -21.6), Vector3(-9.0, 0, -21.8)]:
		m.blob(p + Vector3(0, 0.2, 0), Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)),
				"stone_dark", 0, 0.15, rng)
	return m.build()


## Tall swallow-tail banner on a pole.
static func _banner(m: LowPoly, p: Vector3, colour: String) -> void:
	m.cylinder(p, 0.22, 0.18, 0.25, 8, "stone")
	m.cylinder(p + Vector3.UP * 0.25, 0.06, 0.05, 3.6, 8, "wood_dark")
	m.blob(p + Vector3.UP * 3.9, Vector3(0.1, 0.1, 0.1), "lantern_glow", 0)
	m.push_at(p + Vector3(0, 3.6, 0), 90.0 if p.x < 0.0 else -90.0)
	m.extrude(PackedVector2Array([Vector2(0.05, 0.0), Vector2(0.45, 0.0), Vector2(0.45, -1.6), Vector2(0.25, -1.3), Vector2(0.05, -1.6)]),
			0.04, colour, "roof_dark")
	m.pop()


## Hourglass of the chapter (Hourglass Keeper): wooden frame, two glass bulbs with sand.
static func _hourglass(m: LowPoly, p: Vector3) -> void:
	m.cylinder(p, 0.6, 0.6, 0.15, 12, "wood_dark")
	m.cylinder(p + Vector3.UP * 1.95, 0.6, 0.6, 0.15, 12, "wood_dark")
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0
		m.cylinder(p + Vector3(cos(a) * 0.5, 0.15, sin(a) * 0.5), 0.05, 0.05, 1.8, 6, "wood")
	m.lathe(p + Vector3.UP * 0.15, PackedVector2Array([Vector2(0.38, 0.0), Vector2(0.42, 0.3), Vector2(0.3, 0.65), Vector2(0.06, 0.9),
		Vector2(0.06, 0.92), Vector2(0.3, 1.15), Vector2(0.42, 1.5), Vector2(0.38, 1.8)]), 12, "wind")
	m.cylinder(p + Vector3.UP * 0.15, 0.34, 0.2, 0.35, 12, "rice_ripe")
	m.cylinder(p + Vector3.UP * 1.25, 0.18, 0.36, 0.3, 12, "rice_ripe", "", false)


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"tree": "veg_tree_round_a", "bamboo": "veg_bamboo_a", "bush": "veg_bush_a", "flower": "veg_flower_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"tree":
			return ZoneKit.scatter(9390, 8, 24, 1.6, BLOCKED, 0.85, 1.25)
		"bamboo":
			return ZoneKit.scatter(9391, 4, 10, 1.2, BLOCKED, 0.8, 1.2, Vector2(25.5, 28.0))
		"bush":
			return ZoneKit.scatter(9393, 30, 0, 1.0, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter(9394, 100, 0, 0.3, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter(9395, 260, 0, 0.3, BLOCKED, 0.8, 1.4)
	return []
