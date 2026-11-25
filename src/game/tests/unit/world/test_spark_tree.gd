extends GdUnitTestSuite
## F068-AC-09 (WORLD_DESIGN §4.1, §11 mastery visualization; open decision F067-4): the Spark Tree shows 5 growth
## stages — grey sapling → young tree → growing → full banyan → in bloom — from mastery levels of the contract enum
## `MasteryLevel` (SparkTreeModel, display only), DEFAULT_STAGE without data; the hub tree swaps its baked mesh.

const ArtCatalog := preload("res://tools/art/art_catalog.gd")
const ZONE: String = "res://scenes/world/zones/main_hub/Zone.tscn"


static func _slots(mesh: Mesh) -> Dictionary:
	var out: Dictionary = {}
	for uv: Vector2 in mesh.surface_get_arrays(0)[Mesh.ARRAY_TEX_UV]:
		if uv.y < 1.0:
			out[Art3dTokens.ART_PALETTE[int(uv.x)]] = true
	return out


func test_stage_is_the_rounded_mean_level() -> void:
	assert_int(SparkTreeModel.stage_for(PackedStringArray())).is_equal(SparkTreeModel.DEFAULT_STAGE)
	assert_int(SparkTreeModel.stage_for(PackedStringArray(["unknown_level"]))).is_equal(SparkTreeModel.DEFAULT_STAGE)
	assert_int(SparkTreeModel.stage_for(PackedStringArray(["not_started", "not_started"]))).is_equal(0)
	assert_int(SparkTreeModel.stage_for(PackedStringArray(["beginning"]))).is_equal(1)
	assert_int(SparkTreeModel.stage_for(PackedStringArray(["beginning", "proficient"]))).is_equal(2)
	assert_int(SparkTreeModel.stage_for(PackedStringArray(["proficient", "mastered", "developing"]))).is_equal(3)
	assert_int(SparkTreeModel.stage_for(PackedStringArray(["mastered", "mastered", "bogus"]))).is_equal(4)
	assert_array(Array(SparkTreeModel.LEVELS)).contains_exactly(["not_started", "beginning", "developing", "proficient",
		"mastered"])


func test_stages_go_from_grey_to_bloom() -> void:
	var sizes: Array[float] = []
	for stage: int in ArtCatalog.SPARK_TREE_STAGES:
		var mesh: Mesh = load(ArtCatalog.spark_tree_path(stage)) as Mesh
		sizes.append(mesh.get_aabb().size.y)
	for i: int in 3:
		assert_float(sizes[i + 1]).is_greater(sizes[i])
	var grey: Dictionary = _slots(load(ArtCatalog.spark_tree_path(0)) as Mesh)
	assert_bool(grey.has("ash")).is_true()
	assert_bool(grey.has("leaf") or grey.has("leaf_dark") or grey.has("spark")).is_false()
	var young: Dictionary = _slots(load(ArtCatalog.spark_tree_path(1)) as Mesh)
	assert_bool(young.has("leaf_dark")).is_true()
	assert_bool(young.has("spark")).is_false()
	assert_bool(_slots(load(ArtCatalog.spark_tree_path(2)) as Mesh).has("spark")).is_true()
	var bloom: Dictionary = _slots(load(ArtCatalog.spark_tree_path(4)) as Mesh)
	assert_bool(bloom.has("flower_pink") and bloom.has("spark")).is_true()
	assert_bool(_slots(load(ArtCatalog.spark_tree_path(3)) as Mesh).has("flower_pink")).is_false()


func test_hub_tree_swaps_its_stage_mesh() -> void:
	var runner: GdUnitSceneRunner = scene_runner(ZONE)
	await runner.simulate_frames(1)
	var tree: SparkTree = runner.scene().get_node("Static/SparkTree") as SparkTree
	assert_int(tree.stage_meshes.size()).is_equal(ArtCatalog.SPARK_TREE_STAGES)
	assert_int(tree.stage).is_equal(SparkTreeModel.DEFAULT_STAGE)
	assert_str(tree.mesh.resource_path).is_equal(ArtCatalog.spark_tree_path(SparkTreeModel.DEFAULT_STAGE))
	var changed: Array[int] = []
	tree.stage_changed.connect(func(s: int) -> void: changed.append(s))
	assert_int(tree.show_mastery(PackedStringArray(["not_started"]))).is_equal(0)
	assert_str(tree.mesh.resource_path).is_equal(ArtCatalog.spark_tree_path(0))
	tree.set_stage(99)
	assert_int(tree.stage).is_equal(4)
	assert_str(tree.mesh.resource_path).is_equal(ArtCatalog.spark_tree_path(4))
	assert_array(changed).contains_exactly([0, 4])
	# Display only: no collision or contract change (the seat collision stays where F062 put it).
	assert_object(runner.scene().find_child("SparkTreeCollision", true, false)).is_not_null()
