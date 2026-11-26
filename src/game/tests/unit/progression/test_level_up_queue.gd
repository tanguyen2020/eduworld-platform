extends GdUnitTestSuite
## F069 LevelUpQueue (T010; F069-AC-03, AC-UX-0701/0704): several `progression.level.reached` become ONE announcement at
## the highest level with every reward; while a timed activity runs (`AppState.blocking_activity`) nothing is released,
## and the batch comes out as soon as it ends; a message arriving while the modal is open waits for it to close.

var store: Node = null
var queue: LevelUpQueue = null
var released: Array[LevelUpModel] = []


func before_test() -> void:
	store = auto_free(BlockingStoreFake.new())
	queue = LevelUpQueue.new(store)
	released = []
	queue.batch_released.connect(func(b: LevelUpModel) -> void: released.append(b))


func test_two_messages_merge_into_one_multi_level_modal() -> void:
	store.call("set_blocking", &"challenge")
	queue.push(LevelUpModel.from_dict(F069Fixtures.level_up(3, 4)))
	queue.push(LevelUpModel.from_dict(F069Fixtures.level_up(4, 5, false)))
	assert_array(released).is_empty()
	store.call("set_blocking", &"")
	assert_int(released.size()).is_equal(1)
	var b: LevelUpModel = released[0]
	assert_int(b.to_level).is_equal(5)
	assert_int(b.levels_gained).is_equal(2)
	assert_bool(b.is_multi()).is_true()
	assert_int(b.rewards.size()).is_equal(2)
	assert_int(b.rewards[0].amount).is_equal(100)
	assert_str(b.first_item_id()).is_equal("acc_cap_star")


func test_free_queue_releases_at_once_and_merges_into_the_open_modal() -> void:
	queue.push(LevelUpModel.from_dict(F069Fixtures.level_up(3, 4)))
	assert_int(released.size()).is_equal(1)
	assert_object(queue.current).is_not_null()
	# A second level while the modal is still open turns it into "Lên 2 cấp!" (never two modals in a row).
	queue.push(LevelUpModel.from_dict(F069Fixtures.level_up(4, 5)))
	assert_int(released.size()).is_equal(2)
	assert_int(released[1].to_level).is_equal(5)
	assert_int(released[1].levels_gained).is_equal(2)
	queue.mark_shown()
	assert_int(released.size()).is_equal(2)
	assert_object(queue.current).is_null()


func test_real_app_state_blocking_holds_the_batch() -> void:
	var real: LevelUpQueue = LevelUpQueue.new(AppState)
	var got: Array[LevelUpModel] = []
	real.batch_released.connect(func(b: LevelUpModel) -> void: got.append(b))
	AppState.begin_blocking(&"exam")
	real.push(LevelUpModel.from_dict(F069Fixtures.level_up(1, 2)))
	assert_array(got).is_empty()
	AppState.end_blocking(&"exam")
	assert_int(got.size()).is_equal(1)
	assert_str(String(AppState.blocking_activity)).is_empty()
