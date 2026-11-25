extends RefCounted
## Recipes of "Vương quốc Số" (Math Kingdom, `learning_zone.math`; F093, WORLD_DESIGN §2.2 chapter 1 "sửa cầu, chia
## hàng", §4.2, §11; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## Set pieces: the Number Castle with glazed blue tiles behind the wind gate, the arched Number Bridge over a stream
## (discovery.math.number_bridge), the village well with a little tiled roof (discovery.math.old_well), the Abacus Arch
## over the main lane (collectible.math.shell_abacus), the fruit stall with apple baskets (quest.math.*, F072), a
## bamboo water wheel (cọn nước), blue-roofed houses, counting blocks with dice pips and geometric topiary. Colours are
## art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "math"
const THEME_OVERRIDES: Dictionary = {"paving_dark": "stone", "hedge_top": "leaf_light"}
## Stream centre line x(z) (west of the cross path); the Number Bridge spans it at z = -6.
const STREAM_X: float = -16.0
const STREAM_WIDTH: float = 2.4
const BRIDGE_CENTER: Vector3 = Vector3(-16, 0, -6)
const BRIDGE_HALF: float = 3.6
const BRIDGE_RISE: float = 0.9
const BRIDGE_WIDTH: float = 2.2
const WELL_CENTER: Vector3 = Vector3(15.5, 0, -6)
const ABACUS_CENTER: Vector3 = Vector3(0, 0, -14)
const ABACUS_POST_X: float = 2.9

## Fruit stall behind quest.math.fruit_stall (8, 0, 6); apple baskets beside quest.math.apple_basket_1–3 (F072 seed).
const STALL_CENTER: Vector3 = Vector3(11, 0, 4.4)
const BASKETS: Array[Vector3] = [Vector3(10.7, 0, 9.3), Vector3(12.7, 0, 7.6), Vector3(14.7, 0, 10.3)]
const WHEEL_CENTER: Vector3 = Vector3(-16, 0, -15.5)
## Blue-roofed houses: [centre, yaw (deg, front door along local +Z), footprint (w, d)] — HouseCollision* in the scene.
const HOUSES: Array = [
	[Vector3(-8.5, 0, 18.0), 90.0, Vector2(5.0, 4.0)],
	[Vector3(18.0, 0, -16.0), -90.0, Vector2(5.0, 4.0)],
]
## Rectangles kept free of scatter: [centre, half size].
const BLOCKED: Array = [
	[Vector3(-16, 0, 0), Vector2(2.4, 26.0)],  # stream
	[Vector3(-8.5, 0, 18.0), Vector2(3.2, 3.2)],
	[Vector3(18.0, 0, -16.0), Vector2(3.2, 3.2)],
	[Vector3(15.5, 0, -6), Vector2(2.4, 2.4)],
	[Vector3(11.5, 0, 7.0), Vector2(4.5, 4.5)],
	[Vector3(0, 0, -31), Vector2(10.0, 4.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_math_terrain_a": terrain, "env/env_math_far_a": far, "bld/bld_math_set_a": buildings, "prop/prop_math_set_a": props,
		"prop/prop_math_bridge_a": number_bridge, "prop/prop_math_well_a": old_well, "prop/prop_math_abacus_a": abacus_arch,
		"prop/prop_math_stall_a": fruit_stall,
	}


static func stream_x(z: float) -> float:
	return STREAM_X + 1.2 * sin((z + 6.0) * 0.24)


## Deck height of the Number Bridge at local x (−BRIDGE_HALF…BRIDGE_HALF): a smooth arch, 0 at both feet. The
## BridgeDeck collision (ConvexPolygonShape3D) of the zone scene samples the same curve.
static func deck_height(x: float) -> float:
	return BRIDGE_RISE * cos(clampf(x / BRIDGE_HALF, -1.0, 1.0) * PI * 0.5)


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9301
	var th: Dictionary = theme()
	ZoneKit.terrain(m, th)
	# Cross path continues over the bridge to a lookout and to the well.
	ZoneKit.path_strip(m, Vector3(-22.5, 0, -7.2), Vector3(-15.0, 0, -4.8), "path")
	ZoneKit.path_strip(m, Vector3(15.0, 0, -7.4), Vector3(17.5, 0, -4.6), "path")
	# Blue tile inlay ring of the plaza.
	HubArt._annulus(m, ZoneKit.PLAZA_CENTER + Vector3.UP * 0.09, 5.55, 5.85, 32, "tile_blue")
	_stream(m, rng)
	# Market corner: packed earth under the fruit stall and the baskets.
	HubArt._disk(m, Vector3(11.5, 0.04, 7.0), 4.4, 18, "path")
	ZoneKit.boundary(m, th)
	return m.build()


## Stream from a spring pond in the north-west to a pond in the south-west, with soil banks and pebbles.
static func _stream(m: LowPoly, rng: RandomNumberGenerator) -> void:
	var z0: float = -22.0
	var z1: float = 21.0
	var steps: int = 22
	var y: float = 0.05
	var half: float = STREAM_WIDTH * 0.5
	for i: int in steps:
		var za: float = lerpf(z0, z1, float(i) / steps)
		var zb: float = lerpf(z0, z1, float(i + 1) / steps)
		var xa: float = stream_x(za)
		var xb: float = stream_x(zb)
		m.quad(Vector3(xa - half, y, za), Vector3(xb - half, y, zb), Vector3(xb + half, y, zb), Vector3(xa + half, y, za), "water")
		m.quad(Vector3(xa - 0.35, y + 0.003, za), Vector3(xb - 0.35, y + 0.003, zb), Vector3(xb + 0.35, y + 0.003, zb),
				Vector3(xa + 0.35, y + 0.003, za), "water_deep")
		for s: float in [-1.0, 1.0]:
			var ia: Vector3 = Vector3(xa + s * half, y, za)
			var ib: Vector3 = Vector3(xb + s * half, y, zb)
			var oa: Vector3 = Vector3(xa + s * (half + 0.45), 0.02, za)
			var ob: Vector3 = Vector3(xb + s * (half + 0.45), 0.02, zb)
			if s > 0.0:
				m.quad(ia, ib, ob, oa, "soil")
			else:
				m.quad(ia, oa, ob, ib, "soil")
		if i % 2 == 0:
			var side: float = 1.0 if i % 4 == 0 else -1.0
			m.blob(Vector3(xa + side * (half + 0.2), 0.08, za + 0.6), Vector3(0.32, 0.18, 0.26), "stone", 0, 0.12, rng)
	for c: Vector3 in [Vector3(stream_x(z0), 0, z0 - 1.2), Vector3(stream_x(z1), 0, z1 + 1.4)]:
		HubArt._disk(m, c + Vector3.UP * (y + 0.002), 2.6, 16, "water")
		HubArt._annulus(m, c + Vector3.UP * (y + 0.002), 2.6, 3.0, 16, "soil")
		for k: int in 6:
			var a: float = TAU * float(k) / 6.0 + 0.3
			m.blob(c + Vector3(cos(a) * 3.0, 0.1, sin(a) * 3.0), Vector3(0.4, 0.22, 0.34), "stone", 0, 0.12, rng)
	# lily pads and a lotus in the south pond
	var pond: Vector3 = Vector3(stream_x(z1), y + 0.01, z1 + 1.4)
	for p: Vector3 in [Vector3(-0.9, 0, 0.4), Vector3(0.8, 0, -0.6), Vector3(0.2, 0, 1.1)]:
		HubArt._disk(m, pond + p, 0.36, 8, "leaf")
	HubArt._lotus(m, pond + Vector3(-0.9, 0.02, 0.4))


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9309
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_castle(m)
	for h: Array in HOUSES:
		m.push_at(h[0] as Vector3, float(h[1]))
		ZoneKit.house(m, h[2] as Vector2, "tile_blue", "tile_blue_dark", "tile_blue", "lantern_glow")
		m.pop()
	return m.build()


## Number Castle (landmark behind the wind gate): stone plinth, cream gatehouse with an arched door under a two-tier
## glazed blue roof, crenellated walls and two round towers with blue cone roofs crowned by a sphere and a cube.
static func _castle(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP)
	m.box(Vector3(0, 0.3, 0), Vector3(18.0, 0.6, 6.2), "stone", "stone")
	m.box(Vector3(0, 2.9, 0), Vector3(6.4, 4.6, 4.4), "wall")
	m.box(Vector3(0, 0.85, 0), Vector3(6.6, 0.5, 4.6), "tile_blue")
	# arched gate: blue frame and a dark wooden door, with a round window above
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.6, 2.22)))
	m.extrude(ZoneKit.arch(1.35, 3.4, 10), 0.1, "tile_blue")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.6, 2.3)))
	m.extrude(ZoneKit.arch(1.05, 3.0, 10), 0.1, "wood_dark")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 4.45, 2.24)))
	m.extrude(ZoneKit.circle(0.5, 12), 0.1, "lantern_glow", "tile_blue_dark")
	m.pop()
	m.box(Vector3(0, 5.32, 0), Vector3(6.8, 0.35, 4.8), "tile_blue_dark")
	m.tiled_roof(Vector3(0, 5.45, 0), 6.6, 4.6, 2.0, 0.65, 0.4, "tile_blue", "tile_blue_dark")
	m.box(Vector3(0, 7.2, 0), Vector3(3.2, 0.6, 2.2), "wall")
	m.tiled_roof(Vector3(0, 7.5, 0), 3.4, 2.4, 1.2, 0.45, 0.3, "tile_blue", "tile_blue_dark")
	# pyramid finial on the top ridge
	m.cylinder(Vector3(0, 8.85, 0), 0.42, 0.0, 0.8, 4, "lantern_glow", "", false, 0.5)
	for s: float in [-1.0, 1.0]:
		# curtain wall with merlons
		m.box(Vector3(s * 4.9, 2.1, 0.2), Vector3(3.4, 3.6, 1.6), "wall")
		m.box(Vector3(s * 4.9, 0.85, 0.2), Vector3(3.5, 0.5, 1.7), "tile_blue")
		for k: int in 3:
			m.box(Vector3(s * (3.75 + k * 1.1), 4.15, 0.2), Vector3(0.62, 0.5, 1.6), "wall", "tile_blue")
		# round tower
		var tx: float = s * 7.3
		m.cylinder(Vector3(tx, 0.6, 0), 2.0, 1.85, 6.4, 16, "wall")
		m.cylinder(Vector3(tx, 0.6, 0), 2.08, 2.08, 0.45, 16, "tile_blue", "", false)
		m.cylinder(Vector3(tx, 6.75, 0), 2.05, 2.1, 0.4, 16, "tile_blue_dark", "", false)
		for wy: float in [2.6, 4.9]:
			m.push(Transform3D(Basis.IDENTITY, Vector3(tx, wy, 1.86)))
			m.extrude(ZoneKit.arch(0.32, 0.95, 6), 0.12, "wood_dark", "tile_blue_dark")
			m.pop()
		m.cylinder(Vector3(tx, 7.0, 0), 2.55, 2.45, 0.25, 16, "tile_blue_dark", "", false)
		m.cylinder(Vector3(tx, 7.25, 0), 2.45, 0.0, 3.6, 16, "tile_blue")
		m.cylinder(Vector3(tx, 10.75, 0), 0.05, 0.05, 1.3, 6, "wood_dark")
		if s < 0.0:
			m.blob(Vector3(tx, 11.0, 0), Vector3(0.42, 0.42, 0.42), "lantern_glow", 1)
		else:
			m.push(Transform3D(Basis(Vector3.UP, PI * 0.25) * Basis(Vector3.RIGHT, PI * 0.2), Vector3(tx, 11.05, 0)))
			m.box(Vector3.ZERO, Vector3(0.62, 0.62, 0.62), "lantern_glow")
			m.pop()
		# pennant in the subject colour
		m.push(Transform3D(Basis.IDENTITY, Vector3(tx + 0.05, 11.75, 0)))
		m.extrude(PackedVector2Array([Vector2(0, 0.3), Vector2(1.1, 0.05), Vector2(0, -0.2)]), 0.04, "subject_math")
		m.pop()
	m.pop()




# ---------- set pieces at the anchors (local coordinates, placed by the zone scene) ----------

## Number Bridge (discovery.math.number_bridge stands at its east foot): an arched wooden deck over the stream (along
## local X), stone abutments, railings whose posts carry counting beads, and geometric finials on the four end posts.
static func number_bridge() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var steps: int = 12
	var top: PackedVector2Array = PackedVector2Array()
	var bottom: PackedVector2Array = PackedVector2Array()
	for i: int in steps + 1:
		var x: float = lerpf(-BRIDGE_HALF, BRIDGE_HALF, float(i) / steps)
		top.append(Vector2(x, deck_height(x) + 0.08))
		bottom.append(Vector2(x, maxf(deck_height(x) - 0.28, -0.05)))
	var deck: PackedVector2Array = top.duplicate()
	bottom.reverse()
	deck.append_array(bottom)
	m.extrude(deck, BRIDGE_WIDTH, "wood", "wood_dark")
	# plank lines across the deck
	for i: int in range(1, steps):
		var x: float = lerpf(-BRIDGE_HALF, BRIDGE_HALF, float(i) / steps)
		var y: float = deck_height(x) + 0.09
		var slope: float = atan(-BRIDGE_RISE * PI * 0.5 / BRIDGE_HALF * sin(x / BRIDGE_HALF * PI * 0.5))
		m.push(Transform3D(Basis(Vector3.BACK, slope), Vector3(x, y, 0)))
		m.box(Vector3.ZERO, Vector3(0.05, 0.02, BRIDGE_WIDTH - 0.1), "wood_dark")
		m.pop()
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * (BRIDGE_HALF + 0.35), 0.18, 0), Vector3(0.9, 0.36, BRIDGE_WIDTH + 0.6), "stone", "stone")
	# railings: posts with counting beads, rails following the arch
	var colours: PackedStringArray = ["subject_math", "lantern_glow", "cloth_red", "leaf", "tile_blue"]
	for z: float in [-BRIDGE_WIDTH * 0.5 + 0.08, BRIDGE_WIDTH * 0.5 - 0.08]:
		var prev: Vector3 = Vector3.ZERO
		for i: int in 7:
			var x: float = lerpf(-BRIDGE_HALF + 0.25, BRIDGE_HALF - 0.25, float(i) / 6.0)
			var base: Vector3 = Vector3(x, deck_height(x) + 0.05, z)
			var end_post: bool = i == 0 or i == 6
			m.cylinder(base, 0.09 if end_post else 0.06, 0.08 if end_post else 0.055, 0.95, 8, "wood_dark")
			var cap: Vector3 = base + Vector3.UP * 1.05
			if end_post:
				_finial(m, cap + Vector3.UP * 0.12, (1 if i == 6 else 0) + (2 if z > 0.0 else 0))
			else:
				m.blob(cap, Vector3(0.13, 0.13, 0.13), colours[i % colours.size()], 0)
			if i > 0:
				HubArt._limb(m, prev + Vector3.UP * 0.82, base + Vector3.UP * 0.82, 0.04, 0.04, "wood")
				HubArt._limb(m, prev + Vector3.UP * 0.45, base + Vector3.UP * 0.45, 0.03, 0.03, "wood")
			prev = base
	return m.build()


## Geometric finial: 0 sphere, 1 cube (diamond), 2 cone, 3 pyramid — shapes, not colour, tell them apart.
static func _finial(m: LowPoly, p: Vector3, kind: int) -> void:
	match kind % 4:
		0:
			m.blob(p, Vector3(0.2, 0.2, 0.2), "tile_blue", 0)
		1:
			m.push(Transform3D(Basis(Vector3.UP, PI * 0.25) * Basis(Vector3.RIGHT, PI * 0.2), p))
			m.box(Vector3.ZERO, Vector3(0.28, 0.28, 0.28), "lantern_glow")
			m.pop()
		2:
			m.cylinder(p - Vector3.UP * 0.15, 0.2, 0.0, 0.4, 8, "cloth_red")
		_:
			m.cylinder(p - Vector3.UP * 0.15, 0.22, 0.0, 0.4, 4, "leaf", "", false, 0.5)


## Village well (giếng cổ, discovery.math.old_well): a round laterite-brick well with a stone rim, a wooden frame with
## a small blue tiled roof, a pulley, rope and bucket.
static func old_well() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.cylinder(Vector3.ZERO, 1.55, 1.55, 0.18, 16, "stone", "stone")
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(1.25, 0.0), Vector2(1.22, 0.75)]), 16, "roof")
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.95, 0.0), Vector2(0.95, 0.8)]), 16, "roof_dark", true)
	HubArt._ring(m, Vector3(0, 0.75, 0), 0.92, 1.32, 0.16, 16, "stone")
	HubArt._disk(m, Vector3(0, 0.45, 0), 0.95, 16, "water_deep")
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		m.box(Vector3(cos(a) * 1.24, 0.42, sin(a) * 1.24), Vector3(0.06, 0.06, 0.06), "roof_dark")
	for x: float in [-1.05, 1.05]:
		m.cylinder(Vector3(x, 0.9, 0), 0.09, 0.08, 1.9, 8, "wood_dark")
	m.box(Vector3(0, 2.6, 0), Vector3(2.5, 0.14, 0.14), "wood_dark")
	m.tiled_roof(Vector3(0, 2.75, 0), 2.6, 1.3, 0.75, 0.3, 0.2, "tile_blue", "tile_blue_dark")
	# pulley, rope, bucket
	m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(0, 2.4, 0)))
	m.cylinder(Vector3(0, -0.06, 0), 0.18, 0.18, 0.12, 10, "wood")
	m.pop()
	m.cylinder(Vector3(0.18, 1.45, 0), 0.015, 0.015, 0.95, 4, "basket", "", false)
	m.cylinder(Vector3(0.18, 1.05, 0), 0.16, 0.2, 0.32, 10, "wood", "wood_dark")
	# a bench stone and a water jar beside the well
	m.lathe(Vector3(1.9, 0, 0.9), PackedVector2Array([Vector2(0.16, 0), Vector2(0.32, 0.25), Vector2(0.28, 0.55), Vector2(0.14, 0.7),
		Vector2(0.17, 0.78)]), 10, "roof")
	return m.build()


## Abacus Arch over the main lane (collectible.math.shell_abacus stands under it): two posts, a top beam with a tiled
## cap and three rods of coloured counting beads.
static func abacus_arch() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var px: float = ABACUS_POST_X
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * px, 0.2, 0), Vector3(0.9, 0.4, 0.9), "stone", "stone")
		m.box(Vector3(s * px, 2.0, 0), Vector3(0.5, 3.3, 0.5), "wood_dark")
		m.blob(Vector3(s * px, 4.15, 0), Vector3(0.3, 0.3, 0.3), "lantern_glow", 1)
	m.box(Vector3(0, 3.75, 0), Vector3(2.0 * px + 0.9, 0.36, 0.55), "wood_dark")
	m.box(Vector3(0, 1.85, 0), Vector3(2.0 * px - 0.4, 0.2, 0.36), "wood")
	m.tiled_roof(Vector3(0, 3.92, 0), 2.0 * px + 0.9, 0.7, 0.45, 0.25, 0.18, "tile_blue", "tile_blue_dark")
	var rows: Array = [[2.25, 4, "subject_math"], [2.75, 3, "cloth_red"], [3.25, 5, "lantern_glow"]]
	for row: Array in rows:
		var y: float = float(row[0])
		m.push(Transform3D(Basis(Vector3.BACK, -PI * 0.5), Vector3(-px + 0.25, y, 0)))
		m.cylinder(Vector3.ZERO, 0.03, 0.03, 2.0 * px - 0.5, 6, "wood", "", false)
		m.pop()
		var count: int = int(row[1])
		for i: int in count:
			var x: float = -px + 0.55 + float(i) * 0.48
			m.blob(Vector3(x, y, 0), Vector3(0.2, 0.17, 0.2), String(row[2]), 1)
		for i: int in 2:
			m.blob(Vector3(px - 0.55 - float(i) * 0.48, y, 0), Vector3(0.2, 0.17, 0.2), "wall", 1)
	return m.build()


## Market stall (sạp trái cây) behind quest.math.fruit_stall: a wooden counter with fruit piles to share out, a
## striped awning, a hanging scale and crates; front along local +Z.
static func fruit_stall() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.box(Vector3(0, 0.5, 0), Vector3(3.2, 1.0, 1.4), "wood", "wood_dark")
	m.box(Vector3(0, 1.02, 0.1), Vector3(3.3, 0.08, 1.5), "wood_dark")
	for x: float in [-1.5, 1.5]:
		for z: float in [-0.65, 0.75]:
			m.cylinder(Vector3(x, 0.0, z), 0.07, 0.07, 2.6, 8, "wood_dark")
	# striped awning: alternating panels, tilted forwards
	for i: int in 6:
		var x0: float = -1.75 + i * (3.5 / 6.0)
		var x1: float = x0 + 3.5 / 6.0
		# closed thin slabs (not single quads): the inverted-hull outline needs a volume
		var stripe: String = "subject_math" if i % 2 == 0 else "flower_white"
		m.push(Transform3D(Basis(Vector3.RIGHT, 0.2), Vector3((x0 + x1) * 0.5, 2.55, 0.15)))
		m.box(Vector3.ZERO, Vector3(x1 - x0, 0.05, 2.05), stripe)
		m.box(Vector3(0, -0.12, 1.02), Vector3(x1 - x0, 0.26, 0.05), stripe)
		m.pop()
	# fruit piles: apples, oranges, bananas-yellow, grapes-violet, in bamboo trays
	var piles: Array = [[-1.05, "cloth_red"], [-0.35, "subject_vietnamese"], [0.35, "wall_yellow"], [1.05, "cloth_violet"]]
	for pile: Array in piles:
		var x: float = float(pile[0])
		m.cylinder(Vector3(x, 1.06, 0.15), 0.32, 0.36, 0.08, 10, "basket", "basket")
		for k: int in 5:
			var a: float = TAU * float(k) / 5.0
			m.blob(Vector3(x + cos(a) * 0.17, 1.24, 0.15 + sin(a) * 0.17), Vector3(0.12, 0.12, 0.12), String(pile[1]), 0)
		m.blob(Vector3(x, 1.4, 0.15), Vector3(0.12, 0.12, 0.12), String(pile[1]), 0)
	# hanging scale (cân đĩa) and crates
	m.cylinder(Vector3(1.2, 1.9, 0.6), 0.015, 0.015, 0.45, 4, "wood_dark", "", false)
	m.cylinder(Vector3(1.2, 1.88, 0.6), 0.22, 0.25, 0.05, 10, "bronze", "bronze")
	for c: Vector3 in [Vector3(-2.1, 0.25, 0.4), Vector3(2.15, 0.25, 0.1), Vector3(2.1, 0.72, 0.2)]:
		m.box(c, Vector3(0.6, 0.5, 0.5), "wood", "wood_dark")
	return m.build()


## Apple basket (rổ táo) of the quest objects: a woven bamboo basket filled with apples.
static func _apple_basket(m: LowPoly) -> void:
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.22, 0.0), Vector2(0.4, 0.12), Vector2(0.46, 0.4)]), 10, "basket")
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.22, 0.02), Vector2(0.4, 0.14), Vector2(0.44, 0.4)]), 10, "wood_dark", true)
	HubArt._ring(m, Vector3(0, 0.38, 0), 0.42, 0.49, 0.05, 10, "wood")
	for k: int in 6:
		var a: float = TAU * float(k) / 6.0
		m.blob(Vector3(cos(a) * 0.22, 0.42, sin(a) * 0.22), Vector3(0.12, 0.12, 0.12), "cloth_red", 0)
	m.blob(Vector3(0, 0.5, 0), Vector3(0.12, 0.12, 0.12), "cloth_red", 0)


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9302
	# blue lanterns round the plaza and along the lane
	for a: float in [30.0, 150.0, 210.0, 330.0]:
		var p: Vector3 = ZoneKit.PLAZA_CENTER + Vector3(cos(deg_to_rad(a)), 0, sin(deg_to_rad(a))) * (ZoneKit.PLAZA_RADIUS - 0.7)
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", "tile_blue")
	for z: float in [-12.0, -21.5]:
		for s: float in [-1.0, 1.0]:
			ZoneKit.lamp(m, Vector3(s * 3.1, 0, z), "wood_dark", "tile_blue")
	for b: Array in [[Vector3(-4.6, 0.08, 6.6), 50.0], [Vector3(4.6, 0.08, 6.6), -50.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	_chalkboard(m, Vector3(2.7, 0.08, 0.8), ZoneKit.yaw_to(Vector3(2.7, 0, 0.8), ZoneKit.SPAWN))
	# counting blocks (1, 2, 3 stacked dice) by the plaza and the market
	_counting_blocks(m, Vector3(-5.6, 0, -1.4), ZoneKit.yaw_to(Vector3(-5.6, 0, -1.4), ZoneKit.PLAZA_CENTER))
	_counting_blocks(m, Vector3(6.4, 0, 12.2), ZoneKit.yaw_to(Vector3(6.4, 0, 12.2), ZoneKit.SPAWN))
	# geometric topiary on pedestals along the lane: sphere, cube, cone, pyramid
	var topiary: Array = [[Vector3(-4.0, 0, -9.5), 0], [Vector3(4.0, 0, -9.5), 1], [Vector3(-4.0, 0, -17.5), 2],
		[Vector3(4.0, 0, -17.5), 3]]
	for t: Array in topiary:
		_topiary(m, t[0] as Vector3, int(t[1]))
	ZoneKit.flower_bed(m, Vector3(-7.5, 0, -12.0), 1.4, PackedStringArray(["flower_pink", "lantern_glow", "flower_white"]), rng)
	ZoneKit.flower_bed(m, Vector3(7.5, 0, -12.0), 1.4, PackedStringArray(["tile_blue", "flower_white", "flower_pink"]), rng)
	_water_wheel(m, WHEEL_CENTER)
	for p: Vector3 in BASKETS:
		m.push_at(p, rng.randf_range(0.0, 360.0))
		_apple_basket(m)
		m.pop()
	# lookout bench west of the bridge, rocks by the ring
	ZoneKit.bench(m, Vector3(-22.2, 0.0, -6.0), 90.0)
	for p: Vector3 in [Vector3(-21.5, 0, 21.0), Vector3(21.5, 0, 21.5), Vector3(21.8, 0, -21.6), Vector3(-6.5, 0, -21.8)]:
		m.blob(p + Vector3(0, 0.2, 0), Vector3(rng.randf_range(0.6, 0.9), rng.randf_range(0.35, 0.6), rng.randf_range(0.5, 0.8)),
				"stone_dark", 0, 0.15, rng)
	return m.build()


## Mr Numbers' chalkboard on an easel: a dark green board with chalk shapes (triangle, square, circle).
static func _chalkboard(m: LowPoly, p: Vector3, yaw: float) -> void:
	m.push_at(p, yaw)
	for x: float in [-0.65, 0.65]:
		HubArt._limb(m, Vector3(x, 0, -0.25), Vector3(x * 0.85, 1.9, 0.05), 0.04, 0.035, "wood_dark")
	HubArt._limb(m, Vector3(0, 0, -0.6), Vector3(0, 1.7, -0.05), 0.035, 0.03, "wood_dark")
	m.push(Transform3D(Basis(Vector3.RIGHT, -0.12), Vector3(0, 1.25, 0.05)))
	m.box(Vector3.ZERO, Vector3(1.5, 1.05, 0.08), "wood")
	m.box(Vector3(0, 0, 0.03), Vector3(1.32, 0.88, 0.06), "leaf_dark")
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.18, 0.18, 1.0)), Vector3(-0.42, 0.05, 0.07)))
	m.extrude(PackedVector2Array([Vector2(-1, -0.9), Vector2(1, -0.9), Vector2(0, 1)]), 0.02, "flower_white")
	m.pop()
	m.box(Vector3(0.02, 0.05, 0.07), Vector3(0.32, 0.32, 0.02), "flower_white")
	m.box(Vector3(0.02, 0.05, 0.075), Vector3(0.24, 0.24, 0.02), "leaf_dark")
	m.push(Transform3D(Basis.IDENTITY, Vector3(0.44, 0.05, 0.07)))
	m.extrude(ZoneKit.circle(0.17, 12), 0.02, "flower_white")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0.44, 0.05, 0.075)))
	m.extrude(ZoneKit.circle(0.12, 12), 0.02, "leaf_dark")
	m.pop()
	m.pop()
	m.pop()


## Stack of 1, 2 and 3 dice-like blocks with white pips (counting, not text).
static func _counting_blocks(m: LowPoly, p: Vector3, yaw: float) -> void:
	m.push_at(p, yaw)
	var colours: PackedStringArray = ["subject_math", "lantern_glow", "cloth_red"]
	var size: float = 0.62
	for column: int in 3:
		for level: int in column + 1:
			var c: Vector3 = Vector3((column - 1) * (size + 0.08), size * 0.5 + level * size, 0)
			m.box(c, Vector3(size, size, size), colours[column])
			# pips on the front face: column + 1 of them
			for k: int in column + 1:
				var off: float = (float(k) - float(column) * 0.5) * 0.17
				m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), c + Vector3(off, 0.0, size * 0.5 - 0.01)))
				m.cylinder(Vector3.ZERO, 0.06, 0.06, 0.03, 8, "flower_white")
				m.pop()
	m.pop()


## Clipped bush on a square stone pedestal: 0 sphere, 1 cube, 2 cone, 3 pyramid.
static func _topiary(m: LowPoly, p: Vector3, kind: int) -> void:
	m.box(p + Vector3(0, 0.3, 0), Vector3(1.1, 0.6, 1.1), "stone", "tile_blue")
	var c: Vector3 = p + Vector3(0, 0.6, 0)
	match kind:
		0:
			m.blob(c + Vector3(0, 0.75, 0), Vector3(0.78, 0.78, 0.78), "leaf", 1)
		1:
			m.box(c + Vector3(0, 0.62, 0), Vector3(1.15, 1.15, 1.15), "leaf", "leaf_light")
		2:
			m.cylinder(c, 0.7, 0.0, 1.9, 12, "leaf_dark")
		_:
			m.cylinder(c, 0.85, 0.0, 1.6, 4, "leaf", "", false, 0.5)


## Bamboo water wheel (cọn nước) turning in the stream: rim, spokes, bamboo tubes, two posts with an axle.
static func _water_wheel(m: LowPoly, c: Vector3) -> void:
	var x: float = stream_x(c.z)
	var hub: Vector3 = Vector3(x, 1.35, c.z)
	m.push(Transform3D(Basis(Vector3.UP, PI * 0.5), hub))
	HubArt._torus_ring(m, Vector3.ZERO, 1.55, 0.07, 18, 5, "bamboo")
	HubArt._torus_ring(m, Vector3.ZERO, 1.25, 0.05, 18, 5, "wood")
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		var d: Vector3 = Vector3(cos(a), sin(a), 0)
		HubArt._limb(m, d * 0.15, d * 1.55, 0.04, 0.035, "bamboo")
		var tube: Vector3 = d * 1.62
		HubArt._limb(m, tube - Vector3(-d.y, d.x, 0) * 0.25, tube + Vector3(-d.y, d.x, 0) * 0.25, 0.07, 0.07, "bamboo")
	m.pop()
	for s: float in [-1.0, 1.0]:
		var post: Vector3 = Vector3(x + s * 0.55, 0.0, c.z)
		HubArt._limb(m, post + Vector3(s * 0.25, 0, -0.5), post + Vector3(0, 1.45, 0), 0.07, 0.06, "wood_dark")
		HubArt._limb(m, post + Vector3(s * 0.25, 0, 0.5), post + Vector3(0, 1.45, 0), 0.07, 0.06, "wood_dark")
	HubArt._limb(m, hub + Vector3(-0.7, 0, 0), hub + Vector3(0.7, 0, 0), 0.08, 0.08, "wood_dark")
	# trough carrying the water to the field
	HubArt._limb(m, Vector3(x + 0.8, 2.9, c.z), Vector3(x + 4.5, 1.0, c.z - 0.6), 0.09, 0.09, "bamboo")
	for t: float in [0.3, 0.7]:
		var q: Vector3 = Vector3(x + 0.8, 2.9, c.z).lerp(Vector3(x + 4.5, 1.0, c.z - 0.6), t)
		HubArt._limb(m, Vector3(q.x, 0, q.z), q, 0.04, 0.04, "wood_dark")


# ---------- vegetation ----------

## Scatter kinds of the zone → mesh file name (meshes baked by the hub recipes, shared).
const SCATTER: Dictionary = {
	"tree": "veg_tree_round_a", "bush": "veg_bush_a", "flower": "veg_flower_a", "tuft": "veg_tuft_a", "bamboo": "veg_bamboo_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"tree":
			return ZoneKit.scatter(9310, 8, 20, 1.6, BLOCKED, 0.85, 1.25)
		"bush":
			return ZoneKit.scatter(9311, 34, 0, 1.0, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter(9312, 90, 0, 0.3, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter(9313, 240, 0, 0.3, BLOCKED, 0.8, 1.4)
		"bamboo":
			var rng: RandomNumberGenerator = RandomNumberGenerator.new()
			rng.seed = 9314
			var out: Array[Transform3D] = []
			var zs: PackedFloat32Array = [-19.0, -11.0, 3.0, 9.0, 14.0]
			for i: int in zs.size():
				var z: float = zs[i]
				var side: float = -1.0 if i % 2 == 0 else 1.0
				out.append(HubArt._xf(Vector3(stream_x(z) + side * 2.4, 0, z), rng, 0.8, 1.1))
			return out
	return []
