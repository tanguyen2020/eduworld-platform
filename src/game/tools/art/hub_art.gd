extends RefCounted
## Recipes of the stylized low-poly Main Hub "Quảng trường Tia Sáng" (F067, WORLD_DESIGN §4.1, §11; ADR-0102).
## SPDX-License-Identifier: MIT — EduWorld first-party: every mesh is generated here, nothing is downloaded.
## Bake-time only (tools/art/bake_art.gd writes res://assets/models/**); the zone scene places the baked meshes and
## owns collision, spawn points and interactables (WORLD_RENDERING §2) — no gameplay here.
## Layout (metres, hub coordinates, north = -Z): walkable plaza ±27 m inside a hedge ring, lotus pond at the centre
## (basket boat), the banyan Spark Tree north of it, the wind-gate row at z = -20, the đình hall behind it, the Wind
## Library west, a village gate east, tiled-roof houses, rice terraces west/east, a lake south, hills and the far
## Challenge Mountain on the horizon.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")

const PLAZA_RADIUS: float = 12.0
const BOUNDARY: float = 27.5
const NORTH_EDGE: float = -29.6
const POND_RADIUS: float = 3.0
const TREE_CENTER: Vector3 = Vector3(0, 0, -7.4)
const HALL_CENTER: Vector3 = Vector3(0, 0, -26.2)
const HALL_SIZE: Vector3 = Vector3(12, 4, 6.4)
const LIBRARY_CENTER: Vector3 = Vector3(-18, 0, -8)
const GATE_X: float = 16.0
## Tiled-roof houses: [centre, yaw (deg), footprint (w, d)]. Collision boxes in Zone.tscn use the same values.
const HOUSES: Array = [
	[Vector3(-21.5, 0, 12.5), 90.0, Vector2(5.0, 4.0)],
	[Vector3(21.5, 0, 13.0), -90.0, Vector2(5.0, 4.0)],
	[Vector3(-12.5, 0, 22.0), 0.0, Vector2(5.0, 4.0)],
	[Vector3(13.0, 0, 22.0), 0.0, Vector2(5.4, 4.0)],
]
## Lamp posts around the plaza edge (angles in degrees, 0 = +X), skipping the four paths.
const HUB_MIN_BEVEL: float = 0.15
const LAMP_ANGLES: PackedFloat32Array = [25.0, 65.0, 115.0, 155.0, 205.0, 245.0, 295.0, 335.0]


## World builder (F068): rounded and smooth shaded, but only boxes thicker than HUB_MIN_BEVEL get a bevel — thin beams,
## boards and rafters seen from a distance stay sharp so the hub keeps its triangle budget (PERFORMANCE_BUDGET §7).
static func _builder() -> LowPoly:
	var m: LowPoly = LowPoly.new()
	m.min_bevel_size = HUB_MIN_BEVEL
	return m


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0  # far hills: smooth shading at base detail (F068 budget)
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6701
	# Central lawn grid (4 m cells, flat), soft two-tone patches.
	var cell: float = 4.0
	for ix: int in range(-8, 8):
		for iz: int in range(-8, 8):
			var x0: float = ix * cell
			var z0: float = iz * cell
			var cx: float = x0 + cell * 0.5
			var cz: float = z0 + cell * 0.5
			var patch: bool = sin(cx * 0.19) + cos(cz * 0.23 + 1.3) > 1.05
			var col: String = "grass_dark" if patch else "grass"
			m.quad(Vector3(x0, 0, z0), Vector3(x0, 0, z0 + cell), Vector3(x0 + cell, 0, z0 + cell), Vector3(x0 + cell, 0, z0), col)
	# Plaza: raised stone curb, terracotta paving, a darker ring and a stone ring round the pond.
	_ring(m, Vector3(0, 0.0, 0), PLAZA_RADIUS, PLAZA_RADIUS + 0.45, 0.1, 40, "stone")
	_disk(m, Vector3(0, 0.08, 0), PLAZA_RADIUS, 40, "paving")
	_annulus(m, Vector3(0, 0.085, 0), 9.4, 10.3, 40, "paving_dark")
	_annulus(m, Vector3(0, 0.085, 0), POND_RADIUS + 0.4, POND_RADIUS + 1.3, 32, "stone")
	for k: int in 8:
		var a: float = TAU * (float(k) + 0.5) / 8.0
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		var side: Vector3 = Vector3(-d.z, 0, d.x) * 0.35
		var p0: Vector3 = d * (POND_RADIUS + 1.3) + Vector3.UP * 0.088
		var p1: Vector3 = d * 9.4 + Vector3.UP * 0.088
		m.quad(p0 - side, p0 + side, p1 + side, p1 - side, "paving_dark")
	# Paths (sand) to the wind-gate row, the east gate, the south spawn lane and the library.
	_path(m, Vector3(-27, 0, -22.4), Vector3(27, 0, -17.2))
	_path(m, Vector3(-2.2, 0, -17.2), Vector3(2.2, 0, -PLAZA_RADIUS + 0.4))
	_path(m, Vector3(PLAZA_RADIUS - 0.4, 0, -4.2), Vector3(26.5, 0, 0.2))
	_path(m, Vector3(-2.0, 0, PLAZA_RADIUS - 0.4), Vector3(2.0, 0, 26.5))
	_path(m, Vector3(-15.0, 0, -9.8), Vector3(-PLAZA_RADIUS + 0.4, 0, -6.2))
	# Rice terraces east and west, outside the hedge ring.
	for side: float in [1.0, -1.0]:
		_terraces(m, side)
	# Lake south (raised bank), with a soil rim.
	_lake(m, rng)
	# Gentle slope north behind the hall (the hills and the far mountain are the Far mesh).
	m.quad(Vector3(32, 0, -32), Vector3(60, 3.5, -64), Vector3(-60, 3.5, -64), Vector3(-32, 0, -32), "grass_dark")
	return m.build()


## Far scenery of the Hub (F094, as the zones of F093): the far ground disk, the hills and Challenge Mountain on the
## horizon, drawn with the no-outline preset and without shadows (every pass counts in PERFORMANCE_BUDGET §7). Same
## hills as before: the random sequence of the terrain is replayed up to the lake.
static func far() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6701
	_lake(_builder(), rng)
	_disk(m, Vector3(0, -0.04, 0), 170.0, 20, "hill")
	_hills(m, rng)
	return m.build()


static func _disk(m: LowPoly, c: Vector3, r: float, sides: int, col: String) -> void:
	for i: int in sides:
		var a0: float = TAU * float(i) / float(sides)
		var a1: float = TAU * float(i + 1) / float(sides)
		m.tri(c, c + Vector3(cos(a1), 0, sin(a1)) * r, c + Vector3(cos(a0), 0, sin(a0)) * r, col)


static func _annulus(m: LowPoly, c: Vector3, r0: float, r1: float, sides: int, col: String) -> void:
	for i: int in sides:
		var a0: float = TAU * float(i) / float(sides)
		var a1: float = TAU * float(i + 1) / float(sides)
		var d0: Vector3 = Vector3(cos(a0), 0, sin(a0))
		var d1: Vector3 = Vector3(cos(a1), 0, sin(a1))
		m.quad(c + d0 * r0, c + d1 * r0, c + d1 * r1, c + d0 * r1, col)


## Vertical band (curb / rim) from r0 to r1 with height h: top annulus + outer and inner walls.
static func _ring(m: LowPoly, c: Vector3, r0: float, r1: float, h: float, sides: int, col: String) -> void:
	_annulus(m, c + Vector3.UP * h, r0, r1, sides, col)
	for i: int in sides:
		var a0: float = TAU * float(i) / float(sides)
		var a1: float = TAU * float(i + 1) / float(sides)
		var d0: Vector3 = Vector3(cos(a0), 0, sin(a0))
		var d1: Vector3 = Vector3(cos(a1), 0, sin(a1))
		m.quad(c + d0 * r1, c + d0 * r1 + Vector3.UP * h, c + d1 * r1 + Vector3.UP * h, c + d1 * r1, col)
		m.quad(c + d1 * r0, c + d1 * r0 + Vector3.UP * h, c + d0 * r0 + Vector3.UP * h, c + d0 * r0, col)


static func _path(m: LowPoly, a: Vector3, b: Vector3) -> void:
	var y: float = 0.045
	m.quad(Vector3(a.x, y, a.z), Vector3(a.x, y, b.z), Vector3(b.x, y, b.z), Vector3(b.x, y, a.z), "path")


static func _terraces(m: LowPoly, side: float) -> void:
	var levels: int = 7
	var segs: int = 12
	var z0: float = -44.0
	var z1: float = 44.0
	for k: int in levels:
		var inner: float = 31.0 + 4.2 * k
		var outer: float = inner + 4.2
		var y_low: float = 0.75 * k
		var y_top: float = 0.75 * (k + 1)
		var top_col: String = ["rice_young", "water", "rice_ripe", "rice_young", "water", "rice_young", "rice_ripe"][k]
		for s: int in segs:
			var za: float = lerpf(z0, z1, float(s) / float(segs))
			var zb: float = lerpf(z0, z1, float(s + 1) / float(segs))
			var wa: float = 1.8 * sin(za * 0.075 + k * 0.5)
			var wb: float = 1.8 * sin(zb * 0.075 + k * 0.5)
			var ia: Vector3 = Vector3(side * (inner + wa), y_top, za)
			var ib: Vector3 = Vector3(side * (inner + wb), y_top, zb)
			var oa: Vector3 = Vector3(side * (outer + wa + 0.6), y_top, za)
			var ob: Vector3 = Vector3(side * (outer + wb + 0.6), y_top, zb)
			var la: Vector3 = Vector3(ia.x, y_low, za)
			var lb: Vector3 = Vector3(ib.x, y_low, zb)
			if side > 0.0:
				m.quad(ia, ib, ob, oa, top_col)
				m.quad(la, lb, ib, ia, "soil")
			else:
				m.quad(ia, oa, ob, ib, top_col)
				m.quad(la, ia, ib, lb, "soil")
			if top_col == "water":
				var lip: float = 0.14
				var pa: Vector3 = ia + Vector3.UP * lip
				var pb: Vector3 = ib + Vector3.UP * lip
				var qa: Vector3 = pa + Vector3(side * 0.35, 0, 0)
				var qb: Vector3 = pb + Vector3(side * 0.35, 0, 0)
				if side > 0.0:
					m.quad(pa, pb, qb, qa, "grass_dark")
					m.quad(ia, ib, pb, pa, "soil")
				else:
					m.quad(pa, qa, qb, pb, "grass_dark")
					m.quad(ia, pa, pb, ib, "soil")


static func _lake(m: LowPoly, rng: RandomNumberGenerator) -> void:
	var c: Vector3 = Vector3(0, 0, 44)
	var pts: Array[Vector3] = []
	var n: int = 22
	for i: int in n:
		var a: float = TAU * float(i) / float(n)
		var r: float = 1.0 + rng.randf_range(-0.08, 0.08)
		pts.append(c + Vector3(cos(a) * 30.0 * r, 0, sin(a) * 10.0 * r))
	for i: int in n:
		var p: Vector3 = pts[i]
		var q: Vector3 = pts[(i + 1) % n]
		m.tri(c + Vector3.UP * 0.07, q + Vector3.UP * 0.07, p + Vector3.UP * 0.07, "water")
		var po: Vector3 = c + (p - c) * 1.08
		var qo: Vector3 = c + (q - c) * 1.08
		m.quad(p + Vector3.UP * 0.07, q + Vector3.UP * 0.07, qo + Vector3.UP * 0.16, po + Vector3.UP * 0.16, "soil")
		m.quad(po + Vector3.UP * 0.16, qo + Vector3.UP * 0.16, qo + Vector3.UP * 0.0, po + Vector3.UP * 0.0, "grass_dark")
	# darker band near the shore line
	for i: int in n:
		var p: Vector3 = c + (pts[i] - c) * 0.86 + Vector3.UP * 0.075
		var q: Vector3 = c + (pts[(i + 1) % n] - c) * 0.86 + Vector3.UP * 0.075
		var po: Vector3 = c + (pts[i] - c) * 0.99 + Vector3.UP * 0.075
		var qo: Vector3 = c + (pts[(i + 1) % n] - c) * 0.99 + Vector3.UP * 0.075
		m.quad(p, q, qo, po, "water_deep")


static func _hills(m: LowPoly, rng: RandomNumberGenerator) -> void:
	for i: int in 18:
		var a: float = TAU * (float(i) + rng.randf_range(-0.3, 0.3)) / 18.0
		var r: float = rng.randf_range(92.0, 125.0)
		var h: float = rng.randf_range(10.0, 24.0)
		var w: float = rng.randf_range(20.0, 32.0)
		var col: String = "hill" if i % 2 == 0 else "hill_far"
		m.blob(Vector3(cos(a) * r, -h * 0.25, sin(a) * r), Vector3(w, h, w * 0.8), col, 1, 0.05, rng)
	# Challenge Mountain on the northern horizon, with a pale cap.
	m.cylinder(Vector3(18, -2, -150), 46.0, 6.0, 58.0, 7, "hill_far", "flower_white", true, 0.3)
	m.cylinder(Vector3(18, 40.5, -150), 13.4, 0.5, 18.0, 7, "flower_white", "", false, 0.3)


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0  # lanterns and knobs: smooth shading at base detail (F068 budget)
	_hall(m)
	_library(m)
	for h: Array in HOUSES:
		m.push_at(h[0] as Vector3, float(h[1]))
		_house(m, h[2] as Vector2)
		m.pop()
	_village_gate(m)
	_hedges(m)
	return m.build()


## Đình (communal hall): stone base, wooden columns, cream walls, two-tier mái ngói with curled corners.
static func _hall(m: LowPoly) -> void:
	m.push_at(HALL_CENTER)
	var w: float = HALL_SIZE.x
	var d: float = HALL_SIZE.z
	m.box(Vector3(0, 0.25, 0), Vector3(w + 1.2, 0.5, d + 1.2), "stone", "stone")
	m.box(Vector3(0, 0.08, d * 0.5 + 1.0), Vector3(4.0, 0.16, 0.9), "stone")
	m.box(Vector3(0, 2.0, -0.4), Vector3(w - 1.0, 3.0, d - 2.0), "wall")
	for i: int in 6:
		var x: float = lerpf(-w * 0.5 + 0.4, w * 0.5 - 0.4, float(i) / 5.0)
		for z: float in [d * 0.5 - 0.3, -d * 0.5 + 0.3]:
			m.cylinder(Vector3(x, 0.5, z), 0.2, 0.18, 3.1, 8, "wood")
	# doors and red lacquered panels on the front wall
	for i: int in 3:
		var x: float = (i - 1) * 2.6
		m.box(Vector3(x, 1.6, d * 0.5 - 1.4), Vector3(1.5, 2.2, 0.12), "wood_dark")
		m.box(Vector3(x, 2.9, d * 0.5 - 1.38), Vector3(1.7, 0.3, 0.12), "lantern")
	m.tiled_roof(Vector3(0, 3.6, 0), w + 0.4, d + 0.2, 2.3, 1.0, 0.45)
	m.tiled_roof(Vector3(0, 5.5, 0), w * 0.55, d * 0.5, 1.4, 0.5, 0.3)
	m.box(Vector3(0, 5.35, 0), Vector3(w * 0.5, 0.5, d * 0.45), "wood_dark")
	# lanterns under the eaves
	for x: float in [-4.5, -1.5, 1.5, 4.5]:
		_lantern(m, Vector3(x, 3.05, d * 0.5 + 0.2), 0.32)
	m.pop()


static func _library(m: LowPoly) -> void:
	m.push_at(LIBRARY_CENTER, 90.0)
	m.box(Vector3(0, 0.15, 0), Vector3(6.8, 0.3, 6.8), "stone")
	m.box(Vector3(0, 1.75, 0), Vector3(5.6, 2.9, 5.6), "wall_yellow")
	for x: float in [-3.0, 3.0]:
		for z: float in [-3.0, 3.0]:
			m.cylinder(Vector3(x, 0.3, z), 0.17, 0.15, 2.9, 8, "wood")
	m.box(Vector3(0, 1.3, -2.85), Vector3(1.4, 2.0, 0.12), "wood_dark")
	for x: float in [-1.8, 1.8]:
		m.box(Vector3(x, 1.9, -2.83), Vector3(1.0, 0.9, 0.1), "wood")
		m.box(Vector3(x, 1.9, -2.85), Vector3(0.8, 0.7, 0.1), "water_deep")
	m.tiled_roof(Vector3(0, 3.2, 0), 6.4, 6.0, 2.4, 0.9, 0.4)
	# wind chimes: little open book sign over the door (icon, not colour only)
	m.box(Vector3(0, 2.75, -3.05), Vector3(1.6, 0.6, 0.1), "flower_white")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 2.75, -3.12)))
	m.extrude(_icon_points("practice_zone"), 0.06, "leaf_dark")
	m.pop()
	m.pop()


static func _house(m: LowPoly, size: Vector2) -> void:
	var w: float = size.x
	var d: float = size.y
	m.box(Vector3(0, 0.12, 0), Vector3(w + 0.6, 0.24, d + 0.6), "stone")
	m.box(Vector3(0, 1.4, 0), Vector3(w, 2.3, d), "wall")
	m.box(Vector3(0, 0.45, 0), Vector3(w + 0.04, 0.4, d + 0.04), "wall_yellow")
	m.box(Vector3(0, 1.1, -d * 0.5 - 0.01), Vector3(1.0, 1.7, 0.1), "wood_dark")
	for x: float in [-w * 0.32, w * 0.32]:
		m.box(Vector3(x, 1.55, -d * 0.5 - 0.01), Vector3(0.8, 0.7, 0.1), "wood")
		m.box(Vector3(x, 1.55, -d * 0.5 - 0.03), Vector3(0.62, 0.52, 0.1), "water_deep")
	# front veranda posts
	for x: float in [-w * 0.5 + 0.2, w * 0.5 - 0.2]:
		m.cylinder(Vector3(x, 0.24, -d * 0.5 - 0.7), 0.1, 0.09, 2.3, 6, "wood")
	m.tiled_roof(Vector3(0, 2.5, -0.3), w + 0.2, d + 1.4, 1.6, 0.5, 0.25)
	# flower pots by the door
	for x: float in [-0.9, 0.9]:
		m.cylinder(Vector3(x, 0.24, -d * 0.5 - 0.45), 0.2, 0.26, 0.35, 6, "roof")
		m.blob(Vector3(x, 0.75, -d * 0.5 - 0.45), Vector3(0.3, 0.26, 0.3), "leaf", 0)
		m.blob(Vector3(x + 0.08, 0.9, -d * 0.5 - 0.55), Vector3(0.09, 0.09, 0.09), "flower_pink", 0)


## Cổng làng: two masonry pillars (collision posts at z = -6 and z = 2) under a small tiled roof.
static func _village_gate(m: LowPoly) -> void:
	for z: float in [-6.0, 2.0]:
		m.box(Vector3(GATE_X, 0.2, z), Vector3(1.4, 0.4, 1.4), "stone_dark")
		m.box(Vector3(GATE_X, 2.6, z), Vector3(1.0, 4.4, 1.0), "wall")
		m.box(Vector3(GATE_X, 4.95, z), Vector3(1.25, 0.3, 1.25), "stone")
		m.cylinder(Vector3(GATE_X, 5.1, z), 0.35, 0.05, 0.7, 4, "roof_dark", "", false, 0.5)
	m.box(Vector3(GATE_X, 4.3, -2.0), Vector3(0.6, 0.6, 7.6), "wood_dark")
	m.box(Vector3(GATE_X - 0.32, 3.9, -2.0), Vector3(0.08, 0.7, 2.6), "lantern")
	m.push_at(Vector3(GATE_X, 4.7, -2.0), 90.0)
	m.tiled_roof(Vector3.ZERO, 8.4, 1.2, 0.9, 0.4, 0.25)
	m.pop()


## Clipped hedge ring on the walkable boundary (collision walls in Zone.tscn).
static func _hedges(m: LowPoly) -> void:
	var h: float = 1.1
	var t: float = 1.0
	m.box(Vector3(-BOUNDARY - t * 0.5, h * 0.5, -1.0), Vector3(t, h, 57.2), "grass_dark", "leaf")
	m.box(Vector3(BOUNDARY + t * 0.5, h * 0.5, -1.0), Vector3(t, h, 57.2), "grass_dark", "leaf")
	m.box(Vector3(-15.75, h * 0.5, BOUNDARY + t * 0.5), Vector3(23.5, h, t), "grass_dark", "leaf")
	m.box(Vector3(15.75, h * 0.5, BOUNDARY + t * 0.5), Vector3(23.5, h, t), "grass_dark", "leaf")
	m.box(Vector3(0, h * 0.5, NORTH_EDGE - t * 0.5), Vector3(2.0 * BOUNDARY + 2.0 * t, h, t), "grass_dark", "leaf")
	# south opening framed by two little stone posts (view to the lake)
	for x: float in [-3.6, 3.6]:
		m.box(Vector3(x, 0.7, BOUNDARY + t * 0.5), Vector3(0.8, 1.4, 1.2), "stone")
		_lantern(m, Vector3(x, 1.75, BOUNDARY + t * 0.5), 0.26)


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0  # lanterns, lotus, stones: smooth shading at base detail (F068 budget)
	_pond(m)
	for a: float in LAMP_ANGLES:
		var r: float = PLAZA_RADIUS - 0.8
		_lamp_post(m, Vector3(cos(deg_to_rad(a)) * r, 0.08, sin(deg_to_rad(a)) * r))
	for k: int in 4:
		var a: float = TAU * (float(k) + 0.5) / 4.0
		var p: Vector3 = Vector3(cos(a), 0, sin(a)) * 6.4
		m.push_at(p + Vector3.UP * 0.08, -rad_to_deg(a) + 90.0)
		_bench(m)
		m.pop()
	_signpost(m, Vector3(3.4, 0.08, 13.6))
	_stone_lantern(m, Vector3(-3.4, 0.0, 26.0))
	_stone_lantern(m, Vector3(3.4, 0.0, 26.0))
	# rocks on the lawns
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6702
	for p: Vector3 in [Vector3(-24, 0, -14), Vector3(23.5, 0, -12), Vector3(-25, 0, 24), Vector3(24.5, 0, 24),
			Vector3(-8, 0, 25.5), Vector3(9, 0, 25.8), Vector3(-25, 0, 3), Vector3(25.2, 0, 6)]:
		m.blob(p + Vector3(0, 0.2, 0), Vector3(rng.randf_range(0.5, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)),
				"stone_dark", 0, 0.15, rng)
	# basket boats on the lake
	for b: Array in [[Vector3(-8, 0.07, 41), 20.0], [Vector3(10, 0.07, 46), -35.0]]:
		m.push_at(b[0] as Vector3, float(b[1]))
		_basket_boat(m, 1.0)
		m.pop()
	return m.build()


static func _pond(m: LowPoly) -> void:
	_ring(m, Vector3(0, 0.0, 0), POND_RADIUS - 0.45, POND_RADIUS, 0.5, 20, "stone")
	_disk(m, Vector3(0, 0.32, 0), POND_RADIUS - 0.45, 20, "water")
	_annulus(m, Vector3(0, 0.325, 0), POND_RADIUS - 0.8, POND_RADIUS - 0.45, 20, "water_deep")
	var pads: Array = [[Vector3(-1.1, 0.34, -0.8), 0.42], [Vector3(-1.5, 0.34, 0.5), 0.34], [Vector3(1.4, 0.34, -1.0), 0.38],
		[Vector3(0.4, 0.34, -1.6), 0.3], [Vector3(-0.3, 0.34, 1.6), 0.36]]
	for pad: Array in pads:
		var c: Vector3 = pad[0] as Vector3
		var r: float = float(pad[1])
		for i: int in 7:
			var a0: float = TAU * float(i) / 8.0
			var a1: float = TAU * float(i + 1) / 8.0
			m.tri(c, c + Vector3(cos(a1), 0, sin(a1)) * r, c + Vector3(cos(a0), 0, sin(a0)) * r, "leaf")
	for f: Vector3 in [Vector3(-1.1, 0.36, -0.8), Vector3(1.4, 0.36, -1.0), Vector3(-0.3, 0.36, 1.6)]:
		_lotus(m, f)
	m.push_at(Vector3(0.9, 0.32, 0.7), 30.0)
	_basket_boat(m, 0.62)
	m.pop()


## Lotus flower: six petals, each a closed tetrahedron (no two-sided plane under the ink outline, F094 / F093-3).
static func _lotus(m: LowPoly, c: Vector3) -> void:
	for i: int in 6:
		var a: float = TAU * float(i) / 6.0
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		_tetra(m, [c + d * 0.05, c + d.rotated(Vector3.UP, 0.5) * 0.16 + Vector3.UP * 0.12,
			c + d.rotated(Vector3.UP, -0.5) * 0.16 + Vector3.UP * 0.12, c + d * 0.15 + Vector3.UP * 0.03], "flower_pink", "flower_pink")
	m.blob(c + Vector3.UP * 0.08, Vector3(0.05, 0.04, 0.05), "lantern_glow", 0)


## Closed tetrahedron of the corners `p` (4 points); the face opposite p[3] gets `top`, the other three `side`.
static func _tetra(m: LowPoly, p: Array, top: String, side: String) -> void:
	var center: Vector3 = ((p[0] as Vector3) + (p[1] as Vector3) + (p[2] as Vector3) + (p[3] as Vector3)) * 0.25
	m.tri_out(p[0] as Vector3, p[1] as Vector3, p[2] as Vector3, top, center)
	m.tri_out(p[0] as Vector3, p[1] as Vector3, p[3] as Vector3, side, center)
	m.tri_out(p[1] as Vector3, p[2] as Vector3, p[3] as Vector3, side, center)
	m.tri_out(p[2] as Vector3, p[0] as Vector3, p[3] as Vector3, side, center)


## Thuyền thúng: a round woven basket boat with a rim and a little paddle.
static func _basket_boat(m: LowPoly, r: float) -> void:
	var profile: PackedVector2Array = PackedVector2Array([
		Vector2(0.15 * r, -0.12 * r), Vector2(0.65 * r, -0.08 * r), Vector2(0.95 * r, 0.08 * r), Vector2(r, 0.24 * r),
	])
	m.lathe(Vector3.ZERO, profile, 12, "basket")
	# inner wall a little inside the outer one: a closed shell, not a two-sided plane under the outline (F094, F093-3)
	var inner: PackedVector2Array = PackedVector2Array()
	for p: Vector2 in profile:
		inner.append(Vector2(p.x * 0.93, p.y + 0.02 * r))
	m.lathe(Vector3.ZERO, inner, 12, "wood_dark", true)
	_ring(m, Vector3(0, 0.22 * r, 0), 0.94 * r, 1.04 * r, 0.06 * r, 12, "wood_dark")
	m.box(Vector3(0.1 * r, 0.12 * r, 0.0), Vector3(0.12 * r, 0.06 * r, 1.4 * r), "wood")


static func _lantern(m: LowPoly, c: Vector3, s: float) -> void:
	m.cylinder(c + Vector3(0, 0.5 * s, 0), 0.03, 0.03, 0.4 * s, 4, "wood_dark")
	m.blob(c, Vector3(0.38, 0.5, 0.38) * s, "lantern", 1)
	m.cylinder(c + Vector3(0, -0.55 * s, 0), 0.2 * s, 0.2 * s, 0.12 * s, 6, "lantern_glow")
	m.cylinder(c + Vector3(0, 0.43 * s, 0), 0.2 * s, 0.2 * s, 0.12 * s, 6, "lantern_glow")


static func _lamp_post(m: LowPoly, p: Vector3) -> void:
	m.cylinder(p, 0.3, 0.26, 0.3, 6, "stone")
	m.cylinder(p + Vector3.UP * 0.3, 0.09, 0.08, 2.6, 6, "wood_dark")
	m.box(p + Vector3(0.28, 2.75, 0), Vector3(0.7, 0.08, 0.08), "wood_dark")
	_lantern(m, p + Vector3(0.55, 2.35, 0), 0.5)


static func _bench(m: LowPoly) -> void:
	m.box(Vector3(0, 0.42, 0), Vector3(1.8, 0.1, 0.5), "wood", "wood")
	m.box(Vector3(0, 0.75, 0.22), Vector3(1.8, 0.35, 0.08), "wood")
	for x: float in [-0.75, 0.75]:
		m.box(Vector3(x, 0.2, 0), Vector3(0.12, 0.42, 0.45), "wood_dark")


static func _stone_lantern(m: LowPoly, p: Vector3) -> void:
	m.box(p + Vector3(0, 0.15, 0), Vector3(0.7, 0.3, 0.7), "stone_dark")
	m.box(p + Vector3(0, 0.75, 0), Vector3(0.3, 0.9, 0.3), "stone")
	m.box(p + Vector3(0, 1.35, 0), Vector3(0.55, 0.4, 0.55), "lantern_glow")
	m.cylinder(p + Vector3(0, 1.55, 0), 0.5, 0.05, 0.45, 4, "stone_dark", "", false, 0.5)


## Wooden signpost near the spawn: arrow boards to the wind gates (N), the village gate (E), the Wind Library (W)
## and the lake (S), each with a shape icon (not colour only — WORLD_DESIGN §11 accessibility).
static func _signpost(m: LowPoly, p: Vector3) -> void:
	m.cylinder(p, 0.12, 0.1, 3.0, 6, "wood_dark")
	m.cylinder(p + Vector3.UP * 3.0, 0.16, 0.0, 0.3, 6, "roof_dark")
	var arrows: Array = [[2.55, 180.0, "arena"], [2.15, 90.0, "quest_zone.discovery_ocean"], [1.75, 0.0, "practice_zone"],
		[1.35, 270.0, "social_zone"]]
	for a: Array in arrows:
		m.push_at(p + Vector3.UP * float(a[0]), float(a[1]))
		var board: PackedVector2Array = PackedVector2Array([Vector2(0.1, -0.17), Vector2(1.05, -0.17), Vector2(1.3, 0),
			Vector2(1.05, 0.17), Vector2(0.1, 0.17)])
		m.push(Transform3D(Basis(Vector3.UP, PI * 0.5), Vector3.ZERO))
		m.extrude(board, 0.06, "wood", "wood_dark")
		m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.24, 0.24, 1)), Vector3(0.62, 0, 0.045)))
		m.extrude(_icon_points(String(a[2])), 0.03, _icon_color(String(a[2])))
		m.pop()
		m.pop()
		m.pop()


# ---------- banyan Spark Tree ----------

## Cây đa — the Spark Tree of the square (WORLD_DESIGN §4.1, §11 mastery visualization; F068): one mesh per growth
## stage on the same round stone seat (collision cylinder SparkTreeCollision in Zone.tscn). 0 = grey sapling (no
## mastery yet), 1 = young green tree (beginning), 2 = growing banyan with a few sparks (developing), 3 = full banyan with
## aerial roots and glowing spark fruits (proficient), 4 = the full tree in bloom with more sparks (mastered).
static func spark_tree(stage: int) -> ArrayMesh:
	var m: LowPoly = _builder()
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6703
	var grey: bool = stage <= 0
	var trunk: String = "ash_dark" if grey else "trunk"
	var g: float = ([0.34, 0.52, 0.76, 1.0, 1.0] as Array)[clampi(stage, 0, 4)]
	m.push_at(TREE_CENTER)
	_ring(m, Vector3.ZERO, 1.25, 2.0, 0.45, 16, "stone")
	_disk(m, Vector3(0, 0.3, 0), 1.25, 16, "soil")
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(g, g, g)), Vector3(0, 0.3, 0)))
	m.lathe(Vector3(0, -0.05, 0), PackedVector2Array([Vector2(1.35, 0.0), Vector2(1.0, 0.65), Vector2(0.8, 1.9),
		Vector2(0.85, 3.1), Vector2(1.15, 4.0)]), 12, trunk)
	if stage < 2:
		_limb(m, Vector3(0, 3.8, 0), Vector3(0, 6.2, 0), 0.75, 0.5, trunk)
	if stage >= 2:
		for k: int in 4:
			var a: float = TAU * (float(k) + 0.3) / 4.0
			var d: Vector3 = Vector3(cos(a), 0, sin(a))
			_limb(m, Vector3(0, 3.6, 0) + d * 0.5, Vector3(0, 5.7, 0) + d * 2.8, 0.42, 0.22, trunk)
		var roots: int = 3 if stage == 2 else 7
		for k: int in roots:
			var a: float = TAU * (float(k) + rng.randf_range(0.2, 0.8)) / float(roots)
			var r: float = rng.randf_range(2.3, 3.4)
			var top: Vector3 = Vector3(cos(a) * r, rng.randf_range(4.5, 5.1), sin(a) * r)
			var foot: Vector3 = Vector3(cos(a) * (r + 0.3), -0.3 / g, sin(a) * (r + 0.3))
			_limb(m, foot, top, 0.055, 0.04, trunk)
	# canopy: soft cartoon clouds of leaves
	var blobs: Array = [[Vector3(0, 6.9, 0), Vector3(4.0, 2.0, 3.8)], [Vector3(2.8, 6.1, 1.2), Vector3(2.9, 1.6, 2.7)],
		[Vector3(-2.9, 6.2, 0.8), Vector3(3.0, 1.6, 2.8)], [Vector3(0.6, 6.0, -2.9), Vector3(3.0, 1.5, 2.6)],
		[Vector3(-0.8, 5.8, 3.0), Vector3(2.8, 1.5, 2.5)], [Vector3(0.4, 8.1, 0.3), Vector3(2.6, 1.4, 2.4)]]
	var count: int = 2 if stage <= 1 else blobs.size()
	for i: int in count:
		var b: Array = blobs[i * 5 if stage <= 1 else i]
		var shades: PackedStringArray = ["leaf_dark", "leaf", "leaf_dark", "leaf", "leaf_dark", "leaf_light"]
		var col: String = "ash" if grey else shades[i * 5 if stage <= 1 else i]
		m.blob(b[0] as Vector3, b[1] as Vector3, col, 1, 0.02, rng)
	# spark fruits hanging under the canopy, more with mastery (small: flat-light detail)
	m.blob_detail = 0
	m.min_sides = 3
	var sparks: int = ([0, 0, 5, 12, 12] as Array)[clampi(stage, 0, 4)]
	for k: int in sparks:
		var a: float = TAU * float(k) / float(sparks) + rng.randf_range(-0.2, 0.2)
		var r: float = rng.randf_range(2.2, 4.2)
		var c: Vector3 = Vector3(cos(a) * r, rng.randf_range(4.7, 5.5), sin(a) * r)
		m.cylinder(c + Vector3.UP * 0.12, 0.015, 0.015, 0.5, 3, trunk, "", false)
		m.blob(c, Vector3(0.17, 0.17, 0.17), "spark" if k % 3 else "lantern_glow", 0)
	# blossoms all over the canopy when mastered
	if stage >= 4:
		for k: int in 11:
			var b: Array = blobs[k % blobs.size()]
			var dir: Vector3 = Vector3(rng.randf_range(-1, 1), rng.randf_range(0.1, 1), rng.randf_range(-1, 1)).normalized()
			var p: Vector3 = (b[0] as Vector3) + dir * (b[1] as Vector3) * 0.95
			m.blob(p, Vector3(0.5, 0.4, 0.5), "flower_pink" if k % 3 else "flower_white", 0)
	m.blob_detail = LowPoly.BLOB_DETAIL
	m.min_sides = LowPoly.MIN_SIDES
	m.pop()
	m.pop()
	return m.build()


static func _limb(m: LowPoly, a: Vector3, b: Vector3, ra: float, rb: float, col: String) -> void:
	var dir: Vector3 = b - a
	var length: float = dir.length()
	var up: Vector3 = dir / length
	var side: Vector3 = up.cross(Vector3.FORWARD if absf(up.dot(Vector3.FORWARD)) < 0.9 else Vector3.RIGHT).normalized()
	var fwd: Vector3 = side.cross(up)
	m.push(Transform3D(Basis(side, up, fwd), a))
	m.cylinder(Vector3.ZERO, ra, rb, length, 6, col)
	m.pop()


# ---------- wind gate (AreaPortal) ----------

## Cổng gió of one destination: wooden frame with a tiled cap, a pale wind swirl and a sign with the destination's
## shape icon in its subject colour (local coordinates, faces +Z towards the plaza).
static func wind_gate(target_area_id: String) -> ArrayMesh:
	var m: LowPoly = _builder()
	m.min_sides = 6  # eleven gates in a row (F068 budget)
	for x: float in [-1.45, 1.45]:
		m.box(Vector3(x, 0.15, 0), Vector3(0.6, 0.3, 0.6), "stone")
		m.cylinder(Vector3(x, 0.3, 0), 0.16, 0.13, 3.0, 8, "wood_dark")
	m.box(Vector3(0, 3.25, 0), Vector3(3.7, 0.24, 0.32), "wood_dark")
	m.box(Vector3(0, 2.75, 0), Vector3(3.1, 0.14, 0.2), "wood")
	m.tiled_roof(Vector3(0, 3.38, 0), 3.6, 0.5, 0.45, 0.25, 0.16)
	_torus_ring(m, Vector3(0, 1.55, 0), 1.05, 0.08, 18, 6, "wind")
	_torus_ring(m, Vector3(0, 1.55, 0), 0.62, 0.05, 14, 5, "spark")
	# sign plate with the icon
	m.box(Vector3(0, 4.25, 0.02), Vector3(1.0, 0.9, 0.12), "flower_white")
	m.box(Vector3(0, 4.25, 0.0), Vector3(1.12, 1.02, 0.08), "wood_dark")
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.34, 0.34, 1)), Vector3(0, 4.25, 0.1)))
	m.extrude(_icon_points(target_area_id), 0.06, _icon_color(target_area_id))
	m.pop()
	m.push(Transform3D(Basis(Vector3.UP, PI).scaled(Vector3(0.34, 0.34, 1)), Vector3(0, 4.25, -0.06)))
	m.extrude(_icon_points(target_area_id), 0.06, _icon_color(target_area_id))
	m.pop()
	return m.build()


static func _torus_ring(m: LowPoly, c: Vector3, big: float, small: float, segs: int, sides: int, col: String) -> void:
	for i: int in segs:
		var a0: float = TAU * float(i) / float(segs)
		var a1: float = TAU * float(i + 1) / float(segs)
		var c0: Vector3 = Vector3(cos(a0), sin(a0), 0)
		var c1: Vector3 = Vector3(cos(a1), sin(a1), 0)
		for j: int in sides:
			var b0: float = TAU * float(j) / float(sides)
			var b1: float = TAU * float(j + 1) / float(sides)
			var p00: Vector3 = c + c0 * (big + cos(b0) * small) + Vector3(0, 0, sin(b0) * small)
			var p01: Vector3 = c + c0 * (big + cos(b1) * small) + Vector3(0, 0, sin(b1) * small)
			var p10: Vector3 = c + c1 * (big + cos(b0) * small) + Vector3(0, 0, sin(b0) * small)
			var p11: Vector3 = c + c1 * (big + cos(b1) * small) + Vector3(0, 0, sin(b1) * small)
			m.quad(p00, p10, p11, p01, col)


## Destination icon outline (about 2 × 2 units, centred): the shape tells the place without relying on colour.
static func _icon_points(target_area_id: String) -> PackedVector2Array:
	match target_area_id:
		"learning_zone.math":  # plus
			return PackedVector2Array([Vector2(-0.3, 1), Vector2(0.3, 1), Vector2(0.3, 0.3), Vector2(1, 0.3), Vector2(1, -0.3),
				Vector2(0.3, -0.3), Vector2(0.3, -1), Vector2(-0.3, -1), Vector2(-0.3, -0.3), Vector2(-1, -0.3), Vector2(-1, 0.3),
				Vector2(-0.3, 0.3)])
		"learning_zone.vietnamese":  # open book
			return PackedVector2Array([Vector2(-1, -0.7), Vector2(0, -0.45), Vector2(1, -0.7), Vector2(1, 0.75), Vector2(0, 0.5),
				Vector2(-1, 0.75)])
		"learning_zone.english":  # speech bubble
			return _bubble()
		"learning_zone.science":  # flask
			return PackedVector2Array([Vector2(-0.25, 1), Vector2(0.25, 1), Vector2(0.25, 0.2), Vector2(0.9, -0.9), Vector2(0.75, -1),
				Vector2(-0.75, -1), Vector2(-0.9, -0.9), Vector2(-0.25, 0.2)])
		"learning_zone.history":  # pagoda roofs
			return PackedVector2Array([Vector2(0, 1), Vector2(0.5, 0.55), Vector2(0.25, 0.55), Vector2(0.75, 0.05), Vector2(0.4, 0.05),
				Vector2(0.95, -0.5), Vector2(0.35, -0.5), Vector2(0.35, -1), Vector2(-0.35, -1), Vector2(-0.35, -0.5), Vector2(-0.95, -0.5),
				Vector2(-0.4, 0.05), Vector2(-0.75, 0.05), Vector2(-0.25, 0.55), Vector2(-0.5, 0.55)])
		"quest_zone.discovery_ocean":  # wave
			return PackedVector2Array([Vector2(-1, -0.8), Vector2(1, -0.8), Vector2(1, -0.2), Vector2(0.6, 0.1), Vector2(0.2, 0.6),
				Vector2(-0.2, 0.9), Vector2(-0.6, 0.7), Vector2(-0.3, 0.45), Vector2(-0.35, 0.1), Vector2(-0.7, -0.2), Vector2(-1, -0.25)])
		"challenge_mountain":  # mountain with a peak
			return PackedVector2Array([Vector2(-1, -0.8), Vector2(-0.35, 0.35), Vector2(-0.1, 0.0), Vector2(0.3, 0.9), Vector2(1, -0.8)])
		"exam_zone":  # tower
			return PackedVector2Array([Vector2(-0.45, -1), Vector2(0.45, -1), Vector2(0.45, 0.35), Vector2(0.65, 0.35), Vector2(0, 1),
				Vector2(-0.65, 0.35), Vector2(-0.45, 0.35)])
		"arena":  # star
			return _star(1.0, 0.45)
		"practice_zone":  # leaf / feather of the Wind Library
			return PackedVector2Array([Vector2(0, -1), Vector2(0.45, -0.5), Vector2(0.6, 0.1), Vector2(0.35, 0.65), Vector2(0, 1),
				Vector2(-0.35, 0.65), Vector2(-0.6, 0.1), Vector2(-0.45, -0.5)])
		"social_zone":  # heart
			return PackedVector2Array([Vector2(0, -0.95), Vector2(0.85, -0.05), Vector2(0.95, 0.45), Vector2(0.65, 0.85), Vector2(0.25, 0.8),
				Vector2(0, 0.45), Vector2(-0.25, 0.8), Vector2(-0.65, 0.85), Vector2(-0.95, 0.45), Vector2(-0.85, -0.05)])
		"main_hub":  # house: the gate home of every zone (F093)
			return PackedVector2Array([Vector2(-0.75, -1), Vector2(0.75, -1), Vector2(0.75, 0.15), Vector2(1.0, 0.15), Vector2(0, 1),
				Vector2(-1.0, 0.15), Vector2(-0.75, 0.15)])
	return _star(0.8, 0.8)


static func _icon_color(target_area_id: String) -> String:
	match target_area_id:
		"learning_zone.math":
			return "subject_math"
		"learning_zone.vietnamese":
			return "subject_vietnamese"
		"learning_zone.english":
			return "subject_english"
		"learning_zone.science":
			return "subject_science"
		"learning_zone.history":
			return "subject_history"
		"quest_zone.discovery_ocean":
			return "water_deep"
		"challenge_mountain":
			return "stone_dark"
		"exam_zone":
			return "cloth_violet"
		"arena":
			return "lantern_glow"
		"practice_zone":
			return "leaf_dark"
		"social_zone":
			return "flower_pink"
		"main_hub":
			return "roof"
	return "spark"


static func _star(outer: float, inner: float) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in 10:
		var a: float = PI * 0.5 + TAU * float(i) / 10.0
		var r: float = outer if i % 2 == 0 else outer * inner
		pts.append(Vector2(cos(a), sin(a)) * r)
	return pts


static func _bubble() -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in 12:
		var a: float = TAU * float(i) / 12.0
		if i == 9:
			pts.append(Vector2(-0.55, -1.0))
		pts.append(Vector2(cos(a) * 1.0, sin(a) * 0.7 + 0.1))
	return pts


# ---------- small props shared by every zone ----------

## Notice board (bảng tin) with a small tiled cap: board face 2.4 × 2.4 m centred 1.6 m up, like its collision box.
static func notice_board() -> ArrayMesh:
	var m: LowPoly = _builder()
	for x: float in [-1.3, 1.3]:
		m.box(Vector3(x, 1.4, 0), Vector3(0.18, 2.8, 0.18), "wood_dark")
	m.box(Vector3(0, 1.6, 0), Vector3(2.4, 2.0, 0.14), "wood")
	m.box(Vector3(0, 1.6, -0.03), Vector3(2.1, 1.7, 0.12), "flower_white")
	for p: Vector3 in [Vector3(-0.55, 1.9, -0.1), Vector3(0.5, 1.95, -0.1), Vector3(-0.3, 1.25, -0.1), Vector3(0.55, 1.3, -0.1)]:
		m.box(p, Vector3(0.7, 0.45, 0.02), "wall")
		m.box(p + Vector3(0, 0.17, -0.012), Vector3(0.08, 0.08, 0.02), "lantern")
	m.tiled_roof(Vector3(0, 2.8, 0), 2.6, 0.5, 0.4, 0.2, 0.12)
	return m.build()


## Generic quest / discovery marker of a WorldAnchor (F094 "dễ dùng, thu hút"): a big floating spark gem (closed
## bipyramid) with a gold halo ring over a stone base with a glowing top — readable from across a
## zone, inviting the student to walk up and interact.
static func spark_marker() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0
	m.min_sides = 6  # one marker per anchor, many in the Hub (PERFORMANCE_BUDGET §7)
	m.cylinder(Vector3.ZERO, 0.42, 0.36, 0.16, 6, "stone", "lantern_glow")
	var c: Vector3 = Vector3(0, 1.4, 0)
	var top: Vector3 = c + Vector3.UP * 0.5
	var bottom: Vector3 = c + Vector3.DOWN * 0.42
	for i: int in 6:
		var a0: float = TAU * float(i) / 6.0
		var a1: float = TAU * float(i + 1) / 6.0
		var p0: Vector3 = c + Vector3(cos(a0), 0, sin(a0)) * 0.32
		var p1: Vector3 = c + Vector3(cos(a1), 0, sin(a1)) * 0.32
		m.tri(p0, top, p1, "spark")
		m.tri(p1, bottom, p0, "lantern_glow")
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), c))
	_torus_ring(m, Vector3.ZERO, 0.58, 0.035, 10, 3, "lantern_glow")
	m.pop()
	return m.build()


# ---------- vegetation (MultiMesh) ----------

static func tree_round() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0  # scattered by the dozen: smooth shading, base detail (F068 budget)
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6710
	m.cylinder(Vector3.ZERO, 0.22, 0.16, 2.0, 6, "trunk")
	m.blob(Vector3(0, 2.6, 0), Vector3(1.45, 1.25, 1.45), "leaf", 1, 0.03, rng)
	m.blob(Vector3(-0.55, 2.35, 0.45), Vector3(0.85, 0.75, 0.85), "leaf", 0, 0.03, rng)
	m.blob(Vector3(0.45, 3.25, 0.2), Vector3(0.9, 0.75, 0.9), "leaf_light", 0, 0.03, rng)
	return m.build()


## Areca palm (cây cau): slim ringed trunk and drooping fronds. Each frond is a closed tetrahedron folded along its
## spine (leaf on one half, darker other half and underside; same 4 triangles as before) — not a two-sided plane, which the inverted-hull outline drew black from
## behind (F093-3, fixed in F094).
static func palm() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.cylinder(Vector3.ZERO, 0.14, 0.1, 4.6, 5, "trunk")
	for k: int in 7:
		var a: float = TAU * float(k) / 7.0
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		var top: Vector3 = Vector3(0, 4.6, 0)
		var tip: Vector3 = top + d * 1.7 + Vector3.DOWN * 0.7
		var mid: Vector3 = top + d * 0.9 + Vector3.UP * 0.25
		var side: Vector3 = Vector3(-d.z, 0, d.x) * 0.28
		_tetra(m, [top, mid + side, tip, mid - side + Vector3.DOWN * 0.12], "leaf", "leaf_dark")
	return m.build()


static func bamboo() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0  # scattered by the dozen: smooth shading, base detail (F068 budget)
	m.min_sides = 6
	for p: Vector3 in [Vector3(0, 0, 0), Vector3(0.35, 0, 0.15), Vector3(-0.25, 0, 0.3), Vector3(0.1, 0, -0.35), Vector3(-0.3, 0, -0.2)]:
		var h: float = 3.6 + p.x * 2.0
		m.cylinder(p, 0.06, 0.05, h, 5, "bamboo")
		m.blob(p + Vector3(0.1, h, 0), Vector3(0.6, 0.35, 0.5), "leaf_light", 0)
	return m.build()


static func bush() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.blob_detail = 0  # scattered by the dozen: smooth shading, base detail (F068 budget)
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6711
	m.blob(Vector3(0, 0.38, 0), Vector3(0.75, 0.5, 0.7), "leaf", 1, 0.05, rng)
	m.blob(Vector3(0.25, 0.65, 0.1), Vector3(0.18, 0.18, 0.18), "flower_pink", 0)
	m.blob(Vector3(-0.3, 0.55, -0.25), Vector3(0.16, 0.16, 0.16), "flower_white", 0)
	return m.build()


static func tuft() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.set_flat()  # dense decor: flat and light (F068)
	for k: int in 4:
		var a: float = TAU * float(k) / 4.0 + 0.4
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		var side: Vector3 = Vector3(-d.z, 0, d.x) * 0.07
		m.tri(-side, d * 0.18 + Vector3.UP * 0.42, side, "grass_dark")
		m.tri(side, d * 0.18 + Vector3.UP * 0.42, -side, "grass_dark")
	return m.build()


static func flower() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.set_flat()  # dense decor: flat and light (F068)
	m.cylinder(Vector3.ZERO, 0.02, 0.02, 0.32, 3, "grass_dark")
	m.blob(Vector3(0, 0.36, 0), Vector3(0.09, 0.06, 0.09), "flower_white", 0)
	m.blob(Vector3(0, 0.39, 0), Vector3(0.035, 0.03, 0.035), "lantern_glow", 0)
	return m.build()


static func rice() -> ArrayMesh:
	var m: LowPoly = _builder()
	m.set_flat()  # dense decor: flat and light (F068)
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0
		var d: Vector3 = Vector3(cos(a), 0, sin(a)) * 0.08
		var side: Vector3 = Vector3(-d.z, 0, d.x)
		m.tri(-side, d * 2.0 + Vector3.UP * 0.5, side, "rice_young")
		m.tri(side, d * 2.0 + Vector3.UP * 0.5, -side, "rice_young")
	return m.build()


## Placement transforms of the hub vegetation, by kind (deterministic, seeded). Inside the hedge ring only low decor
## (bushes, tufts, flowers) and a few trees in the corners; trees, palms and bamboo frame the ring outside.
static func placements(kind: String) -> Array[Transform3D]:
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6720 + ["tree", "palm", "bamboo", "bush", "tuft", "flower", "rice"].find(kind)
	var out: Array[Transform3D] = []
	match kind:
		"tree":
			for i: int in 26:
				var p: Vector3 = _outside_ring(rng, 30.5, 36.0)
				out.append(_xf(p, rng, 0.85, 1.25))
			for p: Vector3 in [Vector3(-24.5, 0, -25.5), Vector3(24.5, 0, -25.5), Vector3(-24.5, 0, 25.0), Vector3(24.8, 0, 24.5),
					Vector3(-24.6, 0, -2.0), Vector3(24.6, 0, -8.5)]:
				out.append(_xf(p, rng, 0.9, 1.1))
		"palm":
			for i: int in 14:
				var x: float = rng.randf_range(-26.0, 26.0)
				out.append(_xf(Vector3(x, 0, rng.randf_range(31.0, 33.5)), rng, 0.9, 1.2))
			for p: Vector3 in [Vector3(-19, 0, -16.5), Vector3(19.5, 0, -16.5), Vector3(-25, 0, 9), Vector3(25, 0, 17)]:
				out.append(_xf(p, rng, 0.95, 1.1))
		"bamboo":
			for i: int in 12:
				var z: float = rng.randf_range(-30.0, -16.0) if i % 2 == 0 else rng.randf_range(4.0, 28.0)
				var x: float = (29.6 + rng.randf_range(0.0, 1.0)) * (1.0 if i % 4 < 2 else -1.0)
				out.append(_xf(Vector3(x, 0, z), rng, 0.8, 1.2))
		"bush":
			for i: int in 60:
				var p: Vector3 = Vector3(rng.randf_range(-26.0, 26.0), 0, rng.randf_range(-28.5, 26.5))
				if _clear(p, 1.2):
					out.append(_xf(p, rng, 0.7, 1.2))
		"tuft":
			for i: int in 420:
				var p: Vector3 = Vector3(rng.randf_range(-27.0, 27.0), 0, rng.randf_range(-29.0, 27.0))
				if _clear(p, 0.4):
					out.append(_xf(p, rng, 0.8, 1.4))
		"flower":
			for i: int in 220:
				var p: Vector3 = Vector3(rng.randf_range(-27.0, 27.0), 0, rng.randf_range(-29.0, 27.0))
				if _clear(p, 0.4):
					out.append(_xf(p, rng, 0.8, 1.3))
		"rice":
			for side: float in [1.0, -1.0]:
				for k: int in [0, 2, 3, 5, 6]:
					for i: int in 70:
						var z: float = rng.randf_range(-42.0, 42.0)
						var inner: float = 31.0 + 4.2 * k + 1.8 * sin(z * 0.075 + k * 0.5)
						var x: float = side * (inner + rng.randf_range(0.3, 4.4))
						out.append(_xf(Vector3(x, 0.75 * (k + 1), z), rng, 0.8, 1.3))
	return out


static func _xf(p: Vector3, rng: RandomNumberGenerator, s0: float, s1: float) -> Transform3D:
	var s: float = rng.randf_range(s0, s1)
	return Transform3D(Basis(Vector3.UP, rng.randf_range(0.0, TAU)).scaled(Vector3(s, s, s)), p)


static func _outside_ring(rng: RandomNumberGenerator, r0: float, r1: float) -> Vector3:
	var side: int = rng.randi_range(0, 3)
	var t: float = rng.randf_range(-r1, r1)
	var d: float = rng.randf_range(r0, r1)
	match side:
		0:
			return Vector3(d, 0, t)
		1:
			return Vector3(-d, 0, t)
		2:
			return Vector3(t, 0, -d)
	return Vector3(t * 0.6, 0, d + 1.0) if absf(t) > 6.0 else Vector3(t + 8.0, 0, d + 1.0)


## True when `p` keeps `margin` away from the plaza, paths, pond, buildings, gates and anchors of the hub layout.
static func _clear(p: Vector3, margin: float) -> bool:
	var flat: Vector2 = Vector2(p.x, p.z)
	if flat.length() < PLAZA_RADIUS + 0.8 + margin:
		return false
	if absf(p.z + 19.8) < 3.0 + margin:  # wind-gate row and its path
		return false
	if absf(p.x) < 2.5 + margin and p.z > 0:  # south lane
		return false
	if p.x > PLAZA_RADIUS - 1.0 and absf(p.z + 2.0) < 2.4 + margin:  # east path / gate
		return false
	if p.x < -PLAZA_RADIUS + 1.0 and p.x > -15.5 and absf(p.z + 8.0) < 2.0 + margin:
		return false
	if absf(p.x - LIBRARY_CENTER.x) < 3.8 + margin and absf(p.z - LIBRARY_CENTER.z) < 3.8 + margin:
		return false
	if absf(p.x) < 7.5 + margin and p.z < -21.5:  # hall
		return false
	if absf(p.x - GATE_X) < 1.5 + margin and p.z > -7.0 and p.z < 3.0:
		return false
	for h: Array in HOUSES:
		var c: Vector3 = h[0] as Vector3
		if Vector2(p.x - c.x, p.z - c.z).length() < 4.4 + margin:
			return false
	if Vector2(p.x - 3.4, p.z - 13.6).length() < 1.0 + margin:
		return false
	return true
