class_name FriendCodeWidget
extends VBoxContainer
## Mã bạn bè (F110-AC-23; STUDENT_GAME_UX_SOCIAL §6 `FriendCodeWidget`, SOCIAL_SYSTEM §4): "Tạo mã" shows the code in big
## characters with the countdown of the server `expires_at` and "Đọc to" (one character after the other — the current
## one is highlighted, and spoken when the device has a text-to-speech voice), for the friend sitting next to the
## student; "Nhập mã" upper-cases what is typed, keeps only A-Z / 0-9, at most 12 characters, and enables "Gửi" only when
## the code matches `^[A-Z0-9]{6,12}$`. UI only: the friends screen calls SocialService (AGENTS R1).

signal create_requested()
signal redeem_requested(code: String)

const READ_STEP_SEC: float = 0.9

var code: FriendCodeModel = null
## Index of the character being read aloud (-1 = not reading).
var reading_index: int = -1
var _create: Button = null
var _code_box: VBoxContainer = null
var _chars: HFlowContainer = null
var _countdown: Label = null
var _instructions: Label = null
var _read: Button = null
var _field: LineEdit = null
var _send: Button = null
var _read_left: float = 0.0
var _total_minutes: int = 0


func _init() -> void:
	name = "FriendCodeWidget"
	theme_type_variation = &"FormBox"
	add_child(FriendsUi.group_title("ui.friendcode.mine", "star"))
	_create = UiBuild.button("ui.friendcode.create", "plus")
	_create.name = "CreateCode"
	_create.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	_create.pressed.connect(func() -> void: create_requested.emit())
	add_child(_create)
	_code_box = UiBuild.vbox(&"FormBox")
	_code_box.name = "Code"
	_code_box.visible = false
	add_child(_code_box)
	_chars = HFlowContainer.new()
	_chars.name = "Chars"
	_code_box.add_child(_chars)
	var line: HFlowContainer = HFlowContainer.new()
	_code_box.add_child(line)
	_countdown = UiBuild.data_label("", &"InstructionLabel", false)
	_countdown.name = "Countdown"
	_countdown.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	line.add_child(_countdown)
	_read = UiBuild.button("ui.friendcode.read_aloud", "voice", &"SecondaryButton")
	_read.name = "ReadAloud"
	_read.pressed.connect(read_aloud)
	line.add_child(_read)
	_instructions = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_instructions.name = "Instructions"
	_code_box.add_child(_instructions)
	add_child(FriendsUi.group_title("ui.friendcode.enter_title", "edit"))
	var entry: HFlowContainer = HFlowContainer.new()
	entry.name = "Entry"
	add_child(entry)
	_field = LineEdit.new()
	_field.name = "CodeField"
	_field.max_length = FriendCodeModel.MAX_LENGTH
	_field.placeholder_text = "ui.friendcode.placeholder"
	_field.custom_minimum_size = Vector2(float(UiTokens.SIZE_LAYOUT_280) * 0.9, UiBuild.target())
	_field.text_changed.connect(_on_text_changed)
	_field.text_submitted.connect(func(_t: String) -> void: _submit())
	entry.add_child(_field)
	_send = UiBuild.button("ui.friendcode.send", "send")
	_send.name = "SendCode"
	_send.disabled = true
	_send.pressed.connect(_submit)
	entry.add_child(_send)


func _process(delta: float) -> void:
	if reading_index < 0:
		return
	_read_left -= delta
	if _read_left <= 0.0:
		_step_reading()


func show_code(value: FriendCodeModel, now_ms: int) -> void:
	code = value
	reading_index = -1
	_code_box.visible = code != null
	_create.text = "ui.friendcode.create_again" if code != null else "ui.friendcode.create"
	if code == null:
		return
	_total_minutes = maxi(1, ceili(float(code.left_ms(now_ms)) / 60000.0))
	_instructions.text = tr("ui.friendcode.instructions").format({"minutes": LocaleFormat.number(_total_minutes)})
	_render_chars()
	tick(now_ms)


## Countdown of `expires_at` (server time); an expired code disappears and "Tạo mã" comes back.
func tick(now_ms: int) -> void:
	if code == null:
		return
	var left: int = code.left_ms(now_ms)
	if left <= 0:
		code = null
		reading_index = -1
		_code_box.visible = false
		_create.text = "ui.friendcode.create"
		return
	_countdown.text = tr("ui.friendcode.expires_in").format({"time": ExamCountdown.clock_text(left)})


## "Đọc to": one character after the other (highlighted; spoken with the device voice when there is one).
func read_aloud() -> void:
	if code == null:
		return
	reading_index = -1
	_step_reading()


func field() -> LineEdit:
	return _field


func send_button() -> Button:
	return _send


## The characters as read aloud ("A B 3 …").
func spelled() -> String:
	return " ".join(code.code.split("")) if code != null else ""


## Types `text` into the code field as a student would (tests).
func type_text(text: String) -> void:
	_field.text = text
	_on_text_changed(text)


func _on_text_changed(text: String) -> void:
	var clean: String = FriendCodeModel.normalize(text)
	if clean != text:
		var caret: int = _field.caret_column
		_field.text = clean
		_field.caret_column = mini(caret, clean.length())
	_send.disabled = not FriendCodeModel.is_valid(clean)


func _submit() -> void:
	var clean: String = FriendCodeModel.normalize(_field.text)
	if FriendCodeModel.is_valid(clean):
		redeem_requested.emit(clean)


func _render_chars() -> void:
	UiBuild.clear(_chars)
	if code == null:
		return
	for i: int in code.code.length():
		var cell: PanelContainer = UiBuild.panel(&"FeedbackCorrectPanel" if i == reading_index else &"ValueChip")
		cell.name = "Char%d" % i
		var l: Label = UiBuild.data_label(code.code[i], &"CountdownLabel", false)
		l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		cell.add_child(l)
		_chars.add_child(cell)


func _step_reading() -> void:
	if code == null:
		reading_index = -1
		return
	reading_index += 1
	if reading_index >= code.code.length():
		reading_index = -1
		_render_chars()
		return
	_read_left = READ_STEP_SEC
	_render_chars()
	_speak(code.code[reading_index])


func _speak(text: String) -> void:
	if not bool(ProjectSettings.get_setting("audio/general/text_to_speech", false)):
		return
	var voices: PackedStringArray = DisplayServer.tts_get_voices_for_language(L10n.data_language(TranslationServer.get_locale()))
	if not voices.is_empty():
		DisplayServer.tts_speak(text, voices[0])
