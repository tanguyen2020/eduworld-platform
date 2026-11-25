class_name AvatarBuilder
extends Node3D
## Assembles an avatar from an `AvatarConfig` (CHARACTER_SYSTEM §5.2; F070-AC-06, F070-AC-18) — shared by the local
## Player, S02 Character Creation and the Wardrobe (CustomizationStudio); RemotePlayer and the profile card reuse it
## later (F092/F095). Display only and synchronous: no network, no ownership rule (the server decides), so every
## change of choice redraws the preview locally.
## - body type → proportions (`resources/avatar/<code>.tres`), skin tone / hair colour → avatar palette tokens
##   (`ItemVisual.appearance_color`, T041);
## - hair, outfit, accessories (head/face/back/hand) → visual of `resources/items/<item_id>.tres` on its slot anchor;
## - an equipped item without a visual on this client (cosmetic bundle not loaded) → the default item of the same slot
##   from the catalog, `visual_fallback_used` is emitted for the background bundle loader (F070-AC-18); unknown
##   appearance codes keep the neutral look. Never crashes on bad data.
## Art (F067, F068; ADR-0102, ADR-0103 "chibi anime cartoon 3D"): a baked chibi body skinned to the `kid_base` rig
## (KidSkeleton: head about half of the height), a separate face mesh per expression (`set_expression`, with a
## blink of the neutral face), baked hair/accessories on BoneAttachment3D slot anchors and skinned outfits, drawn with
## the shared cel presets (`Art3dTokens.MATERIAL_PATHS`: `toon_skinned` for skinned meshes, `toon` otherwise); skin,
## hair colour and item tints set their `albedo` per avatar (faces flagged ALBEDO in the bake), everything else comes
## from the art palette tokens.

signal avatar_built(config: AvatarConfigModel)
signal visual_fallback_used(item_id: String, slot: String)
signal expression_changed(expression: String)

const ITEM_SLOTS: PackedStringArray = ["hair", "outfit", "head", "face", "back", "hand"]
const ANCHOR_PATHS: Dictionary = {
	"hair": "Bob/Skeleton3D/HeadAttach/Hair", "outfit": "Bob/Skeleton3D/Outfit", "head": "Bob/Skeleton3D/HeadAttach/Head",
	"face": "Bob/Skeleton3D/HeadAttach/Face", "back": "Bob/Skeleton3D/ChestAttach/Back",
	"hand": "Bob/Skeleton3D/HandAttach/Hand",
}
## Blink of the neutral face (display only): every BLINK_EVERY seconds for BLINK_TIME seconds.
const BLINK_EVERY: float = 3.4
const BLINK_TIME: float = 0.12

## Face meshes loaded on this client (display data), by expression.
static var _faces: Dictionary = {}

@export var body_mesh: MeshInstance3D = null
## Legacy greybox head (separate skin mesh); the F067 body has the head in `body_mesh`.
@export var head_mesh: MeshInstance3D = null
@export var proportions: Node3D = null
## Expression face (F068): its mesh is swapped by `set_expression`.
@export var face_mesh: MeshInstance3D = null
## `kid_base` skeleton that skinned item meshes (outfits) bind to.
@export var skeleton: Skeleton3D = null

## The config last applied, exactly as given (fallbacks are display only).
var config: AvatarConfigModel = null
## slot → item code actually drawn (after fallback); a slot without an item is absent.
var shown: Dictionary = {}
## Equipped item codes that had no visual on this client during the last apply.
var missing: PackedStringArray = []
var _skin_material: ShaderMaterial = toon_material(true)
## Expression shown (KidSkeleton.EXPRESSIONS); the blink frame is not an expression.
var expression: String = KidSkeleton.DEFAULT_EXPRESSION
var blinking: bool = false
var _blink_clock: float = 0.0
var _skin_color: Color = ItemVisual.UNKNOWN_APPEARANCE_COLOR
var _hair_color: Color = ItemVisual.UNKNOWN_APPEARANCE_COLOR


func _ready() -> void:
	if body_mesh != null:
		body_mesh.material_override = _skin_material
	if head_mesh != null:
		head_mesh.material_override = _skin_material
	set_process(face_mesh != null)


func _process(delta: float) -> void:
	advance_blink(delta)


## F068: shows facial expression `value` (KidSkeleton.EXPRESSIONS). False (unchanged) for an unknown value.
func set_expression(value: String) -> bool:
	if not value in KidSkeleton.EXPRESSIONS:
		return false
	var changed: bool = value != expression
	expression = value
	blinking = false
	_blink_clock = 0.0
	_show_face(value)
	if changed:
		expression_changed.emit(value)
	return true


## Blink clock of the neutral face (called from `_process`; tests call it directly).
func advance_blink(delta: float) -> void:
	if expression != KidSkeleton.DEFAULT_EXPRESSION:
		return
	_blink_clock += delta
	if blinking and _blink_clock >= BLINK_TIME:
		blinking = false
		_blink_clock = 0.0
		_show_face(expression)
	elif not blinking and _blink_clock >= BLINK_EVERY:
		blinking = true
		_blink_clock = 0.0
		_show_face(KidSkeleton.BLINK)


func _show_face(value: String) -> void:
	if face_mesh == null:
		return
	if not _faces.has(value):
		var path: String = KidSkeleton.face_path(value)
		_faces[value] = load(path) as Mesh if ResourceLoader.exists(path) else null
	if _faces[value] != null:
		face_mesh.mesh = _faces[value] as Mesh


## Redraws the avatar for `new_config` (null clears the items). `catalog` gives the default item of each slot.
func apply(new_config: AvatarConfigModel, catalog: CharacterCatalogModel = null) -> void:
	config = new_config
	shown.clear()
	missing.clear()
	for slot: String in ITEM_SLOTS:
		_clear_anchor(slot)
	if new_config == null:
		avatar_built.emit(new_config)
		return
	_apply_body(new_config)
	var defaults: PackedStringArray = catalog.default_item_ids if catalog != null else PackedStringArray()
	_place("hair", new_config.hair_item_id, defaults)
	_place("outfit", new_config.outfit_item_id, defaults)
	for slot: String in AvatarConfigModel.ACCESSORY_SLOTS:
		if new_config.accessories.has(slot):
			_place(slot, String(new_config.accessories[slot]), defaults)
	avatar_built.emit(new_config)


func slot_anchor(slot: String) -> Node3D:
	return get_node_or_null(String(ANCHOR_PATHS.get(slot, ""))) as Node3D


func skin_color() -> Color:
	return _skin_color


func hair_color() -> Color:
	return _hair_color


func _apply_body(c: AvatarConfigModel) -> void:
	var shape: ItemVisual = ItemVisual.load_appearance(c.body_type)
	if proportions != null:
		proportions.scale = shape.body_scale if shape != null else Vector3.ONE
	_skin_color = ItemVisual.appearance_color(c.skin_tone)
	_skin_material.set_shader_parameter(&"albedo", _skin_color)
	_hair_color = ItemVisual.appearance_color(c.hair_color)


## Draws `item_id` on `slot`, or the slot's default when the visual is missing / for another slot.
func _place(slot: String, item_id: String, defaults: PackedStringArray) -> void:
	var visual: ItemVisual = ItemVisual.load_item(item_id)
	var drawn: String = item_id
	if visual == null or visual.slot != slot:
		missing.append(item_id)
		drawn = ItemVisual.first_for_slot(slot, defaults, item_id)
		visual = ItemVisual.load_item(drawn) if not drawn.is_empty() else null
		visual_fallback_used.emit(item_id, slot)
	if visual == null:
		return
	var anchor: Node3D = slot_anchor(slot)
	if anchor == null:
		return
	anchor.add_child(_build_part(visual, slot))
	shown[slot] = drawn


func _build_part(visual: ItemVisual, slot: String) -> Node3D:
	if visual.scene != null:
		var instance: Node3D = visual.scene.instantiate() as Node3D
		if instance != null:
			instance.position = visual.offset
			return instance
	var part: MeshInstance3D = MeshInstance3D.new()
	part.name = "Part"
	part.mesh = visual.mesh
	part.position = visual.offset
	var skinned: bool = is_skinned(visual.mesh) and skeleton != null
	if skinned:
		# Outfits bend with the rig: bound to the kid_base skeleton (the anchor sits under it).
		part.skeleton = NodePath("../..")
	var material: ShaderMaterial = null
	if visual.material_override is ShaderMaterial:
		material = (visual.material_override as ShaderMaterial).duplicate() as ShaderMaterial
	else:
		material = toon_material(skinned)
	material.set_shader_parameter(&"albedo", _hair_color if slot == "hair" else visual.display_tint())
	part.material_override = material
	return part


## True when `mesh` carries bone weights (skinned to the kid_base rig).
static func is_skinned(mesh: Mesh) -> bool:
	if mesh == null or mesh.get_surface_count() == 0:
		return false
	return (mesh.surface_get_format(0) & Mesh.ARRAY_FORMAT_BONES) != 0


## A per-avatar copy of the shared toon preset (generated from the tokens): `toon_skinned` (outline along the skinned
## normal) for skinned meshes, `toon` otherwise; the outline pass stays shared.
static func toon_material(skinned: bool = false) -> ShaderMaterial:
	var key: String = "toon_skinned" if skinned else "toon"
	var preset: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS[key])) as ShaderMaterial
	return preset.duplicate() as ShaderMaterial if preset != null else ShaderMaterial.new()


func _clear_anchor(slot: String) -> void:
	var anchor: Node3D = slot_anchor(slot)
	if anchor == null:
		return
	for child: Node in anchor.get_children():
		anchor.remove_child(child)
		child.queue_free()
