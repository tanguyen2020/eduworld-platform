extends RefCounted
## Recipes of "Rừng Ngôn Ngữ — khu Tiếng Anh" (Language Forest, `learning_zone.english`; F093, WORLD_DESIGN §2.3
## Robo Ben, §4.2, §11; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## Set pieces: the clock tower behind the wind gate (a nod to Robo Ben; ticks and hands only), Robo Ben's round robot
## workshop with a big gear, a porthole and a smoking chimney (discovery.english.robot_workshop), the Echo Cave in a
## mossy rock mound with violet crystals and sound rings (discovery.english.echo_cave), the Gear Arch over the lane
## (collectible.english.gear_word), a pillar post box for letters, speech-bubble signs (shapes, no text, R2), little
## robot statues, violet lanterns and bằng lăng trees in bloom. Colours are art palette slots only (R32).

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "english"
const THEME_OVERRIDES: Dictionary = {"paving_dark": "cloth_violet", "hedge": "leaf_dark", "hedge_top": "blossom_violet"}
const WORKSHOP_CENTER: Vector3 = Vector3(16.8, 0, -6)
const CAVE_CENTER: Vector3 = Vector3(-17.2, 0, -6)
const GEAR_POST_X: float = 2.9
const BLOCKED: Array = [
	[WORKSHOP_CENTER, Vector2(3.4, 3.4)],
	[CAVE_CENTER, Vector2(3.8, 3.8)],
	[Vector3(-18.0, 0, 16.0), Vector2(3.2, 3.2)],
	[Vector3(0, 0, -31), Vector2(10.0, 4.0)],
]
const HOUSE: Array = [Vector3(-18.0, 0, 16.0), 90.0, Vector2(5.0, 4.0)]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_english_terrain_a": terrain, "env/env_english_far_a": far, "bld/bld_english_set_a": buildings, "prop/prop_english_set_a": props,
		"bld/bld_english_workshop_a": robot_workshop, "prop/prop_english_cave_a": echo_cave, "prop/prop_english_gear_arch_a": gear_arch,
		"veg/veg_tree_blossom_a": blossom_tree,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9341
	var th: Dictionary = theme()
	ZoneKit.terrain(m, th)
	ZoneKit.path_strip(m, Vector3(15.0, 0, -7.2), Vector3(17.0, 0, -4.8), "path")
	ZoneKit.path_strip(m, Vector3(-17.0, 0, -7.2), Vector3(-15.0, 0, -4.8), "path")
	# stepping stones to the house and violet petals under the bằng lăng
	for i: int in 6:
		HubArt._disk(m, Vector3(-4.0 - i * 2.3, 0.05, 12.0 + i * 0.8), 0.55, 8, "stone")
	for i: int in 18:
		var p: Vector3 = Vector3(rng.randf_range(-22.0, 22.0), 0.03, rng.randf_range(-22.0, 22.0))
		if ZoneKit.clear(p, 0.8, BLOCKED):
			HubArt._disk(m, p, rng.randf_range(0.5, 0.9), 7, "blossom_violet")
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9349
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_clock_tower(m)
	m.push_at(HOUSE[0] as Vector3, float(HOUSE[1]))
	ZoneKit.house(m, HOUSE[2] as Vector2, "subject_english", "cloth_violet", "blossom_violet", "lantern_glow")
	m.pop()
	return m.build()


## Clock tower: stone base, a tall cream tower with violet bands, four clock faces (ticks and two hands), a belfry
## and a pointed violet spire, between two little cottages with violet roofs.
static func _clock_tower(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 0.3, 0), Vector3(14.0, 0.6, 5.6), "stone", "stone")
	m.box(Vector3(0, 1.3, 0), Vector3(3.8, 1.4, 3.8), "stone_dark")
	m.box(Vector3(0, 5.3, 0), Vector3(3.2, 6.8, 3.2), "wall")
	for y: float in [2.1, 5.0, 8.7]:
		m.box(Vector3(0, y, 0), Vector3(3.4, 0.3, 3.4), "cloth_violet")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 2.0, 1.62)))
	m.extrude(ZoneKit.arch(0.6, 1.9, 8), 0.1, "wood_dark")
	m.pop()
	for side: int in 4:
		m.push_at(Vector3.ZERO, 90.0 * side)
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 7.2, 1.62)))
		m.extrude(ZoneKit.circle(1.05, 20), 0.1, "subject_english")
		m.pop()
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 7.2, 1.68)))
		m.extrude(ZoneKit.circle(0.9, 20), 0.08, "flower_white")
		m.pop()
		for k: int in 12:
			var a: float = TAU * float(k) / 12.0
			m.push(Transform3D(Basis(Vector3.BACK, a), Vector3(0, 7.2, 1.74)))
			m.box(Vector3(0.72, 0, 0), Vector3(0.18 if k % 3 == 0 else 0.1, 0.05, 0.03), "eye")
			m.pop()
		m.push(Transform3D(Basis(Vector3.BACK, 0.5), Vector3(0, 7.2, 1.77)))
		m.box(Vector3(0, 0.25, 0), Vector3(0.07, 0.5, 0.03), "eye")
		m.pop()
		m.push(Transform3D(Basis(Vector3.BACK, -1.9), Vector3(0, 7.2, 1.79)))
		m.box(Vector3(0, 0.33, 0), Vector3(0.05, 0.66, 0.03), "eye")
		m.pop()
		m.pop()
	# belfry with four posts and a bell, then the spire
	for x: float in [-1.3, 1.3]:
		for z: float in [-1.3, 1.3]:
			m.box(Vector3(x, 9.6, z), Vector3(0.4, 1.5, 0.4), "wall")
	m.lathe(Vector3(0, 9.0, 0), PackedVector2Array([Vector2(0.55, 0.0), Vector2(0.5, 0.3), Vector2(0.32, 0.8), Vector2(0.05, 1.0)]), 12,
			"bronze")
	m.box(Vector3(0, 10.45, 0), Vector3(3.6, 0.3, 3.6), "cloth_violet")
	m.cylinder(Vector3(0, 10.6, 0), 2.45, 0.0, 4.0, 4, "subject_english", "", false, 0.5)
	m.cylinder(Vector3(0, 14.5, 0), 0.05, 0.05, 1.0, 6, "metal_dark")
	m.blob(Vector3(0, 15.6, 0), Vector3(0.25, 0.25, 0.25), "lantern_glow", 1)
	for s: float in [-1.0, 1.0]:
		m.push_at(Vector3(s * 4.6, 0.6, 0.4), 0.0)
		m.box(Vector3(0, 1.3, 0), Vector3(3.6, 2.6, 3.2), "wall")
		m.box(Vector3(0, 0.3, 0), Vector3(3.7, 0.5, 3.3), "cloth_violet")
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.0, 1.62)))
		m.extrude(ZoneKit.arch(0.45, 1.6, 8), 0.08, "wood_dark")
		m.pop()
		for x: float in [-1.1, 1.1]:
			m.push(Transform3D(Basis.IDENTITY, Vector3(x, 1.6, 1.62)))
			m.extrude(ZoneKit.circle(0.32, 10), 0.08, "lantern_glow", "wood_dark")
			m.pop()
		m.push_at(Vector3(0, 2.6, 0), 90.0)
		m.tiled_roof(Vector3.ZERO, 3.8, 4.0, 1.7, 0.4, 0.15, "subject_english", "cloth_violet")
		m.pop()
		m.pop()
	m.pop()


## Robo Ben's robot workshop (discovery.english.robot_workshop in front of its door): a round cream workshop with
## steel bands, a violet dome, a big bronze gear on the wall, a porthole, a chimney with smoke puffs and an antenna;
## a workbench with crates outside. Front along local +Z.
static func robot_workshop() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.cylinder(Vector3.ZERO, 2.6, 2.6, 0.25, 20, "stone", "stone")
	m.cylinder(Vector3(0, 0.25, 0), 2.2, 2.15, 2.6, 20, "wall")
	for y: float in [0.35, 2.55]:
		m.cylinder(Vector3(0, y, 0), 2.24, 2.24, 0.22, 20, "metal", "", false)
	m.blob(Vector3(0, 2.75, 0), Vector3(2.3, 1.7, 2.3), "subject_english", 2)
	m.cylinder(Vector3(0, 2.7, 0), 2.35, 2.35, 0.2, 20, "cloth_violet", "", false)
	# door, porthole, gear
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.25, 2.2)))
	m.extrude(ZoneKit.arch(0.55, 1.75, 8), 0.12, "wood_dark", "metal_dark")
	m.pop()
	m.push(Transform3D(Basis(Vector3.UP, -0.75), Vector3(0, 0, 0)))
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 1.55, 2.21)))
	m.extrude(ZoneKit.circle(0.45, 14), 0.1, "metal")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 1.55, 2.28)))
	m.extrude(ZoneKit.circle(0.32, 14), 0.06, "wind")
	m.pop()
	m.pop()
	m.push(Transform3D(Basis(Vector3.UP, 0.8), Vector3(0, 0, 0)))
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 1.45, 2.24)))
	m.extrude(gear_points(0.85, 0.66, 10), 0.16, "bronze")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 1.45, 2.36)))
	m.extrude(ZoneKit.circle(0.28, 12), 0.08, "metal_dark")
	m.pop()
	m.pop()
	# chimney, smoke, antenna
	m.cylinder(Vector3(-1.1, 3.0, -0.6), 0.28, 0.26, 2.2, 10, "metal_dark")
	m.cylinder(Vector3(-1.1, 5.15, -0.6), 0.36, 0.36, 0.2, 10, "metal")
	for k: int in 3:
		m.blob(Vector3(-1.1 + k * 0.25, 5.75 + k * 0.5, -0.6 - k * 0.2), Vector3(0.32, 0.26, 0.32) * (1.0 + k * 0.25), "flower_white", 1)
	m.cylinder(Vector3(0.8, 4.2, 0.3), 0.04, 0.04, 1.1, 6, "metal_dark")
	m.blob(Vector3(0.8, 5.35, 0.3), Vector3(0.14, 0.14, 0.14), "cloth_red", 1)
	# workbench and crates
	m.push_at(Vector3(1.9, 0, 2.2), -35.0)
	m.box(Vector3(0, 0.75, 0), Vector3(1.4, 0.12, 0.6), "wood")
	for x: float in [-0.6, 0.6]:
		m.box(Vector3(x, 0.35, 0), Vector3(0.1, 0.7, 0.5), "wood_dark")
	m.push(Transform3D(Basis.IDENTITY, Vector3(-0.3, 0.88, 0)))
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5), Vector3.ZERO))
	m.extrude(gear_points(0.16, 0.12, 8), 0.06, "metal")
	m.pop()
	m.pop()
	m.box(Vector3(0.3, 0.88, 0.05), Vector3(0.3, 0.14, 0.2), "metal_dark")
	m.pop()
	for c: Vector3 in [Vector3(-2.2, 0.3, 1.6), Vector3(-2.5, 0.3, 0.9), Vector3(-2.3, 0.85, 1.3)]:
		m.box(c, Vector3(0.6, 0.55, 0.55), "wood", "wood_dark")
	return m.build()


## Echo Cave: a mossy rock mound with a dark arched mouth, violet crystal clusters and pale sound rings. Mouth along
## local +Z.
static func echo_cave() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9342
	m.blob(Vector3(0, 0.6, -0.4), Vector3(3.3, 2.7, 2.9), "stone", 1, 0.06, rng)
	m.blob(Vector3(-2.0, 0.4, 0.6), Vector3(1.6, 1.4, 1.5), "stone_dark", 1, 0.08, rng)
	m.blob(Vector3(2.1, 0.3, 0.7), Vector3(1.4, 1.1, 1.3), "stone_dark", 1, 0.08, rng)
	m.blob(Vector3(0.2, 3.1, -0.5), Vector3(2.4, 0.7, 2.1), "leaf", 1, 0.05, rng)
	m.blob(Vector3(-1.2, 2.7, 0.5), Vector3(1.1, 0.45, 1.0), "leaf_light", 0, 0.05, rng)
	# mouth: dark arch with a stone lip
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, -0.02, 2.35)))
	m.extrude(ZoneKit.arch(1.05, 2.1, 10), 0.3, "stone_dark")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, -0.02, 2.5)))
	m.extrude(ZoneKit.arch(0.85, 1.85, 10), 0.08, "eye")
	m.pop()
	m.blob_detail = 0
	var crystals: Array = [[Vector3(-1.6, 0, 2.3), 0.9, -0.3], [Vector3(-1.3, 0, 2.7), 0.6, 0.2], [Vector3(1.5, 0, 2.4), 1.0, 0.25],
		[Vector3(1.85, 0, 2.0), 0.65, -0.2], [Vector3(0.9, 2.6, 1.5), 0.55, 0.4], [Vector3(-2.6, 1.2, 1.2), 0.6, -0.5]]
	var colours: PackedStringArray = ["subject_english", "blossom_violet", "wind"]
	for i: int in crystals.size():
		var c: Array = crystals[i]
		m.push(Transform3D(Basis(Vector3.BACK, float(c[2])), c[0] as Vector3))
		var h: float = float(c[1])
		m.cylinder(Vector3.ZERO, 0.18 * h + 0.06, 0.2 * h + 0.06, h, 6, colours[i % colours.size()], "", false)
		m.cylinder(Vector3(0, h, 0), 0.2 * h + 0.06, 0.0, 0.45 * h, 6, colours[i % colours.size()])
		m.pop()
	# sound rings floating out of the mouth (thin arcs)
	for k: int in 3:
		var r: float = 0.5 + k * 0.35
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 1.2, 2.9 + k * 0.45)))
		for j: int in 6:
			var a0: float = -PI * 0.35 + PI * 0.7 * float(j) / 6.0
			var a1: float = -PI * 0.35 + PI * 0.7 * float(j + 1) / 6.0
			HubArt._limb(m, Vector3(sin(a0) * r, cos(a0) * r * 0.8, 0), Vector3(sin(a1) * r, cos(a1) * r * 0.8, 0), 0.04, 0.04, "wind")
		m.pop()
	return m.build()


## Gear Arch over the lane (collectible.english.gear_word under it): two steel posts, a beam and two meshing gears.
static func gear_arch() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var px: float = GEAR_POST_X
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * px, 0.2, 0), Vector3(0.9, 0.4, 0.9), "stone", "stone")
		m.cylinder(Vector3(s * px, 0.4, 0), 0.28, 0.24, 3.2, 12, "metal")
		m.cylinder(Vector3(s * px, 1.2, 0), 0.31, 0.31, 0.16, 12, "cloth_violet", "", false)
		m.blob(Vector3(s * px, 3.85, 0), Vector3(0.32, 0.32, 0.32), "lantern_glow", 1)
	m.box(Vector3(0, 3.45, 0), Vector3(2.0 * px + 0.5, 0.36, 0.5), "metal_dark")
	m.push(Transform3D(Basis(Vector3.BACK, 0.15), Vector3(-0.5, 4.75, 0)))
	m.extrude(gear_points(1.2, 0.98, 12), 0.25, "subject_english", "cloth_violet")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(-0.5, 4.75, 0.16)))
	m.extrude(ZoneKit.circle(0.42, 14), 0.1, "lantern_glow", "bronze")
	m.pop()
	m.push(Transform3D(Basis(Vector3.BACK, -0.1), Vector3(1.35, 4.3, -0.05)))
	m.extrude(gear_points(0.68, 0.52, 8), 0.22, "bronze", "metal_dark")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(1.35, 4.3, 0.1)))
	m.extrude(ZoneKit.circle(0.22, 12), 0.08, "metal")
	m.pop()
	return m.build()


## Gear outline: `teeth` teeth between the outer radius `r1` and the root radius `r0`.
static func gear_points(r1: float, r0: float, teeth: int) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in teeth:
		var a: float = TAU * float(i) / float(teeth)
		var w: float = TAU / float(teeth)
		pts.append(Vector2(cos(a), sin(a)) * r0)
		pts.append(Vector2(cos(a + w * 0.18), sin(a + w * 0.18)) * r1)
		pts.append(Vector2(cos(a + w * 0.5), sin(a + w * 0.5)) * r1)
		pts.append(Vector2(cos(a + w * 0.68), sin(a + w * 0.68)) * r0)
	return pts


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9343
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = ZoneKit.PLAZA_CENTER + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (ZoneKit.PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "metal_dark", "subject_english")
	for z: float in [-10.0, -21.5]:
		for s: float in [-1.0, 1.0]:
			ZoneKit.lamp(m, Vector3(s * 3.1, 0, z), "metal_dark", "subject_english")
	for b: Array in [[Vector3(-4.6, 0.08, 6.6), 50.0], [Vector3(4.6, 0.08, 6.6), -50.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	_post_box(m, Vector3(-5.4, 0, -1.6))
	for s: Array in [[Vector3(3.6, 0, 11.0), 0], [Vector3(-3.6, 0, -8.6), 1], [Vector3(3.6, 0, -18.0), 2]]:
		_bubble_sign(m, s[0] as Vector3, int(s[1]))
	for r: Array in [[Vector3(13.4, 0, -2.2), 1.0], [Vector3(14.2, 0, -10.0), 0.8], [Vector3(5.8, 0, -2.2), 0.9]]:
		_robot(m, r[0] as Vector3, float(r[1]), ZoneKit.yaw_to(r[0] as Vector3, ZoneKit.SPAWN))
	ZoneKit.flower_bed(m, Vector3(-7.5, 0, -12.0), 1.4, PackedStringArray(["blossom_violet", "lantern_glow", "flower_white"]), rng)
	ZoneKit.flower_bed(m, Vector3(7.5, 0, -12.0), 1.4, PackedStringArray(["subject_english", "flower_pink", "flower_white"]), rng)
	ZoneKit.flower_bed(m, Vector3(8.0, 0, 14.0), 1.6, PackedStringArray(["blossom_violet", "flower_white", "flower_pink"]), rng)
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, 21.5), Vector3(21.8, 0, -21.6), Vector3(-9.0, 0, -21.8)]:
		m.blob(p + Vector3(0, 0.2, 0), Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)),
				"stone_dark", 0, 0.15, rng)
	return m.build()


## Pillar post box for letters: red cylinder, dome cap, a dark slot.
static func _post_box(m: LowPoly, p: Vector3) -> void:
	m.cylinder(p, 0.42, 0.42, 0.15, 12, "stone")
	m.cylinder(p + Vector3.UP * 0.15, 0.34, 0.34, 1.15, 12, "cloth_red")
	m.blob(p + Vector3.UP * 1.3, Vector3(0.38, 0.22, 0.38), "cloth_red", 1)
	m.box(p + Vector3(0, 1.05, 0.33), Vector3(0.34, 0.06, 0.04), "eye")
	m.cylinder(p + Vector3.UP * 1.27, 0.37, 0.37, 0.06, 12, "lantern_glow", "", false)


## Speech-bubble sign (shape only, no text): 0 round bubble, 1 cloud bubble, 2 star — on a post.
static func _bubble_sign(m: LowPoly, p: Vector3, kind: int) -> void:
	m.cylinder(p, 0.07, 0.06, 2.0, 8, "wood_dark")
	m.push_at(p + Vector3.UP * 2.25, ZoneKit.yaw_to(p, ZoneKit.SPAWN))
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.6, 0.6, 1.0)), Vector3.ZERO))
	var shape: PackedVector2Array = HubArt._bubble() if kind == 0 else (HubArt._star(1.0, 0.5) if kind == 2 else ZoneKit.circle(1.0, 12))
	m.extrude(shape, 0.12, "flower_white", "cloth_violet")
	m.pop()
	for k: int in 3:
		m.blob(Vector3(-0.25 + k * 0.25, 0.05, 0.08), Vector3(0.07, 0.07, 0.04), "subject_english", 0)
	m.pop()


## Little robot statue: rounded box body, a head with two glowing eyes and an antenna.
static func _robot(m: LowPoly, p: Vector3, s: float, yaw: float) -> void:
	m.push_at(p, yaw, Vector3(s, s, s))
	m.box(Vector3(0, 0.08, 0), Vector3(0.7, 0.16, 0.5), "metal_dark")
	m.box(Vector3(0, 0.55, 0), Vector3(0.62, 0.7, 0.45), "metal")
	m.box(Vector3(0, 1.15, 0), Vector3(0.7, 0.5, 0.5), "metal")
	for x: float in [-0.15, 0.15]:
		m.blob(Vector3(x, 1.18, 0.26), Vector3(0.08, 0.08, 0.04), "spark", 0)
	m.box(Vector3(0, 0.6, 0.23), Vector3(0.3, 0.2, 0.04), "subject_english")
	m.cylinder(Vector3(0, 1.4, 0), 0.025, 0.025, 0.3, 4, "metal_dark")
	m.blob(Vector3(0, 1.75, 0), Vector3(0.07, 0.07, 0.07), "cloth_red", 0)
	for x: float in [-0.42, 0.42]:
		m.box(Vector3(x, 0.6, 0), Vector3(0.14, 0.45, 0.14), "metal_dark")
	m.pop()


# ---------- vegetation ----------

## Bằng lăng tree in bloom (Lagerstroemia, lilac blossom over green leaves): repeated, base detail, ≤ 2k.
static func blossom_tree() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9344
	m.cylinder(Vector3.ZERO, 0.2, 0.15, 2.2, 6, "trunk")
	m.blob(Vector3(0, 2.8, 0), Vector3(1.45, 1.15, 1.45), "leaf", 1, 0.03, rng)
	m.blob(Vector3(0.3, 3.35, 0.2), Vector3(1.05, 0.75, 1.05), "blossom_violet", 1, 0.03, rng)
	m.blob(Vector3(-0.7, 2.75, 0.5), Vector3(0.7, 0.6, 0.7), "cloth_violet", 0, 0.03, rng)
	m.blob(Vector3(0.7, 2.6, -0.5), Vector3(0.65, 0.55, 0.65), "blossom_violet", 0, 0.03, rng)
	return m.build()


const SCATTER: Dictionary = {
	"blossom": "veg_tree_blossom_a", "tree": "veg_tree_round_a", "bush": "veg_bush_a", "flower": "veg_flower_a",
	"tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"blossom":
			return ZoneKit.scatter(9350, 8, 16, 1.6, BLOCKED, 0.85, 1.25)
		"tree":
			return ZoneKit.scatter(9351, 2, 10, 1.6, BLOCKED, 0.85, 1.2)
		"bush":
			return ZoneKit.scatter(9353, 32, 0, 1.0, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter(9354, 100, 0, 0.3, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter(9355, 220, 0, 0.3, BLOCKED, 0.8, 1.4)
	return []
