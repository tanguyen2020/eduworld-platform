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
## Art (F067, ADR-0102): a baked chibi body (head ≈ 1/3, face in the mesh) and baked hair/outfit/accessory meshes
## drawn with the shared toon preset (`Art3dTokens.MATERIAL_PATHS["toon"]`); skin, hair colour and item tints set its
## `albedo` per avatar (faces flagged ALBEDO in the bake), everything else comes from the art palette tokens. The
## anchors are plain Node3D; a skinned `kid_base` rig (BoneAttachment3D, merged mesh, §5.2) is a later art pass.

signal avatar_built(config: AvatarConfigModel)
signal visual_fallback_used(item_id: String, slot: String)

const ITEM_SLOTS: PackedStringArray = ["hair", "outfit", "head", "face", "back", "hand"]
const ANCHOR_PATHS: Dictionary = {
	"hair": "Bob/Slots/Hair", "outfit": "Bob/Slots/Outfit", "head": "Bob/Slots/Head", "face": "Bob/Slots/Face",
	"back": "Bob/Slots/Back", "hand": "Bob/Slots/Hand",
}

@export var body_mesh: MeshInstance3D = null
## Legacy greybox head (separate skin mesh); the F067 body has the head in `body_mesh`.
@export var head_mesh: MeshInstance3D = null
@export var proportions: Node3D = null

## The config last applied, exactly as given (fallbacks are display only).
var config: AvatarConfigModel = null
## slot → item code actually drawn (after fallback); a slot without an item is absent.
var shown: Dictionary = {}
## Equipped item codes that had no visual on this client during the last apply.
var missing: PackedStringArray = []
var _skin_material: ShaderMaterial = toon_material()
var _skin_color: Color = ItemVisual.UNKNOWN_APPEARANCE_COLOR
var _hair_color: Color = ItemVisual.UNKNOWN_APPEARANCE_COLOR


func _ready() -> void:
	if body_mesh != null:
		body_mesh.material_override = _skin_material
	if head_mesh != null:
		head_mesh.material_override = _skin_material


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
	var material: ShaderMaterial = null
	if visual.material_override is ShaderMaterial:
		material = (visual.material_override as ShaderMaterial).duplicate() as ShaderMaterial
	else:
		material = toon_material()
	material.set_shader_parameter(&"albedo", _hair_color if slot == "hair" else visual.display_tint())
	part.material_override = material
	return part


## A per-avatar copy of the shared toon preset (generated from the tokens); the outline pass stays shared.
static func toon_material() -> ShaderMaterial:
	var preset: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon"])) as ShaderMaterial
	return preset.duplicate() as ShaderMaterial if preset != null else ShaderMaterial.new()


func _clear_anchor(slot: String) -> void:
	var anchor: Node3D = slot_anchor(slot)
	if anchor == null:
		return
	for child: Node in anchor.get_children():
		anchor.remove_child(child)
		child.queue_free()
