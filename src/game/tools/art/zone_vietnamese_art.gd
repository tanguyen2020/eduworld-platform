extends RefCounted
## Recipes of "Rừng Ngôn Ngữ — khu Tiếng Việt" (Language Forest, `learning_zone.vietnamese`; F093, WORLD_DESIGN §2.2
## chapter 2 "tìm lại chữ bị lạc, đọc thư cổ", §4.2, §11; ADR-0103). SPDX-License-Identifier: MIT — EduWorld
## first-party, nothing downloaded.
## Set pieces inspired by the Temple of Literature (Văn Miếu): the Khuê Văn Các pavilion with sun windows behind the
## wind gate, a stone stele on a turtle under a small tiled roof (discovery.vietnamese.ancient_letter), the Word Tree
## with paper tags and lanterns (discovery.vietnamese.word_tree), giant calligraphy brushes in blue-and-white pots
## holding a paper scroll over the lane (collectible.vietnamese.page_lost — abstract brush strokes, no text), the
## square Thiên Quang pond, a lotus pond, a writing desk by Mrs Letters, orange paper lanterns, a cream wall with a
## terracotta cap, autumn trees and bamboo. Colours are art palette slots only (R32); collision lives in the scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "vietnamese"
const THEME_OVERRIDES: Dictionary = {"path": "paving", "paving": "paving", "paving_dark": "roof", "hedge": "wall",
	"hedge_top": "roof"}
const STELE_CENTER: Vector3 = Vector3(15.5, 0, -6)
const WORD_TREE_CENTER: Vector3 = Vector3(-15.5, 0, -6)
const BRUSH_X: float = 2.9
const SQUARE_POND: Vector3 = Vector3(-11, 0, 15)
const SQUARE_POND_HALF: float = 2.8
const LOTUS_POND: Vector3 = Vector3(13, 0, 13.5)
const HOUSE: Array = [Vector3(18.0, 0, -16.0), -90.0, Vector2(5.0, 4.0)]
const BLOCKED: Array = [
	[Vector3(15.5, 0, -6), Vector2(2.6, 2.6)],
	[Vector3(-15.5, 0, -6), Vector2(2.6, 2.6)],
	[SQUARE_POND, Vector2(3.6, 3.6)],
	[LOTUS_POND, Vector2(3.4, 3.4)],
	[Vector3(18.0, 0, -16.0), Vector2(3.2, 3.2)],
	[Vector3(0, 0, -31), Vector2(10.0, 4.0)],
	[Vector3(-7.5, 0, -12.0), Vector2(1.6, 1.6)],
	[Vector3(7.5, 0, -12.0), Vector2(1.6, 1.6)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_vietnamese_terrain_a": terrain, "env/env_vietnamese_far_a": far, "bld/bld_vietnamese_set_a": buildings, "prop/prop_vietnamese_set_a": props,
		"prop/prop_vietnamese_stele_a": stele_turtle, "prop/prop_vietnamese_word_tree_a": word_tree,
		"prop/prop_vietnamese_brushes_a": brush_scroll, "veg/veg_tree_autumn_a": autumn_tree,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9321
	var th: Dictionary = theme()
	ZoneKit.terrain(m, th)
	ZoneKit.path_strip(m, Vector3(15.0, 0, -7.2), Vector3(17.5, 0, -4.8), "paving")
	ZoneKit.path_strip(m, Vector3(-17.5, 0, -7.2), Vector3(-15.0, 0, -4.8), "paving")
	# brick border of the lane (gạch Bát Tràng)
	for s: float in [-1.0, 1.0]:
		ZoneKit.path_strip(m, Vector3(s * 2.0 - 0.18, 0, -23.0), Vector3(s * 2.0 + 0.18, 0, -2.6), "roof", 0.05)
		ZoneKit.path_strip(m, Vector3(s * 2.0 - 0.18, 0, 10.6), Vector3(s * 2.0 + 0.18, 0, 24.0), "roof", 0.05)
	# fallen autumn leaves on the lawn
	for i: int in 16:
		var p: Vector3 = Vector3(rng.randf_range(-22.0, 22.0), 0.03, rng.randf_range(-22.0, 22.0))
		if ZoneKit.clear(p, 0.8, BLOCKED):
			HubArt._disk(m, p, rng.randf_range(0.55, 0.95), 7, "leaf_autumn" if i % 3 else "wall_yellow")
	_square_pond(m)
	_lotus_pond(m, rng)
	ZoneKit.boundary(m, th)
	return m.build()


## Thiên Quang pond (giếng Thiên Quang): a square pool with a stone rim and round balusters.
static func _square_pond(m: LowPoly) -> void:
	var c: Vector3 = SQUARE_POND
	var h: float = SQUARE_POND_HALF
	m.quad(c + Vector3(-h, 0.06, -h), c + Vector3(-h, 0.06, h), c + Vector3(h, 0.06, h), c + Vector3(h, 0.06, -h), "water")
	m.quad(c + Vector3(-h + 0.7, 0.065, -h + 0.7), c + Vector3(-h + 0.7, 0.065, h - 0.7), c + Vector3(h - 0.7, 0.065, h - 0.7),
			c + Vector3(h - 0.7, 0.065, -h + 0.7), "water_deep")
	for s: float in [-1.0, 1.0]:
		m.box(c + Vector3(s * (h + 0.2), 0.22, 0), Vector3(0.4, 0.44, 2.0 * h + 0.8), "stone", "stone")
		m.box(c + Vector3(0, 0.22, s * (h + 0.2)), Vector3(2.0 * h, 0.44, 0.4), "stone", "stone")
		for k: int in 5:
			var t: float = lerpf(-h - 0.2, h + 0.2, float(k) / 4.0)
			m.cylinder(c + Vector3(s * (h + 0.2), 0.44, t), 0.12, 0.12, 0.42, 8, "stone_dark")
			m.cylinder(c + Vector3(t, 0.44, s * (h + 0.2)), 0.12, 0.12, 0.42, 8, "stone_dark")
	HubArt._lotus(m, c + Vector3(-1.0, 0.08, 0.6))
	HubArt._disk(m, c + Vector3(-1.0, 0.07, 0.6), 0.4, 8, "leaf")
	HubArt._disk(m, c + Vector3(0.9, 0.07, -0.8), 0.34, 8, "leaf")


static func _lotus_pond(m: LowPoly, rng: RandomNumberGenerator) -> void:
	var c: Vector3 = LOTUS_POND
	HubArt._ring(m, c, 2.6, 3.0, 0.35, 20, "stone")
	HubArt._disk(m, c + Vector3.UP * 0.22, 2.6, 20, "water")
	for i: int in 6:
		var a: float = TAU * float(i) / 6.0 + rng.randf_range(-0.3, 0.3)
		var p: Vector3 = c + Vector3(cos(a) * 1.6, 0.24, sin(a) * 1.6)
		HubArt._disk(m, p, rng.randf_range(0.3, 0.45), 8, "leaf")
		if i % 2 == 0:
			HubArt._lotus(m, p + Vector3.UP * 0.02)


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9329
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_khue_van_cac(m)
	m.push_at(HOUSE[0] as Vector3, float(HOUSE[1]))
	ZoneKit.house(m, HOUSE[2] as Vector2, "roof", "roof_dark", "wall_yellow", "subject_vietnamese")
	m.pop()
	return m.build()


## Khuê Văn Các (Constellation of Literature pavilion): four cream brick pillars on a stone terrace carrying a wooden
## upper storey with round sun windows on every side and a two-tier curled tiled roof, between two low walls with
## small gates.
static func _khue_van_cac(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 0.25, 0), Vector3(8.6, 0.5, 6.6), "stone", "stone")
	m.box(Vector3(0, 0.55, 3.6), Vector3(3.2, 0.2, 1.2), "stone")
	for x: float in [-2.1, 2.1]:
		for z: float in [-1.7, 1.7]:
			m.box(Vector3(x, 2.25, z), Vector3(1.4, 3.5, 1.4), "wall")
			m.box(Vector3(x, 0.75, z), Vector3(1.55, 0.5, 1.55), "roof_dark")
			m.box(Vector3(x, 3.85, z), Vector3(1.6, 0.3, 1.6), "roof_dark")
	# arches between the pillars (front and back): a beam with a rounded underside
	for z: float in [-1.7, 1.7]:
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 2.6, z)))
		m.extrude(PackedVector2Array([Vector2(-1.4, 1.4), Vector2(1.4, 1.4), Vector2(1.4, 0.2), Vector2(0.9, 0.75),
			Vector2(0.0, 0.95), Vector2(-0.9, 0.75), Vector2(-1.4, 0.2)]), 1.2, "wall")
		m.pop()
	# upper storey: floor, balcony rail, wooden walls, sun windows
	m.box(Vector3(0, 4.15, 0), Vector3(6.4, 0.3, 5.4), "wood_dark")
	m.box(Vector3(0, 5.3, 0), Vector3(4.4, 2.0, 3.6), "wood")
	for side: int in 4:
		m.push_at(Vector3(0, 0, 0), 90.0 * side)
		var depth: float = 1.81 if side % 2 == 0 else 2.21
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 5.3, depth)))
		m.extrude(ZoneKit.circle(0.72, 16), 0.08, "wood_dark")
		m.pop()
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 5.3, depth + 0.05)))
		m.extrude(ZoneKit.circle(0.58, 16), 0.06, "lantern_glow")
		m.pop()
		for k: int in 8:
			var a: float = TAU * float(k) / 8.0
			m.push(Transform3D(Basis(Vector3.BACK, a), Vector3(0, 5.3, depth + 0.1)))
			m.box(Vector3(0.3, 0, 0), Vector3(0.55, 0.05, 0.04), "wood_dark")
			m.pop()
		# balcony rail
		var edge: float = 2.6 if side % 2 == 0 else 3.1
		m.box(Vector3(0, 4.75, edge), Vector3(5.6 if side % 2 == 0 else 4.6, 0.08, 0.08), "wood_dark")
		for k: int in 7:
			var t: float = lerpf(-2.6 if side % 2 == 0 else -2.1, 2.6 if side % 2 == 0 else 2.1, float(k) / 6.0)
			m.box(Vector3(t, 4.5, edge), Vector3(0.07, 0.5, 0.07), "wood_dark")
		m.pop()
	m.tiled_roof(Vector3(0, 6.3, 0), 5.6, 4.6, 1.2, 0.7, 0.45)
	m.box(Vector3(0, 7.65, 0), Vector3(3.0, 0.7, 2.4), "wood")
	m.tiled_roof(Vector3(0, 8.0, 0), 3.4, 2.8, 1.3, 0.5, 0.35)
	m.blob(Vector3(0, 9.55, 0), Vector3(0.3, 0.3, 0.3), "lantern_glow", 1)
	# side walls with small gates (tam quan)
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * 7.0, 1.2, 0.6), Vector3(4.6, 2.4, 0.8), "wall")
		m.box(Vector3(s * 7.0, 2.5, 0.6), Vector3(4.9, 0.25, 1.1), "roof")
		m.box(Vector3(s * 9.8, 1.9, 0.6), Vector3(1.0, 3.8, 1.0), "wall")
		m.cylinder(Vector3(s * 9.8, 3.8, 0.6), 0.55, 0.05, 0.9, 4, "roof_dark", "", false, 0.5)
		m.push(Transform3D(Basis.IDENTITY, Vector3(s * 6.4, 0.0, 1.02)))
		m.extrude(ZoneKit.arch(0.65, 1.9, 8), 0.06, "wood_dark")
		m.pop()
	m.pop()


# ---------- set pieces at the anchors (local coordinates, placed by the zone scene; front along local +Z) ----------

## Stele on a stone turtle (bia đá trên lưng rùa) under a small tiled pavilion: the "ancient letter" discovery.
## The stele carries a raised border and a sun disc, no writing (no learning content in the scene, R2).
static func stele_turtle() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.15, 0), Vector3(3.2, 0.3, 4.0), "stone", "stone")
	# turtle
	m.blob(Vector3(0, 0.62, 0), Vector3(1.15, 0.2, 1.45), "stone", 1)
	m.blob(Vector3(0, 0.78, -0.05), Vector3(1.02, 0.5, 1.3), "bronze_patina", 1)
	for k: int in 6:
		var a: float = TAU * float(k) / 6.0
		m.blob(Vector3(cos(a) * 0.55, 1.12, sin(a) * 0.7), Vector3(0.3, 0.1, 0.3), "stone", 0)
	m.blob(Vector3(0, 0.62, 1.4), Vector3(0.24, 0.22, 0.34), "bronze_patina", 1)
	m.blob(Vector3(0, 0.72, 1.72), Vector3(0.3, 0.27, 0.32), "bronze_patina", 1)
	for x: float in [-0.13, 0.13]:
		m.blob(Vector3(x, 0.8, 1.98), Vector3(0.05, 0.05, 0.05), "eye", 0)
	for x: float in [-0.85, 0.85]:
		for z: float in [-0.85, 0.75]:
			m.blob(Vector3(x, 0.42, z), Vector3(0.3, 0.16, 0.36), "bronze_patina", 0)
	# stele
	m.box(Vector3(0, 1.32, -0.05), Vector3(1.2, 0.3, 0.55), "stone_dark")
	m.box(Vector3(0, 2.3, -0.05), Vector3(1.05, 1.7, 0.24), "stone")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 3.15, -0.05)))
	var top: PackedVector2Array = PackedVector2Array()
	for i: int in 9:
		var a: float = PI * float(i) / 8.0
		top.append(Vector2(cos(a) * 0.525, sin(a) * 0.45))
	m.extrude(top, 0.24, "stone")
	m.pop()
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * 0.43, 2.25, 0.09), Vector3(0.06, 1.4, 0.04), "stone_dark")
	m.box(Vector3(0, 1.55, 0.09), Vector3(0.92, 0.06, 0.04), "stone_dark")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 3.2, 0.08)))
	m.extrude(ZoneKit.circle(0.2, 12), 0.04, "stone_dark")
	m.pop()
	# pavilion (nhà bia)
	for x: float in [-1.4, 1.4]:
		for z: float in [-1.75, 1.75]:
			m.cylinder(Vector3(x, 0.3, z), 0.13, 0.12, 3.2, 8, "wood")
	m.box(Vector3(0, 3.45, 0), Vector3(3.1, 0.18, 3.8), "wood_dark")
	m.push_at(Vector3(0, 3.55, 0), 90.0)
	m.tiled_roof(Vector3.ZERO, 4.0, 3.2, 1.3, 0.55, 0.35)
	m.pop()
	return m.build()


## Word Tree (cây chữ): an old tree with a stone seat ring, a gold-and-orange canopy, paper tags on strings and a few
## red lanterns hanging from it.
static func word_tree() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9322
	HubArt._ring(m, Vector3.ZERO, 1.3, 1.85, 0.45, 16, "stone")
	HubArt._disk(m, Vector3(0, 0.3, 0), 1.3, 16, "soil")
	m.lathe(Vector3(0, 0.25, 0), PackedVector2Array([Vector2(0.95, 0.0), Vector2(0.62, 0.55), Vector2(0.5, 1.8), Vector2(0.56, 2.9),
		Vector2(0.78, 3.4)]), 10, "trunk")
	for k: int in 3:
		var a: float = TAU * (float(k) + 0.2) / 3.0
		var d: Vector3 = Vector3(cos(a), 0, sin(a))
		HubArt._limb(m, Vector3(0, 3.3, 0) + d * 0.3, Vector3(0, 4.8, 0) + d * 2.1, 0.3, 0.15, "trunk")
	var crowns: Array = [[Vector3(0, 5.6, 0), Vector3(3.0, 1.7, 2.9), "leaf_autumn"],
		[Vector3(2.0, 5.0, 0.8), Vector3(2.0, 1.3, 1.9), "wall_yellow"],
		[Vector3(-2.0, 5.1, 0.6), Vector3(2.1, 1.3, 2.0), "leaf_autumn"],
		[Vector3(0.3, 4.9, -2.0), Vector3(2.0, 1.2, 1.8), "rice_ripe"],
		[Vector3(0.2, 6.6, 0.2), Vector3(1.8, 1.1, 1.7), "wall_yellow"]]
	for i: int in crowns.size():
		var c: Array = crowns[i]
		# the two big crowns smooth (subdivision 2), the small ones at base detail (PERFORMANCE_BUDGET §7)
		m.blob(c[0] as Vector3, c[1] as Vector3, String(c[2]), 1 if i < 2 else 0, 0.03, rng)
	# hanging paper tags and lanterns (small: base detail)
	m.blob_detail = 0
	var tags: PackedStringArray = ["flower_white", "wall", "wall_yellow", "flower_white"]
	for k: int in 14:
		var a: float = TAU * float(k) / 14.0 + rng.randf_range(-0.15, 0.15)
		var r: float = rng.randf_range(1.6, 2.7)
		var top: Vector3 = Vector3(cos(a) * r, rng.randf_range(4.0, 4.4), sin(a) * r)
		var length: float = rng.randf_range(0.5, 0.9)
		m.cylinder(top - Vector3.UP * length, 0.012, 0.012, length, 3, "wood_dark", "", false)
		m.push_at(top - Vector3.UP * (length + 0.17), rad_to_deg(a))
		m.box(Vector3.ZERO, Vector3(0.2, 0.34, 0.03), tags[k % tags.size()])
		m.box(Vector3(0, 0.05, 0.02), Vector3(0.12, 0.03, 0.01), "eye")
		m.box(Vector3(0, -0.04, 0.02), Vector3(0.09, 0.03, 0.01), "eye")
		m.pop()
	for k: int in 4:
		var a: float = TAU * (float(k) + 0.5) / 4.0
		var p: Vector3 = Vector3(cos(a) * 2.2, 3.7, sin(a) * 2.2)
		m.cylinder(p + Vector3.UP * 0.3, 0.012, 0.012, 0.5, 3, "wood_dark", "", false)
		m.blob(p, Vector3(0.22, 0.28, 0.22), "lantern", 1)
		m.cylinder(p + Vector3(0, -0.32, 0), 0.1, 0.1, 0.07, 6, "lantern_glow")
	return m.build()


## Two giant calligraphy brushes standing in blue-and-white pots, a paper scroll with abstract brush strokes and a red
## seal hanging between them over the lane (collectible.vietnamese.page_lost under it). No characters (R2).
static func brush_scroll() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	for s: float in [-1.0, 1.0]:
		var x: float = s * BRUSH_X
		m.lathe(Vector3(x, 0, 0), PackedVector2Array([Vector2(0.42, 0.0), Vector2(0.58, 0.25), Vector2(0.62, 0.65), Vector2(0.5, 0.95),
			Vector2(0.54, 1.05)]), 12, "flower_white")
		HubArt._ring(m, Vector3(x, 0.4, 0), 0.6, 0.65, 0.22, 12, "cloth_blue")
		HubArt._ring(m, Vector3(x, 0.98, 0), 0.5, 0.56, 0.08, 12, "cloth_blue")
		HubArt._disk(m, Vector3(x, 0.95, 0), 0.5, 12, "soil")
		m.cylinder(Vector3(x, 0.9, 0), 0.13, 0.12, 3.0, 8, "bamboo")
		for y: float in [1.6, 2.4, 3.2]:
			m.cylinder(Vector3(x, y, 0), 0.135, 0.135, 0.05, 8, "leaf", "", false)
		m.cylinder(Vector3(x, 3.9, 0), 0.16, 0.16, 0.25, 8, "wood_dark")
		m.lathe(Vector3(x, 4.15, 0), PackedVector2Array([Vector2(0.17, 0.0), Vector2(0.24, 0.25), Vector2(0.18, 0.6), Vector2(0.0, 0.95)]),
				10, "eye")
	# rope between the brushes, scroll with two dowels, strokes and the red seal
	m.push(Transform3D(Basis(Vector3.BACK, -PI * 0.5), Vector3(-BRUSH_X, 3.75, 0)))
	m.cylinder(Vector3.ZERO, 0.025, 0.025, 2.0 * BRUSH_X, 4, "basket", "", false)
	m.pop()
	for x: float in [-1.3, 1.3]:
		m.cylinder(Vector3(x, 3.45, 0), 0.015, 0.015, 0.3, 3, "basket", "", false)
	m.box(Vector3(0, 2.85, 0), Vector3(3.2, 1.15, 0.04), "flower_white")
	for y: float in [3.45, 2.27]:
		m.push(Transform3D(Basis(Vector3.BACK, -PI * 0.5), Vector3(-1.75, y, 0)))
		m.cylinder(Vector3.ZERO, 0.06, 0.06, 3.5, 8, "wood_dark")
		m.pop()
	var strokes: Array = [[Vector3(-0.9, 3.0, 0.03), 0.6, 0.25], [Vector3(-0.35, 2.75, 0.03), 0.45, -0.5], [Vector3(0.25, 3.05, 0.03), 0.5, 0.1],
		[Vector3(0.65, 2.7, 0.03), 0.35, 0.9]]
	for st: Array in strokes:
		m.push(Transform3D(Basis(Vector3.BACK, float(st[2])), st[0] as Vector3))
		m.box(Vector3.ZERO, Vector3(float(st[1]), 0.09, 0.02), "eye")
		m.pop()
	m.box(Vector3(1.25, 2.55, 0.03), Vector3(0.22, 0.22, 0.02), "lantern")
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9323
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = ZoneKit.PLAZA_CENTER + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (ZoneKit.PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "subject_vietnamese")
	for z: float in [-10.0, -21.5]:
		for s: float in [-1.0, 1.0]:
			ZoneKit.lamp(m, Vector3(s * 3.1, 0, z), "wood_dark", "subject_vietnamese")
	for b: Array in [[Vector3(-4.6, 0.08, 6.6), 50.0], [Vector3(4.6, 0.08, 6.6), -50.0]]:
		_stone_bench(m, b[0] as Vector3, float(b[1]))
	_writing_desk(m, Vector3(2.8, 0.08, 0.6), ZoneKit.yaw_to(Vector3(2.8, 0, 0.6), ZoneKit.SPAWN))
	# blue-and-white jars (chum) and stone lanterns along the lane
	for p: Vector3 in [Vector3(-5.4, 0, -1.8), Vector3(5.8, 0, -2.4), Vector3(-4.2, 0, 12.5), Vector3(4.4, 0, 13.0)]:
		_jar(m, p, rng)
	for p: Vector3 in [Vector3(-3.6, 0, -16.5), Vector3(3.6, 0, -16.5)]:
		HubArt._stone_lantern(m, p)
	ZoneKit.flower_bed(m, Vector3(-7.5, 0, -12.0), 1.4, PackedStringArray(["subject_vietnamese", "lantern_glow", "flower_white"]), rng)
	ZoneKit.flower_bed(m, Vector3(7.5, 0, -12.0), 1.4, PackedStringArray(["flower_pink", "subject_vietnamese", "flower_white"]), rng)
	ZoneKit.bench(m, Vector3(-11.0, 0.0, 19.6), 0.0)
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, 21.5), Vector3(-21.8, 0, -21.6), Vector3(9.0, 0, -21.8)]:
		m.blob(p + Vector3(0, 0.2, 0), Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)),
				"stone_dark", 0, 0.15, rng)
	return m.build()


static func _stone_bench(m: LowPoly, p: Vector3, yaw: float) -> void:
	m.push_at(p, yaw)
	m.box(Vector3(0, 0.42, 0), Vector3(1.8, 0.16, 0.55), "stone", "stone")
	for x: float in [-0.65, 0.65]:
		m.box(Vector3(x, 0.17, 0), Vector3(0.3, 0.34, 0.45), "stone_dark")
	m.pop()


## Low writing desk (án thư) with an open scroll, an ink stone, a brush and a seal box.
static func _writing_desk(m: LowPoly, p: Vector3, yaw: float) -> void:
	m.push_at(p, yaw)
	m.box(Vector3(0, 0.5, 0), Vector3(1.5, 0.1, 0.7), "wood_dark")
	for x: float in [-0.65, 0.65]:
		m.box(Vector3(x, 0.24, 0), Vector3(0.1, 0.48, 0.6), "wood_dark")
		m.box(Vector3(x, 0.03, 0), Vector3(0.18, 0.06, 0.7), "wood_dark")
	m.box(Vector3(-0.15, 0.56, 0.02), Vector3(0.8, 0.02, 0.45), "flower_white")
	for x: float in [-0.56, 0.26]:
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(x, 0.58, -0.25)))
		m.cylinder(Vector3.ZERO, 0.04, 0.04, 0.5, 6, "wood")
		m.pop()
	m.box(Vector3(0.5, 0.57, 0.05), Vector3(0.22, 0.05, 0.3), "stone_dark")
	m.push(Transform3D(Basis(Vector3.BACK, PI * 0.5), Vector3(0.5, 0.62, -0.2)))
	m.cylinder(Vector3.ZERO, 0.02, 0.02, 0.4, 4, "bamboo")
	m.pop()
	m.box(Vector3(-0.15, 0.58, 0.05), Vector3(0.36, 0.02, 0.05), "eye")
	m.box(Vector3(0.1, 0.58, -0.08), Vector3(0.05, 0.02, 0.2), "eye")
	m.box(Vector3(0.62, 0.6, 0.22), Vector3(0.12, 0.08, 0.12), "lantern")
	m.pop()


## Blue-and-white ceramic jar (chum gốm) with a lid.
static func _jar(m: LowPoly, p: Vector3, rng: RandomNumberGenerator) -> void:
	var s: float = rng.randf_range(0.85, 1.1)
	m.lathe(p, PackedVector2Array([Vector2(0.25, 0.0) * s, Vector2(0.45, 0.25) * s, Vector2(0.48, 0.6) * s, Vector2(0.3, 0.92) * s,
		Vector2(0.32, 1.0) * s]), 10, "flower_white")
	HubArt._ring(m, p + Vector3.UP * 0.45 * s, 0.46 * s, 0.5 * s, 0.14 * s, 10, "cloth_blue")
	m.cylinder(p + Vector3.UP * 1.0 * s, 0.34 * s, 0.2 * s, 0.1 * s, 10, "cloth_blue")


# ---------- vegetation ----------

## Autumn tree of the language forest: trunk and a round orange / gold canopy (repeated: base detail, ≤ 2k).
static func autumn_tree() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9324
	m.cylinder(Vector3.ZERO, 0.22, 0.16, 2.1, 6, "trunk")
	m.blob(Vector3(0, 2.75, 0), Vector3(1.4, 1.25, 1.4), "leaf_autumn", 1, 0.03, rng)
	m.blob(Vector3(-0.55, 2.45, 0.45), Vector3(0.85, 0.75, 0.85), "wall_yellow", 0, 0.03, rng)
	m.blob(Vector3(0.5, 3.35, 0.2), Vector3(0.85, 0.72, 0.85), "rice_ripe", 0, 0.03, rng)
	return m.build()


const SCATTER: Dictionary = {
	"autumn": "veg_tree_autumn_a", "tree": "veg_tree_round_a", "bamboo": "veg_bamboo_a", "bush": "veg_bush_a",
	"flower": "veg_flower_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"autumn":
			return ZoneKit.scatter(9330, 7, 16, 1.6, BLOCKED, 0.85, 1.25)
		"tree":
			return ZoneKit.scatter(9331, 2, 8, 1.6, BLOCKED, 0.85, 1.2)
		"bamboo":
			return ZoneKit.scatter(9332, 4, 8, 1.2, BLOCKED, 0.8, 1.2, Vector2(25.5, 28.0))
		"bush":
			return ZoneKit.scatter(9333, 30, 0, 1.0, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter(9334, 80, 0, 0.3, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter(9335, 220, 0, 0.3, BLOCKED, 0.8, 1.4)
	return []
