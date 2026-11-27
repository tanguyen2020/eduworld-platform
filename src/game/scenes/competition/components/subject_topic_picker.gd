class_name SubjectTopicPicker
extends VBoxContainer
## Subject (and, when the mode queues by topic, topic) of the match (F109-AC-01; STUDENT_GAME_UX_SOCIAL §8
## `SubjectTopicPicker`): one row of chips per choice (yellow + tick when chosen, F114), subjects from the mode the
## server returned, names from `listSubjects` / `listTopics`. "Tất cả chủ đề" = no topic. UI only.

signal selection_changed()

var subject_id: String = ""
var topic_id: String = ""
var _subjects_row: HBoxContainer = null
var _topics_title: Label = null
var _topics_row: HBoxContainer = null
var _topics_scroller: Control = null
var _subject_group: ButtonGroup = ButtonGroup.new()
var _topic_group: ButtonGroup = ButtonGroup.new()


func _init() -> void:
	name = "SubjectTopicPicker"
	theme_type_variation = &"FieldBox"
	# One line per choice: the word, then the chips (scrolling sideways when they do not fit).
	var subject_line: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(subject_line)
	var subject_title: Label = UiBuild.label("ui.arena.subject.title", &"HeadingLabel", false)
	subject_title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	subject_line.add_child(subject_title)
	_subjects_row = UiBuild.hbox(&"ToolbarBox")
	_subjects_row.name = "Subjects"
	subject_line.add_child(UiBuild.expand(UiBuild.chip_scroller(_subjects_row)))
	var topic_line: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(topic_line)
	_topics_title = UiBuild.label("ui.arena.topic.title", &"HeadingLabel", false)
	_topics_title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	topic_line.add_child(_topics_title)
	_topics_row = UiBuild.hbox(&"ToolbarBox")
	_topics_row.name = "Topics"
	_topics_scroller = UiBuild.chip_scroller(_topics_row)
	topic_line.add_child(UiBuild.expand(_topics_scroller))


## Rebuilds the subject chips for `subject_ids` (keeps the chosen one when still offered, else the first).
func set_subjects(subject_ids: PackedStringArray, names: Callable) -> void:
	UiBuild.clear(_subjects_row)
	if not subject_id in subject_ids:
		subject_id = subject_ids[0] if not subject_ids.is_empty() else ""
		topic_id = ""
	for id: String in subject_ids:
		var chip: Button = UiBuild.chip("", _subject_group)
		chip.name = "Subject_%s" % id.right(4)
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.text = String(names.call(id))
		chip.set_pressed_no_signal(id == subject_id)
		chip.pressed.connect(_on_subject.bind(id))
		_subjects_row.add_child(chip)
	_mark()


## Topic chips ("Tất cả chủ đề" + `topics` `{topic_id, name}`); hidden when the mode does not queue by topic.
func set_topics(topics: Array, allowed: bool) -> void:
	UiBuild.clear(_topics_row)
	(_topics_title.get_parent() as Control).visible = allowed
	_topics_scroller.visible = allowed
	if not allowed:
		topic_id = ""
		return
	var all: Button = UiBuild.chip("ui.arena.topic.all", _topic_group)
	all.name = "Topic_all"
	all.set_pressed_no_signal(topic_id.is_empty())
	all.pressed.connect(_on_topic.bind(""))
	_topics_row.add_child(all)
	for t: Variant in topics:
		var d: Dictionary = t as Dictionary
		var id: String = String(d.get("topic_id", ""))
		var chip: Button = UiBuild.chip("", _topic_group)
		chip.name = "Topic_%s" % id.right(4)
		chip.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		chip.text = L10n.pick(d.get("name", {}) as Dictionary)
		chip.set_pressed_no_signal(id == topic_id)
		chip.pressed.connect(_on_topic.bind(id))
		_topics_row.add_child(chip)
	_mark()


func subject_chips() -> Array[Node]:
	return _subjects_row.get_children()


func topic_chips() -> Array[Node]:
	return _topics_row.get_children() if _topics_scroller.visible else []


func _on_subject(id: String) -> void:
	if id != subject_id:
		subject_id = id
		topic_id = ""
	_mark()
	selection_changed.emit()


func _on_topic(id: String) -> void:
	topic_id = id
	_mark()
	selection_changed.emit()


func _mark() -> void:
	UiBuild.mark_chips(_subjects_row.get_children())
	UiBuild.mark_chips(_topics_row.get_children())
