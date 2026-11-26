extends RefCounted
## Recipes of "Tháp Thử Thách" (Trial Tower, `exam_zone`; F094, WORLD_DESIGN §2.3 Giám khảo Đá, §2.4, §4.4 "UI tối
## giản … phòng Gương Phản Chiếu", §11; EXAM_ENGINE; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party,
## nothing downloaded.
## A calm, focused courtyard on the mountain (F094-AC-03: soft blues, creams, stone and moss — no bright red or orange on
## the landmark and set pieces): the Trial Tower of four tiers with glazed blue roofs and round windows behind the wind
## gate; the exam sessions board beside `board.exam_sessions` (a clock face and a blank calendar grid under a small blue
## roof); the still Reflection Pool ("Gương Phản Chiếu") with a round stone mirror frame; a raked gravel garden with
## mossy rocks; a quiet study pavilion; stone lanterns, bamboo fences and pines. No text (R2); colours are art palette
## slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "exam"
const THEME_OVERRIDES: Dictionary = {"lawn": "hill", "lawn_alt": "grass_dark", "paving": "stone", "paving_dark": "tile_blue",
	"curb": "stone_dark", "hedge": "leaf_dark", "hedge_top": "hill", "far": "hill_far"}
const PLAZA: Vector3 = Vector3(0, 0, 3)
const PLAZA_RADIUS: float = 7.0
const BOARD: Vector3 = Vector3(4, 0, 0)
const STAND: Vector3 = Vector3(6.6, 0, -0.9)
const POOL: Vector3 = Vector3(-12.5, 0, -8.0)
const POOL_RADIUS: float = 2.7
const GARDEN: Vector3 = Vector3(12.5, 0, -8.0)
const PAVILION: Vector3 = Vector3(-13.0, 0, 11.5)
const LAYOUT: Dictionary = {
	"plaza": [PLAZA, PLAZA_RADIUS],
	"lanes": [[Vector3(0, 0, 0.5), Vector2(1.8, 23.5)], [Vector3(0, 0, -8.0), Vector2(9.0, 1.4)]],
	"pads": [[POOL, 4.0], [GARDEN, 3.6], [Vector3(0, 0, -20), 3.0]],
}
const BLOCKED: Array = [
	[STAND, Vector2(1.8, 1.4)],
	[POOL, Vector2(4.2, 4.2)],
	[GARDEN, Vector2(3.8, 3.0)],
	[PAVILION, Vector2(3.4, 3.4)],
	[Vector3(3.0, 0, 11.0), Vector2(1.2, 1.2)],
	[Vector3(0, 0, -31), Vector2(12.0, 6.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_exam_terrain_a": terrain, "env/env_exam_far_a": far, "bld/bld_exam_set_a": buildings,
		"prop/prop_exam_set_a": props, "prop/prop_exam_board_stand_a": board_stand,
		"prop/prop_exam_reflection_pool_a": reflection_pool, "prop/prop_exam_gravel_garden_a": gravel_garden,
		"bld/bld_exam_study_pavilion_a": study_pavilion, "veg/veg_pine_a": ZoneKit.pine,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.lawn(m, th)
	ZoneKit.lanes(m, LAYOUT, "path")
	# stone slabs along the gravel lane
	var z: float = -22.5
	while z < 23.0:
		if absf(z - PLAZA.z) > PLAZA_RADIUS + 0.4:
			ZoneKit.path_strip(m, Vector3(-0.7, 0, z), Vector3(0.7, 0, z + 0.9), "stone", 0.05)
		z += 1.5
	ZoneKit.plaza(m, th, PLAZA, PLAZA_RADIUS)
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		HubArt._disk(m, PLAZA + Vector3(cos(a), 0, sin(a)) * 3.2 + Vector3.UP * 0.09, 0.3, 8, "tile_blue")
	ZoneKit.pad(m, th, POOL, 4.0)
	ZoneKit.pad(m, th, GARDEN, 3.8)
	ZoneKit.glow_ring(m, BOARD, 2.4)
	ZoneKit.pad(m, th, Vector3(0, 0, -20))
	ZoneKit.glow_ring(m, Vector3(0, 0, -20), 3.0)
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, a rise and soft hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9481
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_trial_tower(m)
	return m.build()


## Trial Tower: a stone terrace with stairs, four diminishing cream tiers with glazed blue roofs and round windows, a
## pale spark finial, two small flanking pavilions and calm blue pennants.
static func _trial_tower(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP + Vector3(0, 0, -1.0))
	m.box(Vector3(0, 0.35, 0), Vector3(15.0, 0.7, 7.5), "stone", "stone")
	for k: int in 3:
		m.box(Vector3(0, 0.1 + k * 0.22, 4.1 - k * 0.3), Vector3(3.6, 0.2, 0.4), "stone_dark", "stone")
	var y: float = 0.7
	var tiers: Array = [[5.6, 2.6], [4.6, 2.3], [3.6, 2.0], [2.6, 1.8]]
	for k: int in tiers.size():
		var w: float = float((tiers[k] as Array)[0])
		var h: float = float((tiers[k] as Array)[1])
		m.box(Vector3(0, y + h * 0.5, 0), Vector3(w, h, w), "wall")
		m.box(Vector3(0, y + 0.2, 0), Vector3(w + 0.06, 0.4, w + 0.06), "stone")
		for side: int in 4:
			m.push_at(Vector3.ZERO, 90.0 * side)
			m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, y + h * 0.55, w * 0.5 + 0.02)))
			m.cylinder(Vector3(0, -0.05, 0), 0.42 - 0.05 * k, 0.42 - 0.05 * k, 0.1, 12, "metal_dark")
			m.cylinder(Vector3(0, 0.0, 0), 0.32 - 0.05 * k, 0.32 - 0.05 * k, 0.1, 12, "wind")
			m.pop()
			m.pop()
		m.tiled_roof(Vector3(0, y + h, 0), w + 0.4, w + 0.4, 0.9, 0.7, 0.35, "tile_blue", "tile_blue_dark")
		y += h + 0.8
	m.box(Vector3(0, 1.75, 2.82), Vector3(1.4, 2.0, 0.1), "wood_dark")
	ZoneKit.crystal(m, Vector3(0, y - 0.2, 0), 0.35, 1.5, 6, "spark", 0.35)
	for s: float in [-1.0, 1.0]:
		var c: Vector3 = Vector3(s * 5.6, 0.7, 0.8)
		for x: float in [-0.9, 0.9]:
			for z2: float in [-0.9, 0.9]:
				m.cylinder(c + Vector3(x, 0, z2), 0.1, 0.09, 1.9, 8, "wood")
		m.tiled_roof(c + Vector3(0, 1.9, 0), 2.4, 2.4, 0.9, 0.35, 0.3, "tile_blue", "tile_blue_dark")
		ZoneKit.pennant(m, Vector3(s * 7.0, 0.7, 3.2), "cloth_blue", 3.4, -90.0 if s < 0.0 else 90.0)
	m.pop()


## Quiet study pavilion: a raised wooden floor, four posts, a glazed blue roof, two low desks with books and cushions.
static func study_pavilion() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.2, 0), Vector3(4.6, 0.4, 4.2), "wood", "wood")
	m.box(Vector3(0, 0.1, 2.4), Vector3(1.6, 0.2, 0.6), "stone", "stone")
	for x: float in [-2.0, 2.0]:
		for z: float in [-1.8, 1.8]:
			m.cylinder(Vector3(x, 0.4, z), 0.12, 0.11, 2.2, 8, "wood_dark")
	m.box(Vector3(0, 2.65, 0), Vector3(4.4, 0.16, 4.0), "wood_dark")
	m.tiled_roof(Vector3(0, 2.7, 0), 4.8, 4.4, 1.3, 0.5, 0.35, "tile_blue", "tile_blue_dark")
	for x: float in [-1.0, 1.0]:
		m.box(Vector3(x, 0.65, -0.3), Vector3(1.2, 0.08, 0.6), "wood")
		for dx: float in [-0.5, 0.5]:
			m.box(Vector3(x + dx, 0.52, -0.3), Vector3(0.08, 0.2, 0.5), "wood_dark")
		m.box(Vector3(x - 0.2, 0.72, -0.3), Vector3(0.42, 0.04, 0.3), "flower_white")
		m.blob(Vector3(x, 0.5, 0.4), Vector3(0.3, 0.09, 0.3), "cloth_blue", 1)
	return m.build()


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Exam sessions board (beside board.exam_sessions): stone posts, a wooden board with a round clock face and a blank
## 4 × 3 calendar grid (one day marked in soft blue), a small glazed blue roof and two stone lanterns.
static func board_stand() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	for x: float in [-1.5, 1.5]:
		m.box(Vector3(x, 1.3, 0), Vector3(0.28, 2.6, 0.28), "stone")
	m.box(Vector3(0, 1.55, 0), Vector3(2.9, 1.8, 0.14), "wood")
	m.box(Vector3(0, 1.55, 0.05), Vector3(2.7, 1.6, 0.08), "wall")
	m.tiled_roof(Vector3(0, 2.6, 0), 3.4, 0.6, 0.45, 0.25, 0.15, "tile_blue", "tile_blue_dark")
	# clock face
	var c: Vector3 = Vector3(-0.75, 1.6, 0.1)
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), c))
	m.cylinder(Vector3(0, -0.03, 0), 0.6, 0.6, 0.06, 20, "tile_blue_dark")
	m.cylinder(Vector3(0, 0.0, 0), 0.52, 0.52, 0.06, 20, "flower_white")
	m.pop()
	for k: int in 12:
		var a: float = TAU * float(k) / 12.0
		m.box(c + Vector3(cos(a) * 0.42, sin(a) * 0.42, 0.05), Vector3(0.05, 0.05, 0.02), "tile_blue_dark")
	m.push(Transform3D(Basis(Vector3.BACK, -0.6), c + Vector3(0, 0, 0.06)))
	m.box(Vector3(0, 0.17, 0), Vector3(0.05, 0.34, 0.02), "eye")
	m.pop()
	m.push(Transform3D(Basis(Vector3.BACK, 1.9), c + Vector3(0, 0, 0.07)))
	m.box(Vector3(0, 0.12, 0), Vector3(0.06, 0.24, 0.02), "eye")
	m.pop()
	# calendar grid
	for i: int in 4:
		for j: int in 3:
			var p: Vector3 = Vector3(0.2 + i * 0.32, 1.95 - j * 0.36, 0.1)
			m.box(p, Vector3(0.26, 0.28, 0.02), "cloth_blue" if i == 2 and j == 1 else "flower_white")
	m.box(Vector3(0.68, 2.25, 0.1), Vector3(1.3, 0.12, 0.02), "tile_blue")
	for x: float in [-2.1, 2.1]:
		HubArt._stone_lantern(m, Vector3(x, 0, 0.5))
	return m.build()


## Reflection Pool ("Gương Phản Chiếu"): a still round pool with a stone rim and a deeper ring, a round stone mirror
## frame with pale glass standing at its back, and stepping stones.
static func reflection_pool() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var r: float = POOL_RADIUS
	HubArt._ring(m, Vector3.ZERO, r, r + 0.45, 0.35, 28, "stone")
	HubArt._disk(m, Vector3(0, 0.22, 0), r, 28, "water")
	HubArt._annulus(m, Vector3(0, 0.225, 0), r - 0.9, r - 0.5, 28, "water_deep")
	# mirror frame at the back (local -Z)
	var f: Vector3 = Vector3(0, 0, -r - 0.9)
	m.box(f + Vector3(0, 0.3, 0), Vector3(2.2, 0.6, 0.8), "stone_dark", "stone")
	m.push(Transform3D(Basis.IDENTITY, f + Vector3(0, 2.0, 0)))
	HubArt._torus_ring(m, Vector3.ZERO, 1.2, 0.16, 24, 6, "stone")
	m.pop()
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), f + Vector3(0, 2.0, 0)))
	m.cylinder(Vector3(0, -0.04, 0), 1.08, 1.08, 0.08, 20, "wind")
	m.pop()
	m.blob(f + Vector3(-0.4, 2.4, 0.06), Vector3(0.2, 0.12, 0.02), "eye_light", 0)
	for k: int in 4:
		HubArt._disk(m, Vector3(-1.2 + k * 0.8, 0.26, 0.6 - absf(k - 1.5) * 0.3), 0.32, 10, "stone")
	return m.build()


## Raked gravel garden: a low wooden border round pale gravel with rake lines, three mossy rocks.
static func gravel_garden() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9485
	var hw: float = 3.2
	var hd: float = 2.4
	m.box(Vector3(0, 0.04, 0), Vector3(hw * 2.0, 0.08, hd * 2.0), "path", "path")
	for k: int in 9:
		var z: float = -hd + 0.3 + k * 0.5
		m.box(Vector3(0, 0.09, z), Vector3(hw * 2.0 - 0.4, 0.02, 0.05), "stone")
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(0, 0.12, s * hd), Vector3(hw * 2.0 + 0.3, 0.24, 0.16), "wood")
		m.box(Vector3(s * hw, 0.12, 0), Vector3(0.16, 0.24, hd * 2.0), "wood")
	for p: Array in [[Vector3(-1.4, 0, -0.6), 0.75], [Vector3(1.2, 0, 0.5), 0.55], [Vector3(0.4, 0, -1.2), 0.4]]:
		var c: Vector3 = p[0] as Vector3
		var s2: float = float(p[1])
		m.blob(c + Vector3.UP * s2 * 0.5, Vector3(s2, s2 * 0.75, s2 * 0.85), "stone_dark", 1, 0.1, rng)
		m.blob(c + Vector3(0.1, s2 * 1.05, 0), Vector3(s2 * 0.55, s2 * 0.2, s2 * 0.5), "leaf_dark", 0)
		HubArt._annulus(m, c + Vector3.UP * 0.1, s2 + 0.15, s2 + 0.3, 14, "stone")
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9483
	for a: float in [60.0, 120.0, 240.0, 300.0]:
		var p: Vector3 = PLAZA + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (PLAZA_RADIUS - 0.6)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "wind")
	for b: Array in [[Vector3(-5.6, 0.08, 7.4), 40.0], [Vector3(5.6, 0.08, 7.4), -40.0], [Vector3(-9.0, 0.08, -4.6), 180.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	ZoneKit.signpost(m, Vector3(3.0, 0, 11.0), [[Vector3(0, 0, -20), "home", "roof"], [BOARD, "tower", "tile_blue_dark"],
		[POOL, "star", "tile_blue"], [PAVILION, "book", "cloth_blue"]])
	for z: float in [-4.0, -12.0, -17.0]:
		for s: float in [-1.0, 1.0]:
			HubArt._stone_lantern(m, Vector3(s * 2.7, 0, z))
	# bamboo fences framing the garden corners
	for f: Array in [[Vector3(-17.0, 0, -16.0), 0.0], [Vector3(17.0, 0, -16.0), 0.0], [Vector3(-19.5, 0, 3.0), 90.0], [Vector3(19.5, 0, 3.0), 90.0]]:
		m.push_at(f[0] as Vector3, float(f[1]))
		for k: int in 9:
			m.cylinder(Vector3(-2.0 + k * 0.5, 0, 0), 0.07, 0.07, 1.3 + 0.1 * (k % 2), 6, "bamboo")
		for y: float in [0.4, 1.0]:
			m.box(Vector3(0, y, 0.08), Vector3(4.3, 0.06, 0.06), "wood_dark")
		m.pop()
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, -21.5), Vector3(21.8, 0, 16.0)]:
		ZoneKit.rock(m, p, Vector3(rng.randf_range(0.7, 1.0), rng.randf_range(0.4, 0.7), rng.randf_range(0.6, 0.9)), "stone_dark", rng)
	return m.build()


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"pine": "veg_pine_a", "bamboo": "veg_bamboo_a", "bush": "veg_bush_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"pine":
			return ZoneKit.scatter_in(9490, 8, 22, 1.6, LAYOUT, BLOCKED, 0.8, 1.2)
		"bamboo":
			return ZoneKit.scatter_in(9491, 6, 10, 1.2, LAYOUT, BLOCKED, 0.8, 1.2, Vector2(25.5, 28.0))
		"bush":
			return ZoneKit.scatter_in(9493, 16, 0, 1.0, LAYOUT, BLOCKED, 0.7, 1.1)
		"tuft":
			return ZoneKit.scatter_in(9495, 200, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.3)
	return []
