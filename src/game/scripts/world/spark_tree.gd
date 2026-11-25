class_name SparkTree
extends MeshInstance3D
## The Spark Tree (Cây Tia Sáng) of the Main Hub (F068, WORLD_DESIGN §4.1, §11): shows one baked growth stage
## (`stage_meshes`, tools/art/hub_art.gd `spark_tree`). Display only (AGENTS R1): the stage comes from mastery data
## through `show_mastery()` (SparkTreeModel) or `set_stage()`; the scene holds no rule and calls no service.

signal stage_changed(stage: int)

## Baked meshes of stage 0 (grey sapling) … 4 (in bloom).
@export var stage_meshes: Array[Mesh] = []
@export var stage: int = SparkTreeModel.DEFAULT_STAGE


func _ready() -> void:
	_apply()


func set_stage(value: int) -> void:
	var next: int = clampi(value, 0, maxi(stage_meshes.size() - 1, 0))
	var changed: bool = next != stage
	stage = next
	_apply()
	if changed:
		stage_changed.emit(stage)


## Shows the stage for a list of `MasteryLevel` values; returns it.
func show_mastery(levels: PackedStringArray) -> int:
	set_stage(SparkTreeModel.stage_for(levels))
	return stage


func _apply() -> void:
	if stage >= 0 and stage < stage_meshes.size() and stage_meshes[stage] != null:
		mesh = stage_meshes[stage]
