extends RefCounted
## Recipes of "Đại dương Khám Phá" (Discovery Ocean, `quest_zone.discovery_ocean`; F094, WORLD_DESIGN §2.2 chapter 5
## "hành trình liên môn, bí mật, vật sưu tầm hiếm" — boss Bạch Tuộc Câu Đố, §4.3 "đảo nhỏ, hang động, xác tàu cổ", §11
## thuyền thúng; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## A sandy island of adventure in the sea: a striped lighthouse on a rocky islet behind the wind gate; the old wooden
## shipwreck tilted in the sand with a treasure chest and barrels (no cannons, no weapons) beside
## `discovery.ocean.shipwreck`; the coral cave of rocks, corals, anemones and pearls beside `discovery.ocean.coral_cave`;
## the shell arch of coral columns crowned by a giant scallop over the lane at `collectible.ocean.rare_shell`; the
## friendly Riddle Octopus statue; boardwalks, basket boats (thuyền thúng), buoys, starfish and coconut palms. No text
## (R2); colours are art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "ocean"
const THEME_OVERRIDES: Dictionary = {"lawn": "path", "lawn_alt": "grass", "path": "wood", "paving_dark": "water_deep",
	"hedge": "stone_dark", "hedge_top": "stone", "far": "water"}
const PLAZA: Vector3 = Vector3(0, 0, 4.5)
const PLAZA_RADIUS: float = 6.0
const WRECK: Vector3 = Vector3(18.0, 0, -9.0)
const CAVE: Vector3 = Vector3(-18.0, 0, -9.0)
const SHELL: Vector3 = Vector3(0, 0, -16)
const OCTOPUS: Vector3 = Vector3(-13.0, 0, 13.0)
const HUT: Array = [Vector3(17.5, 0, 15.5), -90.0, Vector2(5.0, 4.0)]
const ISLAND_RADIUS: float = 48.0
const LAYOUT: Dictionary = {
	"plaza": [PLAZA, PLAZA_RADIUS],
	"lanes": [[Vector3(0, 0, 0.5), Vector2(1.8, 23.5)], [Vector3(0, 0, -8.0), Vector2(11.0, 1.5)]],
	"pads": [[Vector3(14, 0, -8), 3.0], [Vector3(-14, 0, -8), 3.0], [SHELL, 3.0], [Vector3(0, 0, -20), 3.0]],
}
const BLOCKED: Array = [
	[WRECK, Vector2(2.8, 5.4)],
	[CAVE, Vector2(3.4, 3.6)],
	[OCTOPUS, Vector2(2.4, 2.4)],
	[Vector3(17.5, 0, 15.5), Vector2(3.2, 3.2)],
	[Vector3(3.0, 0, 11.5), Vector2(1.2, 1.2)],
	[Vector3(0, 0, -31), Vector2(12.0, 6.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_ocean_terrain_a": terrain, "env/env_ocean_far_a": far, "bld/bld_ocean_set_a": buildings,
		"prop/prop_ocean_set_a": props, "prop/prop_ocean_shipwreck_a": shipwreck, "prop/prop_ocean_coral_cave_a": coral_cave,
		"prop/prop_ocean_shell_arch_a": shell_arch, "prop/prop_ocean_octopus_a": octopus_rock,
	}


# ---------- terrain ----------

## Sand island: two-tone sand, wooden boardwalks with planks, the plaza, anchor pads with glowing rings, the beach down to
## the sea and a low rocky sea wall (boundary).
static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.lawn(m, th)
	ZoneKit.lanes(m, LAYOUT, "wood")
	# planks of the boardwalks
	var z: float = -23.0
	while z < 23.5:
		if absf(z - PLAZA.z) > PLAZA_RADIUS + 0.3:
			ZoneKit.path_strip(m, Vector3(-1.8, 0, z), Vector3(1.8, 0, z + 0.07), "wood_dark", 0.05)
		z += 0.9
	var x: float = -11.0
	while x < 11.0:
		if absf(x) > 1.9:
			ZoneKit.path_strip(m, Vector3(x, 0, -9.5), Vector3(x + 0.07, 0, -6.5), "wood_dark", 0.05)
		x += 0.9
	ZoneKit.plaza(m, th, PLAZA, PLAZA_RADIUS)
	for pd: Array in LAYOUT["pads"] as Array:
		ZoneKit.pad(m, th, pd[0] as Vector3)
		ZoneKit.glow_ring(m, pd[0] as Vector3, 3.0)
	# compass rose on the plaza
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(2.2, 2.2, 1.0)), PLAZA + Vector3(0, 0.09, 0)))
	m.extrude(HubArt._star(1.0, 0.3), 0.01, "water_deep")
	m.pop()
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
		var i0: Vector3 = d0 * (32.0 / maxf(absf(d0.x), absf(d0.z)))
		var i1: Vector3 = d1 * (32.0 / maxf(absf(d1.x), absf(d1.z)))
		var o0: Vector3 = d0 * ISLAND_RADIUS + Vector3.DOWN * 0.03
		var o1: Vector3 = d1 * ISLAND_RADIUS + Vector3.DOWN * 0.03
		m.quad(i0 + Vector3.UP * 0.002, i1 + Vector3.UP * 0.002, o1, o0, "path")
		var w0: Vector3 = d0 * (ISLAND_RADIUS + 9.0) + Vector3.DOWN * 0.04
		var w1: Vector3 = d1 * (ISLAND_RADIUS + 9.0) + Vector3.DOWN * 0.04
		m.quad(o0 + Vector3.UP * 0.005, o1 + Vector3.UP * 0.005, w1, w0, "water")
		var f0: Vector3 = d0 * (ISLAND_RADIUS + 0.9)
		var f1: Vector3 = d1 * (ISLAND_RADIUS + 0.9)
		m.quad(o0 + Vector3.UP * 0.01, o1 + Vector3.UP * 0.01, f1, f0, "flower_white")


## Far scenery (no outline, no shadow): the sea with a deep ring, islets with palms and sea stacks.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9441
	ZoneKit.far(m, theme(), rng, false)
	HubArt._annulus(m, Vector3(0, -0.045, 0), 70.0, 168.0, 24, "tile_blue_dark")
	for i: int in 6:
		var a: float = TAU * (float(i) + 0.4) / 6.0 + 0.2
		var r: float = rng.randf_range(78.0, 112.0)
		var c: Vector3 = Vector3(cos(a) * r, -0.5, sin(a) * r)
		HubArt._disk(m, c + Vector3.UP * 0.47, rng.randf_range(8.0, 12.0), 12, "path")
		m.blob(c, Vector3(rng.randf_range(5.0, 8.0), rng.randf_range(2.5, 5.0), rng.randf_range(5.0, 7.0)), "hill", 2, 0.04, rng)
		for k: int in 2:
			var p: Vector3 = c + Vector3(rng.randf_range(-3.0, 3.0), 2.0, rng.randf_range(-3.0, 3.0))
			m.cylinder(p, 0.5, 0.35, 6.0, 6, "trunk")
			m.blob(p + Vector3.UP * 6.2, Vector3(3.0, 1.0, 3.0), "leaf", 1, 0.05, rng)
	for i: int in 4:
		var a: float = TAU * float(i) / 4.0 + 0.9
		var c: Vector3 = Vector3(cos(a) * 62.0, 0, sin(a) * 62.0)
		m.blob(c, Vector3(2.5, rng.randf_range(4.0, 7.0), 2.2), "stone_dark", 1, 0.08, rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_lighthouse(m)
	m.push_at(HUT[0] as Vector3, float(HUT[1]))
	ZoneKit.house(m, HUT[2] as Vector2, "cloth_blue", "tile_blue_dark", "cloth_blue", "flower_white")
	m.pop()
	return m.build()


## Lighthouse on a rocky islet behind the wind gate: red-and-white striped tapering tower, a gallery with a railing,
## the glass lantern room with a warm light, a red dome and a keeper's hut on the rocks; a little jetty.
static func _lighthouse(m: LowPoly) -> void:
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9442
	m.push_at(ZoneKit.BACKDROP + Vector3(0, 0, -1.0))
	for p: Vector3 in [Vector3(0, 0, 0), Vector3(-3.4, 0, 1.2), Vector3(3.0, 0, 1.6), Vector3(-1.5, 0, -2.4), Vector3(2.2, 0, -2.0)]:
		m.blob(p + Vector3.UP * 0.4, Vector3(rng.randf_range(2.6, 3.4), rng.randf_range(1.2, 1.8), rng.randf_range(2.2, 2.8)), "stone_dark", 1, 0.1, rng)
	m.cylinder(Vector3(0, 1.4, 0), 2.3, 2.3, 0.5, 20, "stone", "stone")
	var y: float = 1.9
	for k: int in 6:
		var r0: float = 1.8 - 0.12 * k
		var r1: float = 1.8 - 0.12 * (k + 1)
		m.cylinder(Vector3(0, y, 0), r0, r1, 1.4, 16, "cloth_red" if k % 2 == 0 else "flower_white", "", false)
		y += 1.4
	m.cylinder(Vector3(0, y, 0), 1.6, 1.6, 0.2, 16, "wood_dark", "wood_dark")
	for k: int in 12:
		var a: float = TAU * float(k) / 12.0
		m.cylinder(Vector3(cos(a) * 1.45, y + 0.2, sin(a) * 1.45), 0.04, 0.04, 0.6, 5, "flower_white")
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, y + 0.8, 0)))
	HubArt._torus_ring(m, Vector3.ZERO, 1.45, 0.05, 20, 4, "flower_white")
	m.pop()
	m.cylinder(Vector3(0, y + 0.2, 0), 0.95, 0.95, 1.4, 12, "wind", "", false)
	m.blob(Vector3(0, y + 0.9, 0), Vector3(0.55, 0.55, 0.55), "lantern_glow", 1)
	m.cylinder(Vector3(0, y + 1.6, 0), 1.15, 0.2, 1.0, 12, "cloth_red", "cloth_red")
	m.blob(Vector3(0, y + 2.75, 0), Vector3(0.18, 0.18, 0.18), "lantern_glow", 0)
	m.box(Vector3(0, 2.6, 1.82), Vector3(0.9, 1.5, 0.12), "wood_dark")
	# keeper's hut
	m.push_at(Vector3(-4.2, 1.3, 1.0), 20.0)
	m.box(Vector3(0, 1.0, 0), Vector3(2.6, 2.0, 2.2), "flower_white")
	m.box(Vector3(0, 0.75, 1.12), Vector3(0.7, 1.4, 0.08), "cloth_blue")
	m.tiled_roof(Vector3(0, 2.0, 0), 2.8, 2.4, 1.0, 0.3, 0.2, "cloth_red", "roof_dark")
	m.pop()
	# jetty with posts
	m.box(Vector3(4.6, 0.55, 3.6), Vector3(1.6, 0.16, 5.0), "wood")
	for z: float in [1.5, 3.6, 5.7]:
		for x: float in [3.9, 5.3]:
			m.cylinder(Vector3(x, -0.5, z), 0.1, 0.1, 1.1, 6, "wood_dark")
	m.pop()


# ---------- set pieces at the anchors (local coordinates; front along local +Z) ----------

## Old wooden shipwreck lying in the sand, tilted (beside discovery.ocean.shipwreck): a toy-like hull with a red band
## and portholes, a broken mast with a torn sail, a treasure chest of gold coins, barrels and a crate (no cannons).
static func shipwreck() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.push(Transform3D((Basis(Vector3.BACK, 0.2) * Basis(Vector3.UP, PI * 0.5)).scaled(Vector3(1.25, 1.25, 1.25)), Vector3(0.3, -0.3, 0)))
	var hull: PackedVector2Array = PackedVector2Array([Vector2(-3.5, 1.9), Vector2(-3.2, 0.5), Vector2(-2.3, 0.0), Vector2(2.2, 0.0),
		Vector2(3.2, 0.6), Vector2(3.9, 2.1), Vector2(3.0, 1.6), Vector2(-3.0, 1.6)])
	m.extrude(hull, 2.5, "wood", "wood_dark")
	for s: float in [-1.0, 1.0]:
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0, s * 1.27)))
		m.extrude(PackedVector2Array([Vector2(-3.25, 1.05), Vector2(3.45, 1.05), Vector2(3.6, 1.35), Vector2(-3.35, 1.35)]), 0.06, "cloth_red")
		for x: float in [-1.6, 0.0, 1.6]:
			m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(x, 0.65, s * 0.02)))
			m.cylinder(Vector3(0, -0.04, 0), 0.2, 0.2, 0.08, 10, "eye")
			m.pop()
		m.pop()
	m.box(Vector3(-0.2, 1.62, 0), Vector3(5.8, 0.08, 2.2), "wood_dark")
	# broken mast and torn sail
	m.push(Transform3D(Basis(Vector3.BACK, -0.25), Vector3(0.4, 1.6, 0)))
	m.cylinder(Vector3.ZERO, 0.13, 0.1, 3.2, 8, "wood_dark")
	m.box(Vector3(0, 2.7, 0), Vector3(0.1, 0.1, 2.0), "wood_dark")
	m.push(Transform3D(Basis(Vector3.UP, PI * 0.5), Vector3(0.05, 1.7, 0)))
	m.extrude(PackedVector2Array([Vector2(-0.95, 0.95), Vector2(0.95, 0.95), Vector2(0.8, -0.4), Vector2(0.3, -0.1), Vector2(0.0, -0.6),
		Vector2(-0.5, -0.2), Vector2(-0.9, -0.5)]), 0.04, "flower_white")
	m.pop()
	m.pop()
	m.pop()
	# treasure chest (front, open, gold coins) and barrels
	var c: Vector3 = Vector3(-1.9, 0, 3.4)
	m.box(c + Vector3(0, 0.3, 0), Vector3(1.0, 0.6, 0.7), "wood", "lantern_glow")
	m.box(c + Vector3(0, 0.3, 0.36), Vector3(1.04, 0.12, 0.04), "bronze")
	m.push(Transform3D(Basis(Vector3.RIGHT, -1.0), c + Vector3(0, 0.6, -0.35)))
	m.box(Vector3(0, 0.0, 0.35), Vector3(1.0, 0.1, 0.7), "wood_dark")
	m.pop()
	for p: Vector3 in [Vector3(-0.25, 0.66, 0.1), Vector3(0.2, 0.68, -0.05), Vector3(0.0, 0.7, 0.15)]:
		m.cylinder(c + p, 0.1, 0.1, 0.04, 8, "lantern_glow")
	for b: Array in [[Vector3(1.6, 0, 3.0), false], [Vector3(2.3, 0, 2.2), true]]:
		var bp: Vector3 = b[0] as Vector3
		if bool(b[1]):
			m.push(Transform3D(Basis(Vector3.BACK, PI * 0.5), bp + Vector3(0, 0.35, 0)))
			m.cylinder(Vector3(0, -0.45, 0), 0.34, 0.34, 0.9, 10, "wood")
			m.pop()
		else:
			m.lathe(bp, PackedVector2Array([Vector2(0.3, 0), Vector2(0.38, 0.45), Vector2(0.3, 0.9)]), 10, "wood")
			HubArt._disk(m, bp + Vector3.UP * 0.9, 0.3, 10, "wood_dark")
	m.box(Vector3(2.8, 0.3, 3.6), Vector3(0.6, 0.6, 0.6), "basket")
	return m.build()


## Coral cave (beside discovery.ocean.coral_cave): a rock mound with a dark arched mouth, branching corals, anemones,
## shells, starfish and glowing pearls at the entrance.
static func coral_cave() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9445
	for p: Array in [[Vector3(-2.1, 0, -0.4), Vector3(1.6, 2.5, 2.2)], [Vector3(2.1, 0, -0.4), Vector3(1.6, 2.3, 2.2)],
			[Vector3(0, 2.3, -0.6), Vector3(3.2, 1.4, 2.2)], [Vector3(0, 0.6, -2.0), Vector3(3.0, 2.4, 1.6)]]:
		m.blob(p[0] as Vector3 + Vector3.UP * 0.9, p[1] as Vector3, "stone", 1, 0.08, rng)
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0, 0.9)))
	m.extrude(ZoneKit.arch(1.15, 2.5, 10), 0.3, "eye")
	m.pop()
	var corals: PackedStringArray = ["flower_pink", "cloth_red", "cloth_violet", "lantern_glow"]
	for k: int in 7:
		var a: float = PI * (0.1 + 0.8 * float(k) / 6.0)
		var base: Vector3 = Vector3(cos(a) * 2.6, 0, sin(a) * 1.7 + 0.9)
		_coral(m, base, corals[k % corals.size()], rng)
	for p: Vector3 in [Vector3(-0.8, 0.12, 1.6), Vector3(0.9, 0.12, 1.8), Vector3(0.1, 0.12, 2.2)]:
		m.blob(p, Vector3(0.12, 0.12, 0.12), "spark", 0)
	for p: Vector3 in [Vector3(-1.6, 0.05, 2.4), Vector3(1.8, 0.05, 2.6)]:
		_starfish(m, p, "cloth_red")
	return m.build()


## Branching coral: three or four rounded branches with blob tips.
static func _coral(m: LowPoly, base: Vector3, colour: String, rng: RandomNumberGenerator) -> void:
	for b: int in 3:
		var tip: Vector3 = base + Vector3(rng.randf_range(-0.45, 0.45), rng.randf_range(0.6, 1.1), rng.randf_range(-0.3, 0.3))
		HubArt._limb(m, base, tip, 0.09, 0.06, colour)
		m.blob(tip, Vector3(0.11, 0.11, 0.11), colour, 0)


## Starfish lying flat (closed five-point star).
static func _starfish(m: LowPoly, p: Vector3, colour: String) -> void:
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(0.3, 0.3, 1.0)), p))
	m.extrude(HubArt._star(1.0, 0.42), 0.08, colour)
	m.pop()


## Shell arch over the lane (collectible.ocean.rare_shell under it): two coral-encrusted stone columns at x = ±2.7
## crowned by a giant pink scallop with white ribs and a pearl.
static func shell_arch() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9446
	for s: float in [-1.0, 1.0]:
		m.cylinder(Vector3(s * 2.7, 0, 0), 0.6, 0.45, 3.4, 10, "stone")
		for k: int in 3:
			m.blob(Vector3(s * (2.7 + 0.35 * (1 if k % 2 == 0 else -1)), 0.8 + k * 0.9, 0.35), Vector3(0.22, 0.2, 0.2),
					["flower_pink", "cloth_violet", "cloth_red"][k], 0)
		m.blob(Vector3(s * 2.7, 3.5, 0), Vector3(0.6, 0.35, 0.6), "stone", 1)
	m.box(Vector3(0, 3.55, 0), Vector3(5.8, 0.35, 0.5), "stone")
	# giant scallop: fan of ribs
	var ribs: int = 9
	for k: int in ribs:
		var a0: float = PI * (0.05 + 0.9 * float(k) / float(ribs))
		var a1: float = PI * (0.05 + 0.9 * float(k + 1) / float(ribs))
		var pts: PackedVector2Array = PackedVector2Array([Vector2(0, 0), Vector2(cos(a0), sin(a0)) * 2.0, Vector2(cos((a0 + a1) * 0.5), sin((a0 + a1) * 0.5)) * 2.15,
			Vector2(cos(a1), sin(a1)) * 2.0])
		m.push(Transform3D(Basis.IDENTITY, Vector3(0, 3.7, 0.05 * (k % 2))))
		m.extrude(pts, 0.25, "flower_pink" if k % 2 == 0 else "flower_white")
		m.pop()
	m.box(Vector3(0, 3.75, 0.1), Vector3(1.0, 0.4, 0.4), "flower_pink")
	m.blob(Vector3(0, 4.7, 0.3), Vector3(0.28, 0.28, 0.28), "spark", 1)
	return m.build()


## Riddle Octopus statue (Bạch Tuộc Câu Đố, the friendly boss of chapter 5): a smiling violet octopus with big eyes
## sitting on a rock, its curled arms round the rock.
static func octopus_rock() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9447
	m.blob(Vector3(0, 0.5, 0), Vector3(1.6, 0.9, 1.4), "stone_dark", 1, 0.1, rng)
	m.blob(Vector3(0, 2.4, 0), Vector3(1.0, 1.1, 0.95), "cloth_violet", 2)
	for s: float in [-1.0, 1.0]:
		m.blob(Vector3(s * 0.38, 2.45, 0.78), Vector3(0.3, 0.34, 0.16), "eye_white", 1)
		m.blob(Vector3(s * 0.36, 2.4, 0.9), Vector3(0.16, 0.2, 0.08), "eye", 1)
		m.blob(Vector3(s * 0.32, 2.5, 0.96), Vector3(0.05, 0.05, 0.03), "eye_light", 0)
		m.blob(Vector3(s * 0.62, 2.05, 0.72), Vector3(0.12, 0.08, 0.05), "cheek", 0)
	m.blob(Vector3(0, 1.98, 0.88), Vector3(0.16, 0.07, 0.05), "mouth", 0)
	for k: int in 6:
		var a: float = TAU * float(k) / 6.0 + 0.3
		var prev: Vector3 = Vector3(cos(a) * 0.6, 1.6, sin(a) * 0.6)
		for j: int in 4:
			var t: float = float(j + 1) / 4.0
			var r: float = 0.6 + 1.0 * t
			var q: Vector3 = Vector3(cos(a + t * 0.6) * r, 1.6 - 1.3 * t + 0.25 * sin(t * PI), sin(a + t * 0.6) * r)
			HubArt._limb(m, prev, q, 0.2 - 0.04 * j, 0.17 - 0.04 * j, "cloth_violet")
			prev = q
		m.blob(prev, Vector3(0.1, 0.1, 0.1), "flower_pink", 0)
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9443
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = PLAZA + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (PLAZA_RADIUS - 0.6)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "lantern")
	for b: Array in [[Vector3(-4.6, 0.08, 9.6), 40.0], [Vector3(4.6, 0.08, 9.6), -40.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	ZoneKit.signpost(m, Vector3(3.0, 0, 11.5), [[Vector3(0, 0, -20), "home", "roof"], [Vector3(14, 0, -8), "wave", "water_deep"],
		[Vector3(-14, 0, -8), "star", "flower_pink"], [SHELL, "star", "lantern_glow"]])
	# basket boats on the sand, rope posts, buoys, starfish and shells
	for b: Array in [[Vector3(-20.0, 0.1, 19.0), 20.0], [Vector3(-16.5, 0.1, 21.0), -35.0], [Vector3(21.0, 0.1, -19.5), 60.0]]:
		m.push_at(b[0] as Vector3, float(b[1]))
		HubArt._basket_boat(m, 1.0)
		m.pop()
	for z: float in [-3.0, -12.0]:
		for s: float in [-1.0, 1.0]:
			var p: Vector3 = Vector3(s * 2.4, 0, z)
			m.cylinder(p, 0.12, 0.1, 1.0, 8, "wood_dark")
			m.blob(p + Vector3.UP * 1.05, Vector3(0.14, 0.1, 0.14), "wood_dark", 0)
	for p: Array in [[Vector3(21.5, 0, 6.0), "cloth_red"], [Vector3(-21.5, 0, -1.0), "lantern_glow"], [Vector3(9.0, 0, 21.5), "cloth_red"]]:
		m.blob(p[0] as Vector3 + Vector3.UP * 0.35, Vector3(0.4, 0.4, 0.4), String(p[1]), 1)
		m.cylinder(p[0] as Vector3 + Vector3.UP * 0.7, 0.08, 0.05, 0.4, 6, "flower_white")
	for p: Vector3 in [Vector3(-6.0, 0.02, -2.0), Vector3(7.5, 0.02, 1.0), Vector3(-9.0, 0.02, 15.0), Vector3(10.0, 0.02, -14.0)]:
		_starfish(m, p, "cloth_red" if p.x < 0.0 else "lantern_glow")
	for p: Vector3 in [Vector3(-5.0, 0, 13.0), Vector3(6.0, 0, -18.0), Vector3(-8.0, 0, -15.0)]:
		ZoneKit.cone(m, p, 0.28, 0.42, 8, "flower_pink")
	for p: Vector3 in [Vector3(-3.4, 0, -21.8), Vector3(3.4, 0, -21.8)]:
		HubArt._stone_lantern(m, p)
	# a beach umbrella and towels by the hut
	m.cylinder(Vector3(12.0, 0, 18.0), 0.04, 0.04, 2.3, 6, "wood_dark")
	for k: int in 8:
		var a0: float = TAU * float(k) / 8.0
		var a1: float = TAU * float(k + 1) / 8.0
		var e0: Vector3 = Vector3(12.0 + cos(a0) * 1.6, 2.0, 18.0 + sin(a0) * 1.6)
		var e1: Vector3 = Vector3(12.0 + cos(a1) * 1.6, 2.0, 18.0 + sin(a1) * 1.6)
		var col: String = "cloth_red" if k % 2 == 0 else "flower_white"
		m.tri(e0, Vector3(12.0, 2.6, 18.0), e1, col)
		m.tri(e1, Vector3(12.0, 2.15, 18.0), e0, col)
	m.box(Vector3(10.6, 0.03, 18.6), Vector3(0.9, 0.04, 1.8), "cloth_blue")
	for p: Vector3 in [Vector3(-21.5, 0, -21.0), Vector3(21.5, 0, 11.0), Vector3(-21.8, 0, 6.0)]:
		ZoneKit.rock(m, p, Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)), "stone_dark", rng)
	return m.build()


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"palm": "veg_palm_coconut_a", "bush": "veg_bush_a", "flower": "veg_flower_a", "tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"palm":
			return ZoneKit.scatter_in(9450, 10, 18, 1.8, LAYOUT, BLOCKED, 0.85, 1.2, Vector2(26.0, 34.0))
		"bush":
			return ZoneKit.scatter_in(9453, 18, 0, 1.0, LAYOUT, BLOCKED, 0.7, 1.1)
		"flower":
			return ZoneKit.scatter_in(9454, 60, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter_in(9455, 220, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.4)
	return []
