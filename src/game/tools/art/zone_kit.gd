extends RefCounted
## Shared prefab of the subject learning zones (F093; WORLD_DESIGN §4.2 "môn mới dùng prefab vùng chung + cấu hình",
## §11; ADR-0103 "chibi anime cartoon 3D"). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
## Bake-time only (tools/* is not exported): every subject zone uses the same layout of the world seed (F071 —
## spawn south, quest NPC on a round plaza, two discoveries east/west on a cross path, the collectible on the main path
## and the wind gate home in the north) and the same terrain, boundary and scatter rules; the subject recipes
## (zone_<subject>_art.gd) add their landmark, set pieces and colours (a theme dictionary of art palette slots, R32).
## The zone scene owns collision, spawn points and interactables (WORLD_RENDERING §2) — no gameplay here.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")

## Anchor layout shared by the subject zones (positions of the world seed areas.v1.json, F071).
const SPAWN: Vector3 = Vector3(0, 0, 8)
const NPC: Vector3 = Vector3(0, 0, 2)
const EAST: Vector3 = Vector3(12, 0, -6)
const WEST: Vector3 = Vector3(-12, 0, -6)
const NORTH: Vector3 = Vector3(0, 0, -14)
const PORTAL: Vector3 = Vector3(0, 0, -20)
## Round plaza around the NPC and the spawn; the walkable square inside the boundary ring (ground collision ±25).
const PLAZA_CENTER: Vector3 = Vector3(0, 0, 4)
const PLAZA_RADIUS: float = 6.5
const WALK: float = 23.5
const BOUNDARY_THICKNESS: float = 1.0
const BOUNDARY_HEIGHT: float = 1.1
## Landmark of every subject zone: outside the ring behind the wind gate, framing the view from the spawn.
const BACKDROP: Vector3 = Vector3(0, 0, -31)
const MIN_BEVEL: float = 0.15
## Default theme: art palette slot per role (a subject recipe overrides some of them).
const THEME: Dictionary = {
	"lawn": "grass", "lawn_alt": "grass_dark", "paving": "paving", "paving_dark": "paving_dark", "curb": "stone",
	"path": "path", "hedge": "grass_dark", "hedge_top": "leaf", "hill": "hill", "hill_far": "hill_far", "far": "hill",
}


## World builder of the zones: smooth shaded, bevels only on boxes thicker than MIN_BEVEL (PERFORMANCE_BUDGET §7).
static func builder() -> LowPoly:
	var m: LowPoly = LowPoly.new()
	m.min_bevel_size = MIN_BEVEL
	return m


static func theme_with(overrides: Dictionary) -> Dictionary:
	var out: Dictionary = THEME.duplicate()
	out.merge(overrides, true)
	return out


# ---------- terrain ----------

## Near ground of a subject zone (toon + outline): two-tone lawn, round plaza with a curb, the cross of paths and
## paving pads under the discoveries / collectible / wind gate.
static func terrain(m: LowPoly, theme: Dictionary) -> void:
	var cell: float = 4.0
	for ix: int in range(-8, 8):
		for iz: int in range(-8, 8):
			var x0: float = ix * cell
			var z0: float = iz * cell
			var patch: bool = sin((x0 + 2.0) * 0.21 + 0.7) + cos((z0 + 2.0) * 0.17 + 1.9) > 1.0
			m.quad(Vector3(x0, 0, z0), Vector3(x0, 0, z0 + cell), Vector3(x0 + cell, 0, z0 + cell), Vector3(x0 + cell, 0, z0),
					String(theme["lawn_alt"] if patch else theme["lawn"]))
	# Paths: main north-south lane (spawn → collectible → wind gate) and the east-west cross path to the discoveries.
	path_strip(m, Vector3(-2.0, 0, -23.0), Vector3(2.0, 0, 24.0), String(theme["path"]))
	path_strip(m, Vector3(-15.0, 0, -7.8), Vector3(15.0, 0, -4.2), String(theme["path"]))
	# Plaza: curb ring, paving and a darker ring.
	HubArt._ring(m, PLAZA_CENTER, PLAZA_RADIUS, PLAZA_RADIUS + 0.4, 0.1, 32, String(theme["curb"]))
	HubArt._disk(m, PLAZA_CENTER + Vector3.UP * 0.08, PLAZA_RADIUS, 32, String(theme["paving"]))
	HubArt._annulus(m, PLAZA_CENTER + Vector3.UP * 0.085, 4.3, 5.0, 32, String(theme["paving_dark"]))
	for c: Vector3 in [EAST, WEST, NORTH, PORTAL]:
		HubArt._disk(m, c + Vector3.UP * 0.06, 3.0, 20, String(theme["paving"]))
		HubArt._annulus(m, c + Vector3.UP * 0.065, 2.4, 3.0, 20, String(theme["paving_dark"]))


## Far scenery of a subject zone, baked apart and drawn with the no-outline preset without shadows (the outline fades
## out beyond 30 m anyway; PERFORMANCE_BUDGET §7 counts every pass): the far ground disk and, unless `hills` is false
## (an island draws its own sea), a gentle rise behind the ring and a ring of rounded hills.
static func far(m: LowPoly, theme: Dictionary, rng: RandomNumberGenerator, hills: bool = true) -> void:
	HubArt._disk(m, Vector3(0, -0.05, 0), 170.0, 20, String(theme["far"]))
	if hills:
		# Gentle rise behind the ring on every side, then the hills.
		for side: int in 4:
			m.push_at(Vector3.ZERO, 90.0 * side)
			m.quad(Vector3(-32, 0, -32), Vector3(32, 0, -32), Vector3(70, 4.0, -70), Vector3(-70, 4.0, -70), String(theme["lawn_alt"]))
			m.pop()
		for i: int in 16:
			var a: float = TAU * (float(i) + rng.randf_range(-0.3, 0.3)) / 16.0
			var r: float = rng.randf_range(78.0, 105.0)
			var h: float = rng.randf_range(9.0, 20.0)
			var w: float = rng.randf_range(18.0, 28.0)
			# subdivision 2: the cel bands of the big hills follow smooth curves, not facets
			m.blob(Vector3(cos(a) * r, -h * 0.25, sin(a) * r), Vector3(w, h, w * 0.8),
					String(theme["hill"] if i % 2 == 0 else theme["hill_far"]), 2, 0.03, rng)


static func path_strip(m: LowPoly, a: Vector3, b: Vector3, color: String, y: float = 0.045) -> void:
	m.quad(Vector3(a.x, y, a.z), Vector3(a.x, y, b.z), Vector3(b.x, y, b.z), Vector3(b.x, y, a.z), color)


## Clipped boundary ring at ±(WALK + t/2) (collision walls BoundaryNorth/South/East/West in the zone scene).
static func boundary(m: LowPoly, theme: Dictionary) -> void:
	var h: float = BOUNDARY_HEIGHT
	var t: float = BOUNDARY_THICKNESS
	var c: float = WALK + t * 0.5
	var long: float = 2.0 * WALK + 2.0 * t
	for s: float in [-1.0, 1.0]:
		m.box(Vector3(s * c, h * 0.5, 0), Vector3(t, h, long), String(theme["hedge"]), String(theme["hedge_top"]))
		m.box(Vector3(0, h * 0.5, s * c), Vector3(long - 2.0 * t, h, t), String(theme["hedge"]), String(theme["hedge_top"]))


# ---------- shared small props ----------

## Lamp post with a round lantern (`glow` colours the lantern body).
static func lamp(m: LowPoly, p: Vector3, post: String, glow: String) -> void:
	m.cylinder(p, 0.28, 0.24, 0.28, 8, "stone")
	m.cylinder(p + Vector3.UP * 0.28, 0.08, 0.07, 2.5, 8, post)
	m.blob(p + Vector3(0, 3.05, 0), Vector3(0.26, 0.34, 0.26), glow, 1)
	m.cylinder(p + Vector3(0, 2.73, 0), 0.14, 0.14, 0.08, 8, "lantern_glow")
	m.cylinder(p + Vector3(0, 3.33, 0), 0.14, 0.04, 0.16, 8, post)


## Village house with a tiled roof (`roof`/`ridge`), a coloured skirt and flower pots; front door along local +Z.
static func house(m: LowPoly, size: Vector2, roof: String, ridge: String, skirt: String, blossom: String) -> void:
	var w: float = size.x
	var d: float = size.y
	m.box(Vector3(0, 0.12, 0), Vector3(w + 0.6, 0.24, d + 0.6), "stone")
	m.box(Vector3(0, 1.4, 0), Vector3(w, 2.3, d), "wall")
	m.box(Vector3(0, 0.45, 0), Vector3(w + 0.04, 0.4, d + 0.04), skirt)
	m.box(Vector3(0, 1.1, d * 0.5 + 0.01), Vector3(1.0, 1.7, 0.1), "wood_dark")
	for x: float in [-w * 0.32, w * 0.32]:
		m.box(Vector3(x, 1.55, d * 0.5 + 0.01), Vector3(0.8, 0.7, 0.1), "wood")
		m.box(Vector3(x, 1.55, d * 0.5 + 0.03), Vector3(0.62, 0.52, 0.1), "water_deep")
	for x: float in [-w * 0.5 + 0.2, w * 0.5 - 0.2]:
		m.cylinder(Vector3(x, 0.24, d * 0.5 + 0.7), 0.1, 0.09, 2.3, 8, "wood")
	m.tiled_roof(Vector3(0, 2.5, 0.3), w + 0.2, d + 1.4, 1.6, 0.5, 0.25, roof, ridge)
	for x: float in [-0.9, 0.9]:
		m.cylinder(Vector3(x, 0.24, d * 0.5 + 0.45), 0.2, 0.26, 0.35, 8, skirt)
		m.blob(Vector3(x, 0.75, d * 0.5 + 0.45), Vector3(0.3, 0.26, 0.3), "leaf", 0)
		m.blob(Vector3(x + 0.08, 0.9, d * 0.5 + 0.55), Vector3(0.09, 0.09, 0.09), blossom, 0)


## Yaw (degrees) that turns local +Z of a prop at `p` towards `target`.
static func yaw_to(p: Vector3, target: Vector3) -> float:
	return rad_to_deg(atan2(target.x - p.x, target.z - p.z))


## Arch outline (rectangle with a half-circle top): half width `hw`, total height `h`, `segs` arc segments.
static func arch(hw: float, h: float, segs: int) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array([Vector2(-hw, 0), Vector2(hw, 0)])
	var spring: float = h - hw
	for i: int in segs + 1:
		var a: float = PI * float(i) / float(segs)
		pts.append(Vector2(cos(a) * hw, spring + sin(a) * hw))
	return pts


static func circle(r: float, sides: int) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in sides:
		var a: float = TAU * float(i) / float(sides)
		pts.append(Vector2(cos(a), sin(a)) * r)
	return pts


static func bench(m: LowPoly, p: Vector3, yaw: float) -> void:
	m.push_at(p, yaw)
	HubArt._bench(m)
	m.pop()


## Flower bed: low stone rim with a soil top and a few round blossoms.
static func flower_bed(m: LowPoly, c: Vector3, radius: float, flowers: PackedStringArray, rng: RandomNumberGenerator) -> void:
	HubArt._ring(m, c, radius - 0.25, radius, 0.3, 14, "stone")
	HubArt._disk(m, c + Vector3.UP * 0.22, radius - 0.25, 14, "soil")
	for i: int in 7:
		var a: float = TAU * float(i) / 7.0 + rng.randf_range(-0.3, 0.3)
		var r: float = rng.randf_range(0.0, radius - 0.55)
		var p: Vector3 = c + Vector3(cos(a) * r, 0.42, sin(a) * r)
		m.blob(p, Vector3(0.32, 0.24, 0.32), "leaf", 0)
		m.blob(p + Vector3(0, 0.2, 0), Vector3(0.16, 0.12, 0.16), flowers[i % flowers.size()], 0)


# ---------- scatter ----------

## True when `p` keeps `margin` away from the plaza, the paths, the anchor pads and the `blocked` rectangles
## ([centre Vector3, half size Vector2]) of a subject zone's buildings and set pieces.
static func clear(p: Vector3, margin: float, blocked: Array) -> bool:
	if Vector2(p.x - PLAZA_CENTER.x, p.z - PLAZA_CENTER.z).length() < PLAZA_RADIUS + 0.8 + margin:
		return false
	if absf(p.x) < 2.6 + margin and p.z > -24.5 and p.z < 24.5:
		return false
	if absf(p.z + 6.0) < 2.4 + margin and absf(p.x) < 15.5 + margin:
		return false
	for c: Vector3 in [EAST, WEST, NORTH, PORTAL]:
		if Vector2(p.x - c.x, p.z - c.z).length() < 3.6 + margin:
			return false
	for b: Array in blocked:
		var bc: Vector3 = b[0] as Vector3
		var hs: Vector2 = b[1] as Vector2
		if absf(p.x - bc.x) < hs.x + margin and absf(p.z - bc.z) < hs.y + margin:
			return false
	return true


## Seeded placements: `inside` instances inside the ring (clear of the layout), `outside` in the band behind it.
static func scatter(seed_value: int, inside: int, outside: int, margin: float, blocked: Array, s0: float = 0.8,
		s1: float = 1.25, band: Vector2 = Vector2(26.0, 33.0)) -> Array[Transform3D]:
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = seed_value
	var out: Array[Transform3D] = []
	var tries: int = 0
	while out.size() < inside and tries < inside * 20:
		tries += 1
		var p: Vector3 = Vector3(rng.randf_range(-WALK + 0.6, WALK - 0.6), 0, rng.randf_range(-WALK + 0.6, WALK - 0.6))
		if clear(p, margin, blocked):
			out.append(HubArt._xf(p, rng, s0, s1))
	var placed: int = 0
	tries = 0
	while placed < outside and tries < outside * 20:
		tries += 1
		var p: Vector3 = HubArt._outside_ring(rng, band.x, band.y)
		if absf(p.x) < 9.0 and p.z < -24.0:
			continue  # keep the landmark behind the wind gate visible
		var ok: bool = true
		for b: Array in blocked:
			var bc: Vector3 = b[0] as Vector3
			var hs: Vector2 = b[1] as Vector2
			if absf(p.x - bc.x) < hs.x + margin and absf(p.z - bc.z) < hs.y + margin:
				ok = false
		if ok:
			out.append(HubArt._xf(p, rng, s0, s1))
			placed += 1
	return out
