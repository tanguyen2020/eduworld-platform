extends RefCounted
## Shared prefab of the zones (F093 subject zones; F094 zones with their own layout; WORLD_DESIGN §4.2 "môn mới
## dùng prefab vùng chung + cấu hình", §11; ADR-0103 "chibi anime cartoon 3D"). SPDX-License-Identifier: MIT — EduWorld first-party, nothing downloaded.
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
	lawn(m, theme)
	# Paths: main north-south lane (spawn → collectible → wind gate) and the east-west cross path to the discoveries.
	path_strip(m, Vector3(-2.0, 0, -23.0), Vector3(2.0, 0, 24.0), String(theme["path"]))
	path_strip(m, Vector3(-15.0, 0, -7.8), Vector3(15.0, 0, -4.2), String(theme["path"]))
	plaza(m, theme, PLAZA_CENTER, PLAZA_RADIUS)
	for c: Vector3 in [EAST, WEST, NORTH, PORTAL]:
		pad(m, theme, c)


## Two-tone lawn of the walkable square (4 m cells, ±32 m), colours `lawn` / `lawn_alt` of the theme.
static func lawn(m: LowPoly, theme: Dictionary) -> void:
	var cell: float = 4.0
	for ix: int in range(-8, 8):
		for iz: int in range(-8, 8):
			var x0: float = ix * cell
			var z0: float = iz * cell
			var patch: bool = sin((x0 + 2.0) * 0.21 + 0.7) + cos((z0 + 2.0) * 0.17 + 1.9) > 1.0
			m.quad(Vector3(x0, 0, z0), Vector3(x0, 0, z0 + cell), Vector3(x0 + cell, 0, z0 + cell), Vector3(x0 + cell, 0, z0),
					String(theme["lawn_alt"] if patch else theme["lawn"]))


## Round plaza: curb ring, paving and a darker ring 1.5–2.2 m inside the edge.
static func plaza(m: LowPoly, theme: Dictionary, c: Vector3, r: float) -> void:
	HubArt._ring(m, c, r, r + 0.4, 0.1, 32, String(theme["curb"]))
	HubArt._disk(m, c + Vector3.UP * 0.08, r, 32, String(theme["paving"]))
	HubArt._annulus(m, c + Vector3.UP * 0.085, r - 2.2, r - 1.5, 32, String(theme["paving_dark"]))


## Paving pad under an anchor (radius `r`, darker rim).
static func pad(m: LowPoly, theme: Dictionary, c: Vector3, r: float = 3.0) -> void:
	HubArt._disk(m, c + Vector3.UP * 0.06, r, 20, String(theme["paving"]))
	HubArt._annulus(m, c + Vector3.UP * 0.065, r - 0.6, r, 20, String(theme["paving_dark"]))


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


# ---------- zones with their own anchor layout (F094) ----------
## A layout is a Dictionary: "plaza" = [centre Vector3, radius] (optional), "lanes" = [[centre Vector3, half size
## Vector2]] (axis-aligned walking strips), "pads" = [[centre Vector3, radius]] (anchor pads, set piece clearings).

## True when `p` keeps `margin` away from the plaza, the lanes, the pads of `layout` and the `blocked` rectangles.
static func clear_of(p: Vector3, margin: float, layout: Dictionary, blocked: Array) -> bool:
	if layout.has("plaza"):
		var pl: Array = layout["plaza"] as Array
		var pc: Vector3 = pl[0] as Vector3
		if Vector2(p.x - pc.x, p.z - pc.z).length() < float(pl[1]) + 0.8 + margin:
			return false
	for lane: Array in layout.get("lanes", []) as Array:
		var lc: Vector3 = lane[0] as Vector3
		var lh: Vector2 = lane[1] as Vector2
		if absf(p.x - lc.x) < lh.x + 0.6 + margin and absf(p.z - lc.z) < lh.y + 0.6 + margin:
			return false
	for pd: Array in layout.get("pads", []) as Array:
		var c: Vector3 = pd[0] as Vector3
		if Vector2(p.x - c.x, p.z - c.z).length() < float(pd[1]) + 0.6 + margin:
			return false
	for b: Array in blocked:
		var bc: Vector3 = b[0] as Vector3
		var hs: Vector2 = b[1] as Vector2
		if absf(p.x - bc.x) < hs.x + margin and absf(p.z - bc.z) < hs.y + margin:
			return false
	return true


## Seeded placements of a zone with its own layout: `inside` instances inside the ring clear of the layout, `outside`
## in the band behind the ring (the landmark behind the wind gate stays visible).
static func scatter_in(seed_value: int, inside: int, outside: int, margin: float, layout: Dictionary, blocked: Array,
		s0: float = 0.8, s1: float = 1.25, band: Vector2 = Vector2(26.0, 33.0)) -> Array[Transform3D]:
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = seed_value
	var out: Array[Transform3D] = []
	var tries: int = 0
	while out.size() < inside and tries < inside * 30:
		tries += 1
		var p: Vector3 = Vector3(rng.randf_range(-WALK + 0.6, WALK - 0.6), 0, rng.randf_range(-WALK + 0.6, WALK - 0.6))
		if clear_of(p, margin, layout, blocked):
			out.append(HubArt._xf(p, rng, s0, s1))
	var placed: int = 0
	tries = 0
	while placed < outside and tries < outside * 30:
		tries += 1
		var p: Vector3 = HubArt._outside_ring(rng, band.x, band.y)
		if absf(p.x) < 10.0 and p.z < -24.0:
			continue  # keep the landmark behind the wind gate visible
		if clear_of(p, margin, {}, blocked):
			out.append(HubArt._xf(p, rng, s0, s1))
			placed += 1
	return out


## Lanes of a layout as ground strips (`colour`), drawn just above the lawn.
static func lanes(m: LowPoly, layout: Dictionary, colour: String) -> void:
	for lane: Array in layout.get("lanes", []) as Array:
		var lc: Vector3 = lane[0] as Vector3
		var lh: Vector2 = lane[1] as Vector2
		path_strip(m, Vector3(lc.x - lh.x, 0, lc.z - lh.y), Vector3(lc.x + lh.x, 0, lc.z + lh.y), colour)


## Closed cone along +Y (bottom cap facing down): pine tiers, roof caps, crystal tips without open backs (F093-3).
static func cone(m: LowPoly, base: Vector3, r: float, h: float, sides: int, colour: String) -> void:
	m.cylinder(base, r, 0.0, h, sides, colour)
	for i: int in sides:
		var a0: float = TAU * float(i) / float(sides)
		var a1: float = TAU * float(i + 1) / float(sides)
		m.tri(base, base + Vector3(cos(a0), 0, sin(a0)) * r, base + Vector3(cos(a1), 0, sin(a1)) * r, colour)


## Double cone (crystal): a closed bipyramid of `sides` faces along +Y from `base`, widest at `mid` of its height.
static func crystal(m: LowPoly, base: Vector3, r: float, h: float, sides: int, colour: String, mid: float = 0.35) -> void:
	var waist: Vector3 = base + Vector3.UP * h * mid
	var top: Vector3 = base + Vector3.UP * h
	for i: int in sides:
		var a0: float = TAU * float(i) / float(sides)
		var a1: float = TAU * float(i + 1) / float(sides)
		var p0: Vector3 = waist + Vector3(cos(a0), 0, sin(a0)) * r
		var p1: Vector3 = waist + Vector3(cos(a1), 0, sin(a1)) * r
		m.tri(p0, top, p1, colour)
		m.tri(p1, base, p0, colour)


## Swallow-tail pennant (cờ đuôi nheo) on a pole: stone foot, pole, a ball finial and a closed cloth flag along +X.
static func pennant(m: LowPoly, p: Vector3, colour: String, height: float = 3.4, yaw: float = 0.0) -> void:
	m.cylinder(p, 0.2, 0.16, 0.22, 8, "stone")
	m.cylinder(p + Vector3.UP * 0.22, 0.05, 0.045, height, 8, "wood_dark")
	m.blob(p + Vector3.UP * (height + 0.3), Vector3(0.09, 0.09, 0.09), "lantern_glow", 0)
	m.push_at(p + Vector3(0, height + 0.1, 0), yaw)
	m.extrude(PackedVector2Array([Vector2(0.04, 0.0), Vector2(1.2, -0.12), Vector2(0.9, -0.35), Vector2(1.2, -0.6),
		Vector2(0.04, -0.7)]), 0.05, colour)
	m.pop()


## String of triangle flags (dây cờ) sagging between `a` and `b`; flag colours cycle through `colours`.
static func bunting(m: LowPoly, a: Vector3, b: Vector3, colours: PackedStringArray, flags: int = 8, sag: float = 0.5) -> void:
	var prev: Vector3 = a
	var segs: int = flags + 1
	var yaw: float = rad_to_deg(atan2(b.x - a.x, b.z - a.z)) - 90.0
	for i: int in segs:
		var t: float = float(i + 1) / float(segs)
		var q: Vector3 = a.lerp(b, t) + Vector3.DOWN * sag * 4.0 * t * (1.0 - t)
		HubArt._limb(m, prev, q, 0.02, 0.02, "wood_dark")
		if i < flags:
			m.push_at(q, yaw)
			m.extrude(PackedVector2Array([Vector2(-0.22, -0.02), Vector2(0.22, -0.02), Vector2(0.0, -0.5)]), 0.03,
					colours[i % colours.size()])
			m.pop()
		prev = q


## Cluster of balloons on strings tied to a small stone weight (festive zones, F094).
static func balloons(m: LowPoly, p: Vector3, colours: PackedStringArray, rng: RandomNumberGenerator) -> void:
	m.cylinder(p, 0.18, 0.14, 0.2, 8, "stone")
	for i: int in colours.size():
		var a: float = TAU * float(i) / float(colours.size()) + rng.randf_range(-0.3, 0.3)
		var top: Vector3 = p + Vector3(cos(a) * 0.45, rng.randf_range(2.4, 3.0), sin(a) * 0.45)
		HubArt._limb(m, p + Vector3.UP * 0.2, top, 0.012, 0.012, "flower_white")
		m.blob(top + Vector3.UP * 0.34, Vector3(0.3, 0.38, 0.3), colours[i], 0)
		m.cylinder(top, 0.05, 0.08, 0.06, 6, colours[i])


## Rounded boulder.
static func rock(m: LowPoly, p: Vector3, size: Vector3, colour: String, rng: RandomNumberGenerator) -> void:
	m.blob(p + Vector3.UP * size.y * 0.4, size, colour, 0, 0.15, rng)


## Cairn (ụ đá xếp): flattened stones stacked smaller to the top.
static func cairn(m: LowPoly, p: Vector3, rng: RandomNumberGenerator, levels: int = 4) -> void:
	var y: float = 0.0
	for k: int in levels:
		var r: float = 0.55 - 0.1 * k
		var h: float = 0.22 - 0.025 * k
		m.blob(p + Vector3(rng.randf_range(-0.05, 0.05), y + h, rng.randf_range(-0.05, 0.05)), Vector3(r, h, r * 0.9),
				"stone" if k % 2 == 0 else "stone_dark", 0, 0.08, rng)
		y += h * 1.8


## Pine tree (cây thông) of the mountain zones: trunk and three closed cone tiers (F094, scattered as MultiMesh).
static func pine() -> ArrayMesh:
	var m: LowPoly = builder()
	m.blob_detail = 0
	m.min_sides = 7
	m.cylinder(Vector3.ZERO, 0.2, 0.15, 1.2, 7, "trunk")
	cone(m, Vector3(0, 0.9, 0), 1.45, 1.9, 9, "leaf_dark")
	cone(m, Vector3(0, 2.0, 0), 1.15, 1.7, 9, "leaf_dark")
	cone(m, Vector3(0, 3.0, 0), 0.8, 1.5, 9, "leaf")
	return m.build()


# ---------- way-finding (F094: "dễ dùng" — a student always sees where to go) ----------

## Icon outline of a way-finding kind (about 2 × 2 units): the shape tells the destination without relying on colour
## (WORLD_DESIGN §11 accessibility). "home" is the wind gate back to the town (house, as on the gate sign).
static func icon(kind: String) -> PackedVector2Array:
	match kind:
		"home":
			return HubArt._icon_points("main_hub")
		"star":
			return HubArt._star(1.0, 0.45)
		"book":
			return HubArt._icon_points("learning_zone.vietnamese")
		"heart":
			return HubArt._icon_points("social_zone")
		"wave":
			return HubArt._icon_points("quest_zone.discovery_ocean")
		"peak":
			return HubArt._icon_points("challenge_mountain")
		"tower":
			return HubArt._icon_points("exam_zone")
		"leaf":
			return HubArt._icon_points("practice_zone")
		"talk":
			return HubArt._bubble()
	return HubArt._star(0.8, 0.8)


## Wooden signpost: one arrow board per destination ([target Vector3, icon kind, icon colour]), each board pointing at
## its target with the icon on both faces; a little roof cap on the post.
static func signpost(m: LowPoly, p: Vector3, arrows: Array) -> void:
	m.cylinder(p, 0.26, 0.22, 0.2, 8, "stone")
	m.cylinder(p, 0.11, 0.09, 1.4 + 0.42 * arrows.size(), 8, "wood_dark")
	var top: float = 1.4 + 0.42 * arrows.size()
	cone(m, p + Vector3.UP * top, 0.22, 0.3, 8, "roof_dark")
	for i: int in arrows.size():
		var a: Array = arrows[i] as Array
		var t: Vector3 = a[0] as Vector3
		var yaw: float = rad_to_deg(atan2(-(t.x - p.x), -(t.z - p.z)))
		m.push_at(p + Vector3.UP * (top - 0.3 - 0.42 * i), yaw)
		m.push(Transform3D(Basis(Vector3.UP, PI * 0.5), Vector3.ZERO))
		m.extrude(PackedVector2Array([Vector2(0.1, -0.17), Vector2(1.05, -0.17), Vector2(1.3, 0), Vector2(1.05, 0.17),
			Vector2(0.1, 0.17)]), 0.06, "wood", "wood_dark")
		for side: float in [1.0, -1.0]:
			m.push(Transform3D(Basis(Vector3.UP, 0.0 if side > 0.0 else PI).scaled(Vector3(0.13, 0.13, 1)),
					Vector3(0.62, 0, 0.045 * side)))
			m.extrude(icon(String(a[1])), 0.03, String(a[2]))
			m.pop()
		m.pop()
		m.pop()


## Warm glowing ring painted round an anchor pad (the interactable reads as "come here", DESIGN_SYSTEM §5.3 spark).
static func glow_ring(m: LowPoly, c: Vector3, r: float) -> void:
	HubArt._annulus(m, c + Vector3.UP * 0.095, r - 0.95, r - 0.7, 24, "spark")
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		HubArt._disk(m, c + Vector3(cos(a), 0, sin(a)) * (r - 0.33) + Vector3.UP * 0.097, 0.13, 6, "lantern_glow")
