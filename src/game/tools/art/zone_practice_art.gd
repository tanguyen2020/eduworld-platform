extends RefCounted
## Recipes of "Thư viện Gió" (Breeze Library, `practice_zone`; F094, WORLD_DESIGN §2.4, §4.1 "bàn học cá nhân, góc học
## cùng nhau, luyện theo chủ đề", §11; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## A quiet, warm library garden of the central town: the two-storey yellow Wind Library with a tiled roof, a round wind
## window and a wind-mill of cloth sails behind the wind gate; the practice station beside `station.practice.board`
## (a blank slate on an easel, a lectern with an open book, book stacks, a pinwheel); the study-together corner (round
## table, cushion stools, a parasol); a row of individual study desks under a pergola; an arch of giant stacked books
## over the lane; bookshelves, kites, wind chimes and reading cushions. No text or learning content (R2); colours are
## art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "practice"
const THEME_OVERRIDES: Dictionary = {"paving_dark": "wall_yellow", "path": "paving", "hedge": "leaf_dark", "hedge_top": "leaf"}
const PLAZA: Vector3 = Vector3(0, 0, 2.5)
const PLAZA_RADIUS: float = 7.0
const STATION: Vector3 = Vector3(7.8, 0, -0.9)
const CORNER: Vector3 = Vector3(-12.0, 0, -7.0)
const DESKS: Vector3 = Vector3(12.6, 0, -7.6)
const ARCH_Z: float = -13.0
const HOUSE: Array = [Vector3(17.5, 0, 15.5), -90.0, Vector2(5.0, 4.0)]
const LAYOUT: Dictionary = {
	"plaza": [PLAZA, PLAZA_RADIUS],
	"lanes": [[Vector3(0, 0, 0.5), Vector2(2.0, 23.5)], [Vector3(-0.5, 0, -7.0), Vector2(8.5, 1.6)]],
	"pads": [[CORNER, 3.4], [Vector3(0, 0, -20), 3.0]],
}
const BLOCKED: Array = [
	[STATION, Vector2(1.8, 1.8)],
	[CORNER, Vector2(3.2, 3.2)],
	[DESKS, Vector2(3.8, 2.0)],
	[Vector3(0, 0, ARCH_Z), Vector2(4.0, 1.2)],
	[Vector3(17.5, 0, 15.5), Vector2(3.2, 3.2)],
	[Vector3(-16.0, 0, 9.0), Vector2(2.0, 1.0)],
	[Vector3(16.0, 0, 3.0), Vector2(1.0, 2.0)],
	[Vector3(3.2, 0, 11.0), Vector2(1.2, 1.2)],
	[Vector3(0, 0, -31), Vector2(12.0, 5.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_practice_terrain_a": terrain, "env/env_practice_far_a": far, "bld/bld_practice_set_a": buildings,
		"prop/prop_practice_set_a": props, "prop/prop_practice_station_a": practice_station,
		"prop/prop_practice_study_corner_a": study_corner, "bld/bld_practice_desks_a": study_desks,
		"prop/prop_practice_book_arch_a": book_arch,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.lawn(m, th)
	ZoneKit.lanes(m, LAYOUT, String(th["path"]))
	ZoneKit.plaza(m, th, PLAZA, PLAZA_RADIUS)
	ZoneKit.pad(m, th, CORNER, 3.4)
	ZoneKit.pad(m, th, Vector3(0, 0, -20))
	ZoneKit.glow_ring(m, Vector3(0, 0, -20), 3.0)
	ZoneKit.glow_ring(m, Vector3(5, 0, 0), 2.6)
	# stepping stones from the cross path to the study desks
	for p: Vector3 in [Vector3(12.6, 0, -4.9), Vector3(12.6, 0, -3.6)]:
		HubArt._disk(m, p + Vector3.UP * 0.07, 0.45, 10, "stone")
	# a ring of yellow paving around the plaza: the "wind" pattern of six spokes
	for k: int in 6:
		var a: float = TAU * float(k) / 6.0 + 0.3
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		var side: Vector3 = Vector3(-d.z, 0, d.x) * 0.3
		var p0: Vector3 = PLAZA + d * 1.6 + Vector3.UP * 0.088
		var p1: Vector3 = PLAZA + d * 4.6 + Vector3.UP * 0.088
		m.quad(p0 - side, p0 + side, p1 + side * 0.2, p1 - side * 0.2, "wind")
	HubArt._disk(m, PLAZA + Vector3.UP * 0.09, 1.2, 16, "wall_yellow")
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9401
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_wind_library(m)
	m.push_at(HOUSE[0] as Vector3, float(HOUSE[1]))
	ZoneKit.house(m, HOUSE[2] as Vector2, "roof", "roof_dark", "wall_yellow", "flower_white")
	m.pop()
	return m.build()


## Wind Library (Thư viện Gió): stone terrace, a two-storey yellow hall with wooden columns and a two-tier tiled roof,
## a round wind window, two book wings with coloured book spines in their windows and a cloth-sailed wind mill on the
## ridge; wind chimes under the eaves.
static func _wind_library(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 0.3, 0), Vector3(17.0, 0.6, 7.0), "stone", "stone")
	m.box(Vector3(0, 0.08, 3.8), Vector3(4.0, 0.16, 1.0), "stone")
	# main hall, two storeys
	m.box(Vector3(0, 2.4, 0), Vector3(8.0, 3.6, 5.0), "wall_yellow")
	m.box(Vector3(0, 0.85, 0), Vector3(8.06, 0.5, 5.06), "wood")
	for x: float in [-3.7, -1.25, 1.25, 3.7]:
		m.cylinder(Vector3(x, 0.6, 2.75), 0.17, 0.15, 3.6, 8, "wood")
	m.box(Vector3(0, 4.25, 2.6), Vector3(8.6, 0.2, 0.5), "wood_dark")
	m.tiled_roof(Vector3(0, 4.3, 0.1), 8.6, 5.2, 1.6, 0.8, 0.4)
	m.box(Vector3(0, 5.7, 0.1), Vector3(5.0, 1.6, 3.2), "wall_yellow")
	m.tiled_roof(Vector3(0, 6.5, 0.1), 5.4, 3.4, 1.4, 0.55, 0.35)
	# door and the round wind window
	m.box(Vector3(0, 1.75, 2.52), Vector3(1.6, 2.3, 0.1), "wood_dark")
	m.box(Vector3(0, 3.05, 2.54), Vector3(1.9, 0.25, 0.1), "lantern_glow")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 5.75, 1.72)))
	HubArt._torus_ring(m, Vector3.ZERO, 0.62, 0.09, 18, 5, "wood_dark")
	m.pop()
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, 5.75, 1.7)))
	m.cylinder(Vector3(0, -0.05, 0), 0.56, 0.56, 0.1, 16, "wind")
	m.pop()
	for x: float in [-2.6, 2.6]:
		m.box(Vector3(x, 2.1, 2.54), Vector3(1.3, 1.1, 0.1), "wood")
		m.box(Vector3(x, 2.1, 2.57), Vector3(1.05, 0.85, 0.1), "water_deep")
	# book wings with coloured book spines in the windows
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * 6.6, 1.9, 0.4), Vector3(4.2, 2.6, 4.0), "wall")
		m.box(Vector3(s * 6.6, 0.8, 0.4), Vector3(4.26, 0.4, 4.06), "wood")
		m.push_at(Vector3(s * 6.6, 3.2, 0.4), 90.0)
		m.tiled_roof(Vector3.ZERO, 4.4, 4.4, 1.3, 0.5, 0.3)
		m.pop()
		m.box(Vector3(s * 6.6, 2.05, 2.42), Vector3(2.6, 1.3, 0.1), "wood_dark")
		var colours: PackedStringArray = ["cloth_red", "cloth_blue", "cloth_green", "wall_yellow", "cloth_violet", "lantern"]
		for k: int in 12:
			var bx: float = s * 6.6 - 1.15 + k * 0.2
			var h: float = 0.75 + 0.18 * float((k * 7) % 3)
			m.box(Vector3(bx, 1.5 + h * 0.5, 2.48), Vector3(0.17, h, 0.08), colours[k % colours.size()])
	# wind mill of cloth sails on the ridge
	m.box(Vector3(0, 8.0, 0.1), Vector3(0.8, 1.2, 0.8), "wood")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 8.4, 0.62)))
	m.cylinder(Vector3(0, -0.12, 0), 0.18, 0.18, 0.24, 10, "wood_dark")
	for k: int in 4:
		var a: float = PI * 0.5 * k + 0.4
		m.push(Transform3D(Basis(Vector3.BACK, a), Vector3(0, 0, 0.1)))
		m.box(Vector3(0, 1.05, 0), Vector3(0.08, 2.0, 0.08), "wood_dark")
		m.box(Vector3(0.24, 1.25, 0.02), Vector3(0.42, 1.5, 0.05), "flower_white")
		m.pop()
	m.pop()
	# wind chimes under the front eave
	for x: float in [-3.0, -1.0, 1.0, 3.0]:
		_chime(m, Vector3(x, 4.05, 3.1))
	m.pop()


## Wind chime (chuông gió): a little cap, three hanging tubes and a cloth tail.
static func _chime(m: LowPoly, p: Vector3) -> void:
	m.cylinder(p + Vector3.DOWN * 0.12, 0.14, 0.04, 0.12, 8, "bronze")
	for k: int in 3:
		var a: float = TAU * float(k) / 3.0
		m.cylinder(p + Vector3(cos(a) * 0.07, -0.55, sin(a) * 0.07), 0.025, 0.025, 0.4, 6, "metal")
	m.box(p + Vector3(0, -0.85, 0), Vector3(0.12, 0.3, 0.03), "wind")


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Practice station (beside station.practice.board): a blank slate on a wooden easel, a lectern with an open book,
## stacks of books and a pinwheel on a little stone base.
static func practice_station() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.cylinder(Vector3.ZERO, 1.7, 1.65, 0.16, 20, "stone", "stone")
	# easel with the slate
	for x: float in [-0.75, 0.75]:
		HubArt._limb(m, Vector3(x, 0.16, 0.35), Vector3(x * 0.8, 2.4, -0.05), 0.05, 0.04, "wood")
	HubArt._limb(m, Vector3(0, 0.16, -0.7), Vector3(0, 2.2, -0.15), 0.04, 0.04, "wood")
	m.push(Transform3D(Basis(Vector3.RIGHT, -0.18), Vector3(0, 1.55, 0.12)))
	m.box(Vector3.ZERO, Vector3(1.7, 1.15, 0.1), "wood_dark")
	m.box(Vector3(0, 0, 0.03), Vector3(1.5, 0.95, 0.08), "leaf_dark")
	m.box(Vector3(0, -0.66, 0.1), Vector3(1.6, 0.08, 0.14), "wood")
	m.box(Vector3(-0.4, -0.6, 0.13), Vector3(0.16, 0.05, 0.05), "flower_white")
	m.pop()
	# lectern with an open book (blank pages)
	var l: Vector3 = Vector3(-1.15, 0.16, 0.35)
	m.cylinder(l, 0.22, 0.12, 0.95, 8, "wood_dark")
	m.push(Transform3D(Basis(Vector3.RIGHT, 0.35), l + Vector3(0, 1.0, 0)))
	m.box(Vector3.ZERO, Vector3(0.75, 0.08, 0.55), "wood")
	for s: float in [-1.0, 1.0]:
		m.push(Transform3D(Basis(Vector3.BACK, s * 0.12), Vector3(s * 0.17, 0.07, 0)))
		m.box(Vector3.ZERO, Vector3(0.32, 0.05, 0.44), "flower_white")
		m.pop()
	m.box(Vector3(0, 0.05, 0), Vector3(0.7, 0.04, 0.5), "cloth_blue")
	m.pop()
	# book stacks
	var colours: PackedStringArray = ["cloth_red", "cloth_blue", "wall_yellow", "cloth_green", "cloth_violet"]
	for st: Array in [[Vector3(1.15, 0.16, 0.45), 4], [Vector3(1.0, 0.16, -0.5), 3]]:
		var y: float = 0.0
		for k: int in int(st[1]):
			m.push_at(st[0] as Vector3 + Vector3(0, y + 0.08, 0), 12.0 * float((k * 5) % 4))
			m.box(Vector3.ZERO, Vector3(0.62, 0.16, 0.45), colours[(k + int(st[1])) % colours.size()], "flower_white")
			m.pop()
			y += 0.17
	# pinwheel (chong chóng)
	_pinwheel(m, Vector3(-0.6, 0.16, -0.9), 1.6)
	return m.build()


## Pinwheel (chong chóng) on a stick: four closed petals around a hub.
static func _pinwheel(m: LowPoly, p: Vector3, h: float) -> void:
	m.cylinder(p, 0.03, 0.03, h, 6, "wood_dark")
	var c: Vector3 = p + Vector3(0, h, 0.05)
	var colours: PackedStringArray = ["cloth_red", "wall_yellow", "cloth_blue", "cloth_green"]
	for k: int in 4:
		m.push(Transform3D(Basis(Vector3.BACK, PI * 0.5 * k), c))
		m.extrude(PackedVector2Array([Vector2(0, 0), Vector2(0.32, 0.05), Vector2(0.2, 0.3)]), 0.03, colours[k])
		m.pop()
	m.blob(c + Vector3(0, 0, 0.03), Vector3(0.05, 0.05, 0.05), "lantern_glow", 0)


## Study-together corner (góc học cùng nhau): a round table with books, four cushion stools and a striped parasol.
static func study_corner() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.cylinder(Vector3.ZERO, 1.0, 0.95, 0.08, 20, "wood_dark", "wood_dark")
	m.cylinder(Vector3(0, 0.08, 0), 0.12, 0.1, 0.66, 8, "wood_dark")
	m.cylinder(Vector3(0, 0.74, 0), 1.05, 1.05, 0.08, 20, "wood")
	m.box(Vector3(0.3, 0.86, 0.1), Vector3(0.5, 0.06, 0.36), "flower_white")
	m.box(Vector3(-0.35, 0.86, -0.2), Vector3(0.4, 0.1, 0.3), "cloth_red", "flower_white")
	var cushions: PackedStringArray = ["cloth_blue", "cloth_green", "wall_yellow", "cloth_violet"]
	for k: int in 4:
		var a: float = TAU * float(k) / 4.0 + PI * 0.25
		var p: Vector3 = Vector3(cos(a), 0, sin(a)) * 1.6
		m.cylinder(p, 0.32, 0.3, 0.32, 10, "wood")
		m.blob(p + Vector3.UP * 0.4, Vector3(0.36, 0.12, 0.36), cushions[k], 1)
	# parasol: pole and a closed canopy of alternating panels
	m.cylinder(Vector3(0, 0.82, 0), 0.04, 0.04, 1.9, 6, "wood_dark")
	var top: Vector3 = Vector3(0, 3.1, 0)
	for k: int in 8:
		var a0: float = TAU * float(k) / 8.0
		var a1: float = TAU * float(k + 1) / 8.0
		var e0: Vector3 = Vector3(cos(a0) * 1.9, 2.55, sin(a0) * 1.9)
		var e1: Vector3 = Vector3(cos(a1) * 1.9, 2.55, sin(a1) * 1.9)
		var col: String = "flower_white" if k % 2 == 0 else "cloth_green"
		m.tri(e0, top, e1, col)
		m.tri(e1, Vector3(0, 2.7, 0), e0, col)
	m.blob(top + Vector3.UP * 0.05, Vector3(0.1, 0.1, 0.1), "lantern_glow", 0)
	return m.build()


## Individual study desks (bàn học cá nhân): a pergola with a tiled roof over three desks with lamps and stools.
static func study_desks() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.1, 0), Vector3(7.2, 0.2, 3.4), "stone", "stone")
	for x: float in [-3.3, 0.0, 3.3]:
		for z: float in [-1.4, 1.4]:
			m.cylinder(Vector3(x, 0.2, z), 0.12, 0.11, 2.6, 8, "wood")
	m.box(Vector3(0, 2.85, 0), Vector3(7.2, 0.18, 3.2), "wood_dark")
	m.tiled_roof(Vector3(0, 2.9, 0), 7.4, 3.4, 1.2, 0.4, 0.3, "roof", "roof_dark")
	for x: float in [-2.3, 0.0, 2.3]:
		m.box(Vector3(x, 0.85, -0.5), Vector3(1.6, 0.08, 0.8), "wood")
		for dx: float in [-0.7, 0.7]:
			m.box(Vector3(x + dx, 0.5, -0.5), Vector3(0.08, 0.62, 0.7), "wood_dark")
		m.box(Vector3(x - 0.25, 0.92, -0.45), Vector3(0.5, 0.05, 0.36), "flower_white")
		# desk lamp
		m.cylinder(Vector3(x + 0.5, 0.89, -0.7), 0.1, 0.08, 0.04, 8, "metal_dark")
		HubArt._limb(m, Vector3(x + 0.5, 0.93, -0.7), Vector3(x + 0.45, 1.35, -0.6), 0.02, 0.02, "metal_dark")
		m.cylinder(Vector3(x + 0.42, 1.24, -0.55), 0.13, 0.05, 0.16, 8, "wall_yellow")
		m.cylinder(Vector3(x, 0.2, 0.4), 0.22, 0.2, 0.42, 10, "wood")
		m.blob(Vector3(x, 0.66, 0.4), Vector3(0.24, 0.08, 0.24), "cloth_blue", 1)
	return m.build()


## Arch of giant stacked books over the lane (the lane passes under; the stacks stand at x = ±3.0).
static func book_arch() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var colours: PackedStringArray = ["cloth_red", "cloth_blue", "wall_yellow", "cloth_green", "cloth_violet", "leaf_dark"]
	for s: float in [-1.0, 1.0]:
		var y: float = 0.0
		for k: int in 6:
			var h: float = 0.38 + 0.08 * float((k * 2) % 3)
			m.push_at(Vector3(s * 3.0, y + h * 0.5, 0), 8.0 * float((k * 7) % 5) - 16.0)
			m.box(Vector3.ZERO, Vector3(1.2, h, 0.95), colours[(k + (3 if s > 0.0 else 0)) % colours.size()], "flower_white")
			m.box(Vector3(0.0, 0.0, 0.49), Vector3(1.0, h * 0.7, 0.04), "flower_white")
			m.pop()
			y += h + 0.01
	# a giant open book laid face down over the stacks (the arch top): blue covers over thick white page blocks, a spine
	# and a red ribbon
	for s: float in [-1.0, 1.0]:
		m.push(Transform3D(Basis(Vector3.BACK, -s * 0.24), Vector3(s * 1.7, 3.05, 0)))
		m.box(Vector3(0, 0.1, 0), Vector3(3.7, 0.2, 1.6), "cloth_blue")
		m.box(Vector3(-s * 0.05, -0.17, 0), Vector3(3.5, 0.34, 1.42), "flower_white")
		m.pop()
	m.box(Vector3(0, 3.5, 0), Vector3(0.5, 0.3, 1.6), "cloth_blue")
	m.box(Vector3(0.15, 2.75, 0.74), Vector3(0.18, 0.9, 0.05), "lantern")
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9403
	for a: float in [40.0, 140.0, 220.0, 320.0]:
		var p: Vector3 = PLAZA + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "wall_yellow")
	for b: Array in [[Vector3(-5.2, 0.08, 7.0), 40.0], [Vector3(5.2, 0.08, 7.0), -40.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	# bookshelves on the lawn (west) and a reading nook with cushions (east)
	_bookshelf(m, Vector3(-16.0, 0, 9.0), 0.0, rng)
	_bookshelf(m, Vector3(16.0, 0, 3.0), -90.0, rng)
	for c: Array in [[Vector3(-14.0, 0, 6.6), "cloth_violet"], [Vector3(-17.6, 0, 6.4), "cloth_green"], [Vector3(13.6, 0, 1.2), "wall_yellow"]]:
		m.blob(c[0] as Vector3 + Vector3.UP * 0.32, Vector3(0.6, 0.36, 0.55), String(c[1]), 1)
	# kites on poles and pinwheels by the lane
	_kite(m, Vector3(-20.0, 0, -16.0), "cloth_red")
	_kite(m, Vector3(20.5, 0, -18.0), "cloth_blue")
	_kite(m, Vector3(-19.0, 0, 19.0), "wall_yellow")
	for p: Vector3 in [Vector3(-2.9, 0, -4.0), Vector3(2.9, 0, -9.8), Vector3(-2.9, 0, -17.0), Vector3(2.9, 0, 12.0)]:
		_pinwheel(m, p, 1.2)
	ZoneKit.signpost(m, Vector3(3.2, 0, 11.0), [[Vector3(0, 0, -20), "home", "roof"], [Vector3(5, 0, 0), "leaf", "leaf_dark"],
		[CORNER, "book", "cloth_blue"], [DESKS, "star", "wall_yellow"]])
	for p: Vector3 in [Vector3(-3.6, 0, -21.8), Vector3(3.6, 0, -21.8)]:
		HubArt._stone_lantern(m, p)
	ZoneKit.flower_bed(m, Vector3(-7.0, 0, -12.0), 1.4, PackedStringArray(["wall_yellow", "flower_white", "flower_pink"]), rng)
	ZoneKit.flower_bed(m, Vector3(7.0, 0, -12.0), 1.4, PackedStringArray(["cloth_blue", "flower_white", "wall_yellow"]), rng)
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, -21.5), Vector3(-21.8, 0, -3.0)]:
		ZoneKit.rock(m, p, Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)), "stone_dark", rng)
	return m.build()


## Tall bookshelf with coloured books (front along local +Z after `yaw`).
static func _bookshelf(m: LowPoly, p: Vector3, yaw: float, rng: RandomNumberGenerator) -> void:
	m.push_at(p, yaw)
	m.box(Vector3(0, 1.1, 0), Vector3(2.4, 2.2, 0.7), "wood")
	m.box(Vector3(0, 2.3, 0), Vector3(2.6, 0.2, 0.8), "wood_dark")
	var colours: PackedStringArray = ["cloth_red", "cloth_blue", "wall_yellow", "cloth_green", "cloth_violet", "flower_white"]
	for row: int in 3:
		var x: float = -1.0
		while x < 0.95:
			var w: float = rng.randf_range(0.12, 0.2)
			var h: float = rng.randf_range(0.42, 0.56)
			m.box(Vector3(x + w * 0.5, 0.3 + row * 0.66 + h * 0.5, 0.3), Vector3(w * 0.9, h, 0.14), colours[rng.randi() % colours.size()])
			x += w
	m.pop()


## Diamond kite on a tall pole with a ribbon tail.
static func _kite(m: LowPoly, p: Vector3, colour: String) -> void:
	m.cylinder(p, 0.2, 0.16, 0.2, 8, "stone")
	m.cylinder(p + Vector3.UP * 0.2, 0.05, 0.04, 4.2, 6, "wood_dark")
	m.push(Transform3D(Basis(Vector3.BACK, 0.3), p + Vector3(0.5, 4.6, 0)))
	m.extrude(PackedVector2Array([Vector2(0, 0.7), Vector2(0.45, 0), Vector2(0, -0.8), Vector2(-0.45, 0)]), 0.04, colour, "wood_dark")
	for k: int in 3:
		m.box(Vector3(0.1 * k, -0.95 - 0.3 * k, 0), Vector3(0.16, 0.12, 0.04), "wall_yellow" if k % 2 == 0 else "flower_white")
	m.pop()


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"tree": "veg_tree_round_a", "bamboo": "veg_bamboo_a", "bush": "veg_bush_a", "flower": "veg_flower_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"tree":
			return ZoneKit.scatter_in(9410, 8, 22, 1.6, LAYOUT, BLOCKED, 0.85, 1.25)
		"bamboo":
			return ZoneKit.scatter_in(9411, 5, 10, 1.2, LAYOUT, BLOCKED, 0.8, 1.2, Vector2(25.5, 28.0))
		"bush":
			return ZoneKit.scatter_in(9413, 28, 0, 1.0, LAYOUT, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter_in(9414, 100, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter_in(9415, 240, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.4)
	return []
