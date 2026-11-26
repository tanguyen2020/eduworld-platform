extends RefCounted
## Recipes of "Vườn Bạn Bè" (Friendship Garden, `social_zone`; F094, WORLD_DESIGN §2.3 Chú Ong Kết Nối, §2.4, §4.1
## "danh sách bạn, lập party, emote, mini-game vui chơi nhẹ, góc club", §11; SOCIAL_SYSTEM; ADR-0103).
## SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## A cheerful flower garden for friends and clubs: Buzzy's beehive pavilion (a honey-yellow hive dome with a round
## door, a heart on top and flower trellises) behind the wind gate; the seed planter beside
## `collectible.social.flower_seed` (a big pot with a young sunflower, a watering can, a little fence); a heart-shaped
## flower arch over the lane; a playground (swings, seesaw); a picnic spot; the club corner of three canopies with
## pennants; heart flower beds, bunting and balloons. No text (R2); colours are art palette slots only (R32);
## collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "social"
const THEME_OVERRIDES: Dictionary = {"paving_dark": "flower_pink", "hedge": "leaf_dark", "hedge_top": "flower_pink"}
const PLAZA: Vector3 = Vector3(0, 0, 4)
const PLAZA_RADIUS: float = 6.5
const SEED_PAD: Vector3 = Vector3(8, 0, -4)
const PLANTER: Vector3 = Vector3(11.2, 0, -6.4)
const PLAYGROUND: Vector3 = Vector3(13.5, 0, 11.0)
const PICNIC: Vector3 = Vector3(-13.0, 0, -9.5)
const CLUBS: Vector3 = Vector3(-13.5, 0, 11.5)
const ARCH_Z: float = -12.0
const FESTIVE: PackedStringArray = ["cloth_red", "wall_yellow", "cloth_blue", "cloth_green", "flower_pink", "cloth_violet"]
const LAYOUT: Dictionary = {
	"plaza": [PLAZA, PLAZA_RADIUS],
	"lanes": [[Vector3(0, 0, 0.5), Vector2(2.0, 23.5)], [Vector3(-1.0, 0, -4.0), Vector2(13.0, 1.5)]],
	"pads": [[SEED_PAD, 3.0], [Vector3(0, 0, -20), 3.0]],
}
const BLOCKED: Array = [
	[PLANTER, Vector2(1.8, 1.8)],
	[PLAYGROUND, Vector2(3.6, 3.0)],
	[PICNIC, Vector2(3.4, 2.8)],
	[CLUBS, Vector2(4.6, 3.4)],
	[Vector3(0, 0, ARCH_Z), Vector2(4.0, 1.2)],
	[Vector3(3.2, 0, 11.5), Vector2(1.2, 1.2)],
	[Vector3(0, 0, -31), Vector2(12.0, 5.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_social_terrain_a": terrain, "env/env_social_far_a": far, "bld/bld_social_set_a": buildings,
		"prop/prop_social_set_a": props, "prop/prop_social_seed_planter_a": seed_planter,
		"prop/prop_social_heart_arch_a": heart_arch, "prop/prop_social_playground_a": playground,
		"prop/prop_social_picnic_a": picnic, "bld/bld_social_club_corner_a": club_corner,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.lawn(m, th)
	ZoneKit.lanes(m, LAYOUT, String(th["path"]))
	ZoneKit.plaza(m, th, PLAZA, PLAZA_RADIUS)
	ZoneKit.pad(m, th, SEED_PAD)
	ZoneKit.pad(m, th, Vector3(0, 0, -20))
	ZoneKit.glow_ring(m, Vector3(0, 0, -20), 3.0)
	ZoneKit.glow_ring(m, SEED_PAD, 3.0)
	# a heart mosaic in the middle of the plaza and soft sand under the playground and the picnic
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(1.3, 1.3, 1.0)), PLAZA + Vector3(0, 0.09, 0)))
	m.extrude(HubArt._icon_points("social_zone"), 0.01, "flower_pink")
	m.pop()
	HubArt._disk(m, PLAYGROUND + Vector3.UP * 0.05, 3.4, 20, "path")
	HubArt._disk(m, CLUBS + Vector3.UP * 0.05, 3.8, 20, "paving")
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9421
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_hive_pavilion(m)
	return m.build()


## Buzzy's beehive pavilion: a stone terrace, a honey-yellow hive dome of stacked rounded rings with a round door and
## windows, a heart on the top, two flower trellis arches with giant flowers and two beehive boxes.
static func _hive_pavilion(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 0.25, 0), Vector3(17.0, 0.5, 7.5), "stone", "stone")
	m.box(Vector3(0, 0.08, 4.1), Vector3(4.0, 0.16, 1.0), "stone")
	var rings: Array = [[3.6, 0.5], [3.9, 1.6], [3.8, 2.7], [3.3, 3.7], [2.6, 4.6], [1.7, 5.3]]
	for k: int in rings.size():
		var r: float = float((rings[k] as Array)[0])
		var y: float = float((rings[k] as Array)[1])
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, y + 0.45, 0)))
		HubArt._torus_ring(m, Vector3.ZERO, r - 0.45, 0.55, 24, 6, "wall_yellow" if k % 2 == 0 else "rice_ripe")
		m.pop()
	m.cylinder(Vector3(0, 0.5, 0), 3.3, 1.2, 5.6, 20, "rice_ripe", "rice_ripe")
	m.blob(Vector3(0, 6.1, 0), Vector3(0.9, 0.5, 0.9), "wall_yellow", 1)
	# round door and windows
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.5, 3.75)))
	m.extrude(ZoneKit.arch(0.95, 2.3, 10), 0.3, "wood_dark")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.5, 3.95)))
	m.extrude(ZoneKit.arch(0.75, 2.05, 10), 0.1, "wood")
	m.pop()
	for x: float in [-2.0, 2.0]:
		m.push(Transform3D(Basis(Vector3.UP, x * 0.18) * Basis(Vector3.RIGHT, PI * 0.5), Vector3(x, 2.9, 3.35 - absf(x) * 0.12)))
		m.cylinder(Vector3(0, -0.1, 0), 0.45, 0.45, 0.2, 12, "wood_dark")
		m.cylinder(Vector3(0, 0.0, 0), 0.36, 0.36, 0.14, 12, "wind")
		m.pop()
	# heart on the top
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.8, 0.8, 1.0)), Vector3(0, 7.3, 0)))
	m.extrude(HubArt._icon_points("social_zone"), 0.35, "flower_pink", "cloth_red")
	m.pop()
	# flower trellis arches with giant flowers
	for s: float in [-1.0, 1.0]:
		var c: Vector3 = Vector3(s * 6.6, 0.5, 1.2)
		for dx: float in [-1.0, 1.0]:
			m.box(c + Vector3(dx, 1.4, 0), Vector3(0.2, 2.8, 0.2), "wood")
		HubArt._torus_ring(m, c + Vector3(0, 2.8, 0), 1.0, 0.12, 12, 5, "leaf")
		for k: int in 5:
			var a: float = PI * float(k) / 4.0
			var p: Vector3 = c + Vector3(cos(a) * 1.0, 2.8 + sin(a) * 1.0, 0.1)
			m.blob(p, Vector3(0.22, 0.22, 0.12), "flower_pink" if k % 2 == 0 else "flower_white", 0)
		_giant_flower(m, c + Vector3(s * 2.4, 0, -0.6), 3.0, "cloth_red" if s < 0.0 else "cloth_violet")
		# beehive box
		var b: Vector3 = Vector3(s * 4.8, 0.5, 3.0)
		m.box(b + Vector3(0, 0.35, 0), Vector3(0.9, 0.7, 0.8), "wall_yellow")
		m.box(b + Vector3(0, 0.78, 0), Vector3(1.05, 0.12, 0.95), "wood_dark")
	m.pop()


## Giant garden flower: a stem with two leaves, five round petals and a yellow centre facing +Z.
static func _giant_flower(m: LowPoly, p: Vector3, h: float, colour: String) -> void:
	m.cylinder(p, 0.09, 0.07, h, 8, "leaf_dark")
	for s: float in [-1.0, 1.0]:
		m.blob(p + Vector3(s * 0.35, h * 0.4, 0), Vector3(0.35, 0.1, 0.18), "leaf", 0)
	var c: Vector3 = p + Vector3(0, h, 0.1)
	for k: int in 5:
		var a: float = TAU * float(k) / 5.0
		m.blob(c + Vector3(cos(a) * 0.42, sin(a) * 0.42, 0), Vector3(0.32, 0.32, 0.1), colour, 1)
	m.blob(c + Vector3(0, 0, 0.08), Vector3(0.26, 0.26, 0.14), "lantern_glow", 1)


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Seed planter (beside collectible.social.flower_seed): a big terracotta pot with a young sunflower, a watering can
## and a little picket fence.
static func seed_planter() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.7, 0.0), Vector2(0.95, 0.75), Vector2(1.08, 0.8), Vector2(1.08, 0.95),
		Vector2(0.98, 0.95)]), 16, "roof")
	HubArt._disk(m, Vector3(0, 0.93, 0), 0.98, 16, "soil")
	# sunflower
	m.cylinder(Vector3(0, 0.9, 0), 0.07, 0.06, 1.7, 8, "leaf_dark")
	for s: float in [-1.0, 1.0]:
		m.blob(Vector3(s * 0.3, 1.5 + s * 0.15, 0), Vector3(0.32, 0.08, 0.16), "leaf", 0)
	var c: Vector3 = Vector3(0, 2.7, 0.08)
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		m.blob(c + Vector3(cos(a) * 0.36, sin(a) * 0.36, 0), Vector3(0.22, 0.22, 0.07), "lantern_glow", 0)
	m.blob(c + Vector3(0, 0, 0.06), Vector3(0.24, 0.24, 0.1), "wood_dark", 1)
	# seedlings
	for p: Vector3 in [Vector3(-0.5, 0.9, 0.3), Vector3(0.45, 0.9, -0.35)]:
		m.blob(p + Vector3.UP * 0.12, Vector3(0.14, 0.12, 0.08), "leaf_light", 0)
	# watering can
	var w: Vector3 = Vector3(1.35, 0, 0.55)
	m.cylinder(w, 0.28, 0.26, 0.5, 12, "cloth_blue", "cloth_blue")
	HubArt._limb(m, w + Vector3(0.2, 0.15, 0), w + Vector3(0.6, 0.55, 0), 0.05, 0.04, "cloth_blue")
	m.cylinder(w + Vector3(0.6, 0.53, 0), 0.08, 0.1, 0.06, 8, "metal")
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), w + Vector3(-0.05, 0.6, 0)))
	HubArt._torus_ring(m, Vector3.ZERO, 0.2, 0.035, 10, 4, "cloth_blue")
	m.pop()
	# picket fence half ring behind the pot
	for k: int in 7:
		var a: float = PI * (0.15 + 0.7 * float(k) / 6.0)
		var p: Vector3 = Vector3(cos(a) * 1.55, 0, -sin(a) * 1.55)
		m.box(p + Vector3.UP * 0.4, Vector3(0.14, 0.8, 0.06), "flower_white")
		m.cylinder(p + Vector3.UP * 0.8, 0.08, 0.0, 0.12, 4, "flower_white", "", false)
	return m.build()


## Heart flower arch over the lane: two flowering pillars at x = ±2.7 and a big heart frame with blossoms on top.
static func heart_arch() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * 2.7, 0.25, 0), Vector3(1.0, 0.5, 1.0), "stone")
		m.cylinder(Vector3(s * 2.7, 0.5, 0), 0.28, 0.24, 2.6, 10, "wood")
		for k: int in 4:
			m.blob(Vector3(s * 2.7 + (0.15 if k % 2 == 0 else -0.15), 0.9 + k * 0.6, 0.2), Vector3(0.32, 0.28, 0.28), "leaf", 0)
			m.blob(Vector3(s * 2.7 + 0.1, 1.05 + k * 0.6, 0.38), Vector3(0.12, 0.12, 0.1), "flower_pink" if k % 2 == 0 else "flower_white", 0)
	m.box(Vector3(0, 3.15, 0), Vector3(6.2, 0.3, 0.4), "wood")
	var heart: PackedVector2Array = HubArt._icon_points("social_zone")
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(1.4, 1.4, 1.0)), Vector3(0, 4.55, 0)))
	m.extrude(heart, 0.3, "flower_pink", "cloth_red")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.9, 0.9, 1.0)), Vector3(0, 4.6, 0.12)))
	m.extrude(heart, 0.12, "flower_white")
	m.pop()
	return m.build()


## Playground: an A-frame swing set with two swings and a seesaw (front along +Z).
static func playground() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	for x: float in [-1.8, 1.8]:
		for z: float in [-0.7, 0.7]:
			HubArt._limb(m, Vector3(x, 0, z * 1.3), Vector3(x, 2.7, 0), 0.07, 0.06, "cloth_red")
	m.push(Transform3D(Basis(Vector3.BACK, PI * 0.5), Vector3(-1.9, 2.7, 0)))
	m.cylinder(Vector3.ZERO, 0.08, 0.08, 3.8, 8, "cloth_blue")
	m.pop()
	for x: float in [-0.8, 0.8]:
		for dx: float in [-0.25, 0.25]:
			m.cylinder(Vector3(x + dx, 0.6, 0), 0.015, 0.015, 2.1, 4, "metal_dark")
		m.box(Vector3(x, 0.6, 0), Vector3(0.7, 0.08, 0.32), "wall_yellow" if x < 0.0 else "cloth_green")
	# seesaw
	var s: Vector3 = Vector3(0, 0, 2.6)
	m.cylinder(s, 0.3, 0.2, 0.45, 10, "cloth_violet")
	m.push(Transform3D(Basis(Vector3.BACK, 0.18), s + Vector3(0, 0.5, 0)))
	m.box(Vector3.ZERO, Vector3(3.4, 0.1, 0.36), "wood")
	for x: float in [-1.5, 1.5]:
		m.box(Vector3(x, 0.18, 0), Vector3(0.36, 0.26, 0.36), "cloth_blue" if x < 0.0 else "cloth_red")
		HubArt._limb(m, Vector3(x * 0.85, 0.05, 0), Vector3(x * 0.85, 0.45, 0), 0.03, 0.03, "metal_dark")
	m.pop()
	return m.build()


## Picnic spot: two checked blankets, a basket with a cloth, a round low table and cushions.
static func picnic() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.push_at(Vector3.ZERO, 0.0, Vector3(1.45, 1.45, 1.45))
	for b: Array in [[Vector3(-1.1, 0.03, 0), 12.0, "cloth_red"], [Vector3(1.4, 0.03, 0.3), -18.0, "cloth_blue"]]:
		m.push_at(b[0] as Vector3, float(b[1]))
		m.box(Vector3.ZERO, Vector3(2.2, 0.04, 2.0), "flower_white")
		for i: int in 3:
			for j: int in 3:
				if (i + j) % 2 == 0:
					m.box(Vector3(-0.73 + i * 0.73, 0.025, -0.66 + j * 0.66), Vector3(0.72, 0.03, 0.65), String(b[2]))
		m.pop()
	# basket
	var k: Vector3 = Vector3(-1.2, 0.05, 0.2)
	m.box(k + Vector3(0, 0.22, 0), Vector3(0.7, 0.44, 0.5), "basket")
	m.box(k + Vector3(0, 0.46, 0), Vector3(0.74, 0.06, 0.54), "cloth_red")
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), k + Vector3(0, 0.46, 0)))
	HubArt._torus_ring(m, Vector3.ZERO, 0.3, 0.03, 10, 4, "basket")
	m.pop()
	# low table with fruit and cups
	var t: Vector3 = Vector3(1.3, 0.05, 0.4)
	m.cylinder(t, 0.12, 0.1, 0.3, 8, "wood_dark")
	m.cylinder(t + Vector3.UP * 0.3, 0.55, 0.55, 0.06, 14, "wood")
	for p: Array in [[Vector3(-0.2, 0.42, 0.1), "cloth_red"], [Vector3(0.15, 0.42, -0.15), "lantern_glow"], [Vector3(0.2, 0.42, 0.2), "leaf_light"]]:
		m.blob(t + (p[0] as Vector3), Vector3(0.09, 0.09, 0.09), String(p[1]), 0)
	for k2: int in 3:
		var a: float = TAU * float(k2) / 3.0 + 0.5
		m.blob(Vector3(cos(a) * 1.6, 0.18, sin(a) * 1.3), Vector3(0.36, 0.14, 0.36), ["wall_yellow", "cloth_green", "flower_pink"][k2], 1)
	m.pop()
	return m.build()


## Club corner (góc club): three canopy tents in club colours with pennants, a table under each and a shared notice
## frame without text.
static func club_corner() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var colours: Array = [["cloth_blue", "flower_white"], ["cloth_green", "wall_yellow"], ["cloth_violet", "flower_pink"]]
	for i: int in 3:
		var c: Vector3 = Vector3(-3.0 + i * 3.0, 0, -0.8 + absf(i - 1) * 0.8)
		var col: Array = colours[i] as Array
		for x: float in [-1.1, 1.1]:
			for z: float in [-1.0, 1.0]:
				m.cylinder(c + Vector3(x, 0, z), 0.05, 0.05, 2.2, 6, "wood_dark")
		# canopy: closed pyramid with striped valance
		var top: Vector3 = c + Vector3(0, 3.0, 0)
		var corners: Array[Vector3] = [c + Vector3(-1.3, 2.2, -1.2), c + Vector3(1.3, 2.2, -1.2), c + Vector3(1.3, 2.2, 1.2), c + Vector3(-1.3, 2.2, 1.2)]
		for k: int in 4:
			m.tri_out(corners[k], corners[(k + 1) % 4], top, String(col[k % 2]), c + Vector3(0, 2.4, 0))
		m.quad_out(corners[0], corners[1], corners[2], corners[3], String(col[0]), c + Vector3(0, 2.6, 0))
		for k: int in 4:
			var a: Vector3 = corners[k]
			var b: Vector3 = corners[(k + 1) % 4]
			m.box((a + b) * 0.5 + Vector3.DOWN * 0.12, Vector3(absf(b.x - a.x) + 0.06, 0.24, absf(b.z - a.z) + 0.06), String(col[1]))
		m.cylinder(top, 0.03, 0.03, 0.6, 5, "wood_dark")
		m.push_at(top + Vector3(0, 0.55, 0), 0.0)
		m.extrude(PackedVector2Array([Vector2(0.02, 0), Vector2(0.6, -0.12), Vector2(0.02, -0.26)]), 0.03, String(col[0]))
		m.pop()
		m.box(c + Vector3(0, 0.75, 0), Vector3(1.6, 0.08, 0.8), "wood")
		for x: float in [-0.65, 0.65]:
			m.box(c + Vector3(x, 0.37, 0), Vector3(0.08, 0.74, 0.7), "wood_dark")
		m.blob(c + Vector3(0.3, 0.88, 0.1), Vector3(0.14, 0.1, 0.14), String(col[1]), 0)
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9423
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = PLAZA + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "flower_pink")
	for b: Array in [[Vector3(-4.6, 0.08, 9.0), 40.0], [Vector3(4.6, 0.08, 9.0), -40.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	# bunting over the lane and round the plaza
	for z: float in [-7.5, -16.5]:
		for s: float in [-1.0, 1.0]:
			m.cylinder(Vector3(s * 3.0, 0, z), 0.08, 0.07, 3.3, 8, "wood")
		ZoneKit.bunting(m, Vector3(-3.0, 3.25, z), Vector3(3.0, 3.25, z), FESTIVE, 7, 0.45)
	# heart flower beds by the plaza
	for c: Vector3 in [Vector3(-8.5, 0, 6.0), Vector3(8.5, 0, 6.0)]:
		_heart_bed(m, c, rng)
	ZoneKit.balloons(m, Vector3(-3.6, 0, -8.8), PackedStringArray(["cloth_red", "wall_yellow", "cloth_blue"]), rng)
	ZoneKit.balloons(m, Vector3(3.6, 0, -15.0), PackedStringArray(["flower_pink", "cloth_green", "cloth_violet"]), rng)
	ZoneKit.signpost(m, Vector3(3.2, 0, 11.5), [[Vector3(0, 0, -20), "home", "roof"], [SEED_PAD, "star", "wall_yellow"],
		[CLUBS, "talk", "cloth_violet"], [PLAYGROUND, "heart", "flower_pink"]])
	for p: Vector3 in [Vector3(-3.6, 0, -21.8), Vector3(3.6, 0, -21.8)]:
		HubArt._stone_lantern(m, p)
	# butterflies resting on the flower bushes
	for p: Array in [[Vector3(-6.2, 1.2, -1.0), "cloth_violet"], [Vector3(5.2, 1.4, -10.5), "wall_yellow"], [Vector3(-9.5, 1.1, 2.4), "cloth_blue"]]:
		_butterfly(m, p[0] as Vector3, String(p[1]))
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, -21.5), Vector3(21.8, 0, 2.0)]:
		ZoneKit.rock(m, p, Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)), "stone_dark", rng)
	return m.build()


## Heart-shaped flower bed: stone rim, soil and round blossoms.
static func _heart_bed(m: LowPoly, c: Vector3, rng: RandomNumberGenerator) -> void:
	var heart: PackedVector2Array = HubArt._icon_points("social_zone")
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(1.6, 1.6, 1.0)), c + Vector3.UP * 0.15))
	m.extrude(heart, 0.3, "stone")
	m.pop()
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(1.35, 1.35, 1.0)), c + Vector3.UP * 0.2))
	m.extrude(heart, 0.3, "soil")
	m.pop()
	for i: int in 7:
		var p: Vector3 = c + Vector3(rng.randf_range(-0.9, 0.9), 0.45, rng.randf_range(-0.9, 0.5))
		m.blob(p, Vector3(0.28, 0.2, 0.28), "leaf", 0)
		m.blob(p + Vector3(0, 0.17, 0), Vector3(0.14, 0.1, 0.14), ["flower_pink", "cloth_red", "flower_white"][i % 3], 0)


## Butterfly: two pairs of closed wings around a little body.
static func _butterfly(m: LowPoly, p: Vector3, colour: String) -> void:
	m.blob(p, Vector3(0.03, 0.03, 0.12), "wood_dark", 0)
	for s: float in [-1.0, 1.0]:
		m.push(Transform3D(Basis(Vector3.BACK, s * 0.5), p + Vector3(s * 0.02, 0.02, 0)))
		m.extrude(PackedVector2Array([Vector2(0, 0.0), Vector2(s * 0.22, 0.16), Vector2(s * 0.28, -0.02), Vector2(s * 0.16, -0.16)]),
				0.02, colour)
		m.pop()


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"tree": "veg_tree_round_a", "blossom": "veg_tree_blossom_a", "bush": "veg_bush_a", "flower": "veg_flower_a",
	"tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"tree":
			return ZoneKit.scatter_in(9430, 6, 20, 1.6, LAYOUT, BLOCKED, 0.85, 1.25)
		"blossom":
			return ZoneKit.scatter_in(9431, 6, 6, 1.6, LAYOUT, BLOCKED, 0.85, 1.2)
		"bush":
			return ZoneKit.scatter_in(9433, 34, 0, 1.0, LAYOUT, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter_in(9434, 150, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter_in(9435, 220, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.4)
	return []
