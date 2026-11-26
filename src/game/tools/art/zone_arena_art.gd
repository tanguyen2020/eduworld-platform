extends RefCounted
## Recipes of "Đấu trường Nhà Vô Địch" (Champion Arena, `arena`; F094, WORLD_DESIGN §2.3 Trọng tài Sấm, Bà Lịch, §4.5
## "bảng xếp hạng 3D (chỉ nickname), xếp hàng 1v1/Team Battle/Timed Quiz/Co-op Challenge, đăng ký tournament",
## §2.1 không vũ khí; COMPETITION_SYSTEM; ADR-0103). SPDX-License-Identifier: MIT — EduWorld first-party, nothing
## downloaded.
## A festive, friendly sports-day plaza (F094-AC-03: many bright colours, no weapons, no fighting imagery): the round
## Champion Stadium with striped awnings, flags and a big star over its gate behind the wind gate; four queue arches
## with shape icons (1v1 two dots, team three dots, timed a clock, co-op a heart) beside the referee; the tournament
## calendar board beside Madam Calendar; the podium 1-2-3 with star cups in front of a board of blank name plates and
## coloured bars (no names, R2); a golden cup statue; festival stalls, balloons, bunting and coloured lanterns.
## Colours are art palette slots only (R32); collision lives in the zone scene.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const HubArt := preload("res://tools/art/hub_art.gd")
const ZoneKit := preload("res://tools/art/zone_kit.gd")

const SLUG: String = "arena"
const THEME_OVERRIDES: Dictionary = {"paving_dark": "lantern_glow", "path": "paving", "hedge": "leaf_dark", "hedge_top": "leaf"}
const FESTIVE: PackedStringArray = ["cloth_red", "lantern_glow", "cloth_blue", "cloth_green", "cloth_violet", "flower_pink"]
const PLAZA: Vector3 = Vector3(0, 0, 3)
const PLAZA_RADIUS: float = 8.0
const GATES: Vector3 = Vector3(12.0, 0, 2.5)
const CALENDAR: Vector3 = Vector3(-9.0, 0, 1.0)
const PODIUM: Vector3 = Vector3(-12.5, 0, -11.0)
const TROPHY: Vector3 = Vector3(12.5, 0, -11.0)
const STALLS: Array = [[Vector3(-15.0, 0, 15.5), 150.0], [Vector3(15.0, 0, 15.5), -150.0]]
const LAYOUT: Dictionary = {
	"plaza": [PLAZA, PLAZA_RADIUS],
	"lanes": [[Vector3(0, 0, 0.5), Vector2(2.0, 23.5)], [Vector3(0, 0, -9.0), Vector2(10.0, 1.5)]],
	"pads": [[GATES + Vector3(-1.2, 0, 0), 4.6], [PODIUM, 4.0], [TROPHY, 3.4], [Vector3(0, 0, -20), 3.0]],
}
const BLOCKED: Array = [
	[GATES, Vector2(2.0, 5.6)],
	[CALENDAR, Vector2(1.4, 2.2)],
	[PODIUM, Vector2(4.2, 3.6)],
	[TROPHY, Vector2(2.6, 2.6)],
	[Vector3(-15.0, 0, 15.5), Vector2(2.8, 2.8)],
	[Vector3(15.0, 0, 15.5), Vector2(2.8, 2.8)],
	[Vector3(3.2, 0, 12.0), Vector2(1.2, 1.2)],
	[Vector3(0, 0, -31), Vector2(13.0, 7.0)],
]


static func theme() -> Dictionary:
	return ZoneKit.theme_with(THEME_OVERRIDES)


## Baked meshes of the zone: path under res://assets/models (without ".res") → recipe.
static func meshes() -> Dictionary:
	return {
		"env/env_arena_terrain_a": terrain, "env/env_arena_far_a": far, "bld/bld_arena_set_a": buildings,
		"prop/prop_arena_set_a": props, "prop/prop_arena_match_gates_a": match_gates,
		"prop/prop_arena_tournament_board_a": tournament_board, "prop/prop_arena_leaderboard_a": leaderboard,
		"prop/prop_arena_trophy_a": trophy_statue,
	}


# ---------- terrain ----------

static func terrain() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var th: Dictionary = theme()
	ZoneKit.lawn(m, th)
	ZoneKit.lanes(m, LAYOUT, "paving")
	ZoneKit.plaza(m, th, PLAZA, PLAZA_RADIUS)
	# festive mosaic: coloured wedges round a big star
	for k: int in 12:
		var a0: float = TAU * float(k) / 12.0
		var a1: float = TAU * float(k + 1) / 12.0
		var p0: Vector3 = PLAZA + Vector3(cos(a0), 0, sin(a0)) * 3.0 + Vector3.UP * 0.088
		var p1: Vector3 = PLAZA + Vector3(cos(a1), 0, sin(a1)) * 3.0 + Vector3.UP * 0.088
		var q0: Vector3 = PLAZA + Vector3(cos(a0), 0, sin(a0)) * 4.4 + Vector3.UP * 0.088
		var q1: Vector3 = PLAZA + Vector3(cos(a1), 0, sin(a1)) * 4.4 + Vector3.UP * 0.088
		m.quad(p0, p1, q1, q0, FESTIVE[k % FESTIVE.size()])
	m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5).scaled(Vector3(2.4, 2.4, 1.0)), PLAZA + Vector3(0, 0.09, 0)))
	m.extrude(HubArt._star(1.0, 0.45), 0.01, "lantern_glow")
	m.pop()
	for pd: Array in [[GATES + Vector3(-1.2, 0, 0), 4.6], [PODIUM, 4.0], [TROPHY, 3.4], [Vector3(0, 0, -20), 3.0]]:
		ZoneKit.pad(m, th, pd[0] as Vector3, float(pd[1]))
	ZoneKit.glow_ring(m, Vector3(0, 0, -20), 3.0)
	ZoneKit.glow_ring(m, Vector3(3, 0, 2), 1.9)
	ZoneKit.glow_ring(m, Vector3(-3, 0, 2), 1.9)
	ZoneKit.boundary(m, th)
	return m.build()


## Far scenery (no outline, no shadow): far ground, rise and hills.
static func far() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9501
	ZoneKit.far(m, theme(), rng)
	return m.build()


# ---------- buildings ----------

static func buildings() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	_stadium(m)
	return m.build()


## Champion Stadium: a round bowl of coloured panels on a stone base, a rim of pennants, striped awnings over the gate,
## an arched entrance and a big golden star with rays over it.
static func _stadium(m: LowPoly) -> void:
	m.push_at(ZoneKit.BACKDROP + Vector3(0, 0, -3.0))
	var r: float = 9.0
	m.cylinder(Vector3.ZERO, r + 0.6, r + 0.5, 0.6, 32, "stone", "stone")
	# the field inside (seen from the hills and the overview): grass with a white ring and a centre star
	HubArt._disk(m, Vector3.UP * 0.62, r - 1.0, 24, "grass")
	HubArt._annulus(m, Vector3.UP * 0.63, 3.2, 3.5, 24, "flower_white")
	var panels: int = 24
	for k: int in panels:
		var a0: float = TAU * float(k) / float(panels)
		var a1: float = TAU * float(k + 1) / float(panels)
		var d0: Vector3 = Vector3(cos(a0), 0, sin(a0))
		var d1: Vector3 = Vector3(cos(a1), 0, sin(a1))
		var mid: Vector3 = (d0 + d1).normalized()
		var col: String = "wall" if k % 2 == 0 else FESTIVE[(k >> 1) % FESTIVE.size()]
		m.quad_out(d0 * r + Vector3.UP * 0.6, d0 * (r - 0.4) + Vector3.UP * 5.0, d1 * (r - 0.4) + Vector3.UP * 5.0, d1 * r + Vector3.UP * 0.6, col,
				Vector3.UP * 2.5)
		m.quad_out(d1 * (r - 0.4) + Vector3.UP * 5.0, d0 * (r - 0.4) + Vector3.UP * 5.0, d0 * (r - 1.0) + Vector3.UP * 5.0,
				d1 * (r - 1.0) + Vector3.UP * 5.0, "stone", mid * (r - 0.7) + Vector3.UP * 2.0)
		m.quad_out(d1 * (r - 1.0) + Vector3.UP * 5.0, d0 * (r - 1.0) + Vector3.UP * 5.0, d0 * (r - 1.0) + Vector3.UP * 0.6,
				d1 * (r - 1.0) + Vector3.UP * 0.6, "wall", mid * (r + 4.0) + Vector3.UP * 2.5)
		if k % 3 == 0:
			ZoneKit.pennant(m, d0 * (r - 0.7) + Vector3.UP * 5.0, FESTIVE[int(k / 3.0) % FESTIVE.size()], 2.4, rad_to_deg(-a0))
	# entrance on the front (+Z): arch, striped awnings and the star
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.6, r - 0.1)))
	m.extrude(ZoneKit.arch(2.4, 4.4, 12), 0.9, "lantern")
	m.pop()
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.6, r + 0.3)))
	m.extrude(ZoneKit.arch(1.9, 3.8, 12), 0.2, "eye")
	m.pop()
	for s: float in [-1.0, 1.0]:
		var c: Vector3 = Vector3(s * 4.6, 2.8, r + 0.8)
		for k: int in 4:
			var x0: float = -1.3 + k * 0.65
			m.push(Transform3D(Basis(Vector3.RIGHT, 0.45), c + Vector3(x0 + 0.325, 0, 0)))
			m.box(Vector3.ZERO, Vector3(0.62, 0.08, 1.6), "cloth_red" if k % 2 == 0 else "flower_white")
			m.pop()
		m.box(c + Vector3(0, -1.4, 0.6), Vector3(0.1, 2.6, 0.1), "wood_dark")
	var star: Vector3 = Vector3(0, 7.0, r + 0.2)
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(1.6, 1.6, 1.0)), star))
	m.extrude(HubArt._star(1.0, 0.45), 0.4, "lantern_glow", "wall_yellow")
	m.pop()
	for k: int in 8:
		var a: float = TAU * float(k) / 8.0
		m.push(Transform3D(Basis(Vector3.BACK, a), star + Vector3(0, 0, -0.1)))
		m.box(Vector3(0, 2.3, 0), Vector3(0.14, 0.6, 0.1), "spark")
		m.pop()
	m.pop()


# ---------- set pieces (local coordinates; front along local +Z) ----------

## Four queue arches in a row along local X (1v1 red with two dots, team blue with three dots, timed yellow with a clock,
## co-op green with a heart) with a mat of arrows in front of each.
static func match_gates() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var colours: PackedStringArray = ["cloth_red", "cloth_blue", "lantern_glow", "cloth_green"]
	for i: int in 4:
		var x: float = -4.5 + i * 3.0
		var col: String = colours[i]
		for s: float in [-1.0, 1.0]:
			m.cylinder(Vector3(x + s * 1.0, 0, 0), 0.2, 0.18, 2.8, 8, col, "flower_white")
			m.blob(Vector3(x + s * 1.0, 2.9, 0), Vector3(0.24, 0.24, 0.24), "flower_white", 1)
		m.push(Transform3D(Basis.IDENTITY, Vector3(x, 2.4, 0)))
		HubArt._torus_ring(m, Vector3.ZERO, 1.0, 0.13, 14, 5, col)
		m.pop()
		# icon plate on top, icon on both faces
		m.push(Transform3D(Basis(Vector3.RIGHT, PI * 0.5), Vector3(x, 3.75, 0)))
		m.cylinder(Vector3(0, -0.08, 0), 0.62, 0.62, 0.16, 14, "flower_white")
		m.pop()
		for side: float in [1.0, -1.0]:
			_gate_icon(m, i, Vector3(x, 3.75, 0.09 * side), col)
		# mat with arrows (pointing in, along -Z)
		m.box(Vector3(x, 0.03, 1.6), Vector3(1.8, 0.04, 1.8), "flower_white")
		for k: int in 2:
			m.push(Transform3D(Basis(Vector3.RIGHT, -PI * 0.5), Vector3(x, 0.06, 2.1 - k * 0.8)))
			m.extrude(PackedVector2Array([Vector2(-0.5, -0.15), Vector2(0, 0.25), Vector2(0.5, -0.15), Vector2(0.5, -0.4), Vector2(0, 0.0),
				Vector2(-0.5, -0.4)]), 0.02, col)
			m.pop()
	return m.build()


## Icon of a queue arch: 0 = 1v1 (two dots), 1 = team (three dots), 2 = timed (clock), 3 = co-op (heart).
static func _gate_icon(m: LowPoly, kind: int, c: Vector3, col: String) -> void:
	match kind:
		0, 1:
			var n: int = 2 if kind == 0 else 3
			for k: int in n:
				var x: float = (float(k) - float(n - 1) * 0.5) * 0.36
				m.push(Transform3D(Basis.IDENTITY, c + Vector3(x, 0, 0)))
				m.extrude(ZoneKit.circle(0.15, 10), 0.04, col)
				m.pop()
		2:
			m.push(Transform3D(Basis.IDENTITY, c))
			HubArt._torus_ring(m, Vector3.ZERO, 0.38, 0.05, 16, 4, col)
			m.box(Vector3(0, 0.12, 0), Vector3(0.06, 0.26, 0.04), "eye")
			m.box(Vector3(0.1, 0, 0), Vector3(0.22, 0.06, 0.04), "eye")
			m.pop()
		3:
			m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.38, 0.38, 1.0)), c))
			m.extrude(HubArt._icon_points("social_zone"), 0.04, col)
			m.pop()


## Tournament calendar board: a framed board with a grid of coloured day squares, a bracket of matches and star badges,
## under a small striped awning, with bunting on top.
static func tournament_board() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	m.min_bevel_size = 0.03  # a board seen up close: round the thin frame and day squares too
	for x: float in [-1.9, 1.9]:
		m.box(Vector3(x, 1.5, 0), Vector3(0.24, 3.0, 0.24), "wood_dark")
	m.box(Vector3(0, 1.7, 0), Vector3(3.8, 2.2, 0.14), "wood")
	m.box(Vector3(0, 1.7, 0.05), Vector3(3.6, 2.0, 0.08), "flower_white")
	for i: int in 5:
		for j: int in 4:
			var col: String = FESTIVE[(i * 3 + j) % FESTIVE.size()] if (i + j * 2) % 3 == 0 else "wall"
			m.box(Vector3(-1.55 + i * 0.36, 2.35 - j * 0.38, 0.1), Vector3(0.3, 0.3, 0.05), col)
	# bracket of matches on the right half
	for k: int in 4:
		m.box(Vector3(0.6, 2.4 - k * 0.42, 0.1), Vector3(0.5, 0.06, 0.02), "cloth_blue")
	for k: int in 2:
		m.box(Vector3(0.88, 2.19 - k * 0.84, 0.1), Vector3(0.06, 0.48, 0.02), "cloth_blue")
		m.box(Vector3(1.14, 2.19 - k * 0.84, 0.1), Vector3(0.5, 0.06, 0.02), "cloth_violet")
	m.box(Vector3(1.4, 1.77, 0.1), Vector3(0.06, 0.9, 0.02), "cloth_violet")
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.2, 0.2, 1.0)), Vector3(1.62, 1.77, 0.12)))
	m.extrude(HubArt._star(1.0, 0.45), 0.04, "lantern_glow")
	m.pop()
	# striped awning
	for k: int in 6:
		m.push(Transform3D(Basis(Vector3.RIGHT, 0.5), Vector3(-1.75 + k * 0.7, 3.0, 0.35)))
		m.box(Vector3.ZERO, Vector3(0.67, 0.06, 0.9), "cloth_green" if k % 2 == 0 else "flower_white")
		m.pop()
	ZoneKit.bunting(m, Vector3(-1.9, 3.4, -0.05), Vector3(1.9, 3.4, -0.05), FESTIVE, 6, 0.25)
	return m.build()


## Podium 1-2-3 with star cups, in front of a board of blank name plates with coloured bars (nickname only in the game
## data; no names in the scene, R2).
static func leaderboard() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var steps: Array = [[0.0, 1.2, "lantern_glow"], [-1.5, 0.85, "metal"], [1.5, 0.6, "bronze"]]
	for s: Array in steps:
		var x: float = float(s[0])
		var h: float = float(s[1])
		m.box(Vector3(x, h * 0.5, 0.6), Vector3(1.5, h, 1.4), "wall", String(s[2]))
		m.box(Vector3(x, h * 0.5, 1.31), Vector3(0.6, 0.5, 0.04), String(s[2]))
		_cup(m, Vector3(x, h, 0.6), 0.55 + 0.15 * h, String(s[2]))
	# board with name plates and bars
	m.box(Vector3(0, 2.6, -1.0), Vector3(5.4, 3.6, 0.2), "cloth_blue")
	m.box(Vector3(0, 2.6, -0.88), Vector3(5.1, 3.3, 0.06), "flower_white")
	for k: int in 5:
		var y: float = 3.9 - k * 0.62
		m.box(Vector3(-1.4, y, -0.84), Vector3(1.6, 0.4, 0.04), "wall")
		var w: float = 2.6 - 0.42 * k
		m.box(Vector3(-0.45 + w * 0.5, y, -0.84), Vector3(w, 0.32, 0.05), FESTIVE[k % FESTIVE.size()])
		m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.14, 0.14, 1.0)), Vector3(-2.35, y, -0.82)))
		m.extrude(HubArt._star(1.0, 0.45), 0.03, "lantern_glow" if k == 0 else "stone")
		m.pop()
	for x: float in [-2.8, 2.8]:
		m.box(Vector3(x, 2.4, -1.0), Vector3(0.3, 4.8, 0.3), "wood_dark")
	ZoneKit.bunting(m, Vector3(-2.8, 4.7, -0.9), Vector3(2.8, 4.7, -0.9), FESTIVE, 7, 0.3)
	return m.build()


## Star cup (cúp): a lathe cup with two handles and a star on top.
static func _cup(m: LowPoly, p: Vector3, s: float, col: String) -> void:
	m.cylinder(p, 0.22 * s, 0.18 * s, 0.1 * s, 10, "wood_dark")
	m.lathe(p + Vector3.UP * 0.1 * s, PackedVector2Array([Vector2(0.06 * s, 0.0), Vector2(0.06 * s, 0.25 * s), Vector2(0.3 * s, 0.4 * s),
		Vector2(0.36 * s, 0.75 * s), Vector2(0.3 * s, 0.78 * s)]), 12, col)
	HubArt._disk(m, p + Vector3.UP * 0.86 * s, 0.31 * s, 12, col)
	for side: float in [-1.0, 1.0]:
		m.push(Transform3D(Basis.IDENTITY, p + Vector3(side * 0.36 * s, 0.6 * s, 0)))
		HubArt._torus_ring(m, Vector3.ZERO, 0.12 * s, 0.03 * s, 10, 4, col)
		m.pop()
	m.push(Transform3D(Basis.IDENTITY.scaled(Vector3(0.18 * s, 0.18 * s, 1.0)), p + Vector3.UP * 1.05 * s))
	m.extrude(HubArt._star(1.0, 0.45), 0.05 * s, "spark")
	m.pop()


## Golden cup statue: stepped stone pedestal, a big star cup and flower beds round it.
static func trophy_statue() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9505
	m.cylinder(Vector3.ZERO, 2.0, 1.9, 0.3, 20, "stone", "stone")
	m.cylinder(Vector3(0, 0.3, 0), 1.3, 1.2, 0.4, 16, "stone_dark", "lantern_glow")
	m.box(Vector3(0, 1.3, 0), Vector3(1.2, 1.6, 1.2), "wall", "stone")
	m.box(Vector3(0, 1.3, 0.61), Vector3(0.7, 0.7, 0.04), "cloth_red")
	_cup(m, Vector3(0, 2.1, 0), 2.2, "lantern_glow")
	for k: int in 6:
		var a: float = TAU * float(k) / 6.0
		var p: Vector3 = Vector3(cos(a), 0, sin(a)) * 1.6 + Vector3.UP * 0.32
		m.blob(p, Vector3(0.3, 0.2, 0.3), "leaf", 0)
		m.blob(p + Vector3.UP * 0.16, Vector3(0.14, 0.1, 0.14), FESTIVE[k % FESTIVE.size()], 0)
	return m.build()


# ---------- props ----------

static func props() -> ArrayMesh:
	var m: LowPoly = ZoneKit.builder()
	m.blob_detail = 0
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 9503
	var angles: Array = [20.0, 70.0, 110.0, 160.0, 200.0, 250.0, 290.0, 340.0]
	var tops: Array[Vector3] = []
	for i: int in angles.size():
		var a: float = deg_to_rad(float(angles[i]))
		var p: Vector3 = PLAZA + Vector3(cos(a), 0, sin(a)) * (PLAZA_RADIUS - 0.6)
		if absf(p.x) < 2.4:
			continue
		ZoneKit.lamp(m, p + Vector3.UP * 0.08, "wood_dark", FESTIVE[i % FESTIVE.size()])
		tops.append(p + Vector3.UP * 3.3)
	# bunting over the lane and between the lamps
	for z: float in [-6.0, -15.0]:
		for s: float in [-1.0, 1.0]:
			m.cylinder(Vector3(s * 3.0, 0, z), 0.08, 0.07, 3.4, 8, "wood")
		ZoneKit.bunting(m, Vector3(-3.0, 3.35, z), Vector3(3.0, 3.35, z), FESTIVE, 7, 0.45)
	for i: int in tops.size() - 1:
		if tops[i].distance_to(tops[i + 1]) < 7.0:
			ZoneKit.bunting(m, tops[i], tops[i + 1], FESTIVE, 4, 0.5)
	for b: Array in [[Vector3(-5.0, 0.08, 10.6), 30.0], [Vector3(5.0, 0.08, 10.6), -30.0]]:
		ZoneKit.bench(m, b[0] as Vector3, float(b[1]))
	ZoneKit.signpost(m, Vector3(3.2, 0, 12.0), [[Vector3(0, 0, -20), "home", "roof"], [GATES, "talk", "cloth_blue"],
		[CALENDAR, "book", "cloth_green"], [PODIUM, "star", "lantern_glow"]])
	for p: Array in [[Vector3(-4.2, 0, -8.0), ["cloth_red", "lantern_glow", "cloth_blue"]], [Vector3(4.2, 0, -14.5), ["cloth_green", "flower_pink", "cloth_violet"]],
			[Vector3(-8.0, 0, 9.5), ["lantern_glow", "cloth_blue", "flower_pink"]], [Vector3(8.6, 0, 9.0), ["cloth_red", "cloth_green", "lantern_glow"]]]:
		ZoneKit.balloons(m, p[0] as Vector3, PackedStringArray(p[1] as Array), rng)
	for st: Array in STALLS:
		m.push_at(st[0] as Vector3, float(st[1]))
		_stall(m, rng)
		m.pop()
	for p: Vector3 in [Vector3(-3.6, 0, -21.8), Vector3(3.6, 0, -21.8)]:
		HubArt._stone_lantern(m, p)
	return m.build()


## Festival stall (quầy lễ hội): counter, four posts, a striped awning and round fruit and drinks on the counter.
static func _stall(m: LowPoly, rng: RandomNumberGenerator) -> void:
	m.box(Vector3(0, 0.5, 0), Vector3(2.6, 1.0, 1.0), "wood", "wood_dark")
	m.box(Vector3(0, 0.5, 0.51), Vector3(2.4, 0.5, 0.04), "lantern_glow")
	for x: float in [-1.25, 1.25]:
		for z: float in [-0.45, 0.45]:
			m.cylinder(Vector3(x, 0, z), 0.06, 0.06, 2.4, 6, "wood_dark")
	for k: int in 6:
		m.push(Transform3D(Basis(Vector3.RIGHT, 0.35), Vector3(-1.3 + k * 0.52 + 0.26, 2.45, 0.2)))
		m.box(Vector3.ZERO, Vector3(0.5, 0.06, 1.6), "cloth_red" if k % 2 == 0 else "flower_white")
		m.pop()
	for k: int in 5:
		m.blob(Vector3(-1.0 + k * 0.5, 1.12, 0.1), Vector3(0.13, 0.13, 0.13), FESTIVE[rng.randi() % FESTIVE.size()], 0)


# ---------- vegetation ----------

const SCATTER: Dictionary = {
	"tree": "veg_tree_round_a", "blossom": "veg_tree_blossom_a", "bush": "veg_bush_a", "flower": "veg_flower_a",
	"tuft": "veg_tuft_a",
}


static func placements(kind: String) -> Array[Transform3D]:
	match kind:
		"tree":
			return ZoneKit.scatter_in(9510, 4, 16, 1.6, LAYOUT, BLOCKED, 0.85, 1.25)
		"blossom":
			return ZoneKit.scatter_in(9511, 4, 6, 1.6, LAYOUT, BLOCKED, 0.85, 1.2)
		"bush":
			return ZoneKit.scatter_in(9513, 24, 0, 1.0, LAYOUT, BLOCKED, 0.7, 1.2)
		"flower":
			return ZoneKit.scatter_in(9514, 80, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.3)
		"tuft":
			return ZoneKit.scatter_in(9515, 150, 0, 0.3, LAYOUT, BLOCKED, 0.8, 1.4)
	return []
