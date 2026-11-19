extends Control
## TenantChoice (F061-AC-13, F061-AC-15, STUDENT_GAME_UX_CORE S01): "Em muốn vào trường nào hôm nay?" after
## authentication only; one large card per `choices[]` entry (name — school), nothing pre-selected, scrollable
## list readable by a screen reader. "Vào game" sends `selectGameLoginTenant` through AuthService (no password).

var _group: ButtonGroup = ButtonGroup.new()
var _selected_tenant_id: String = ""

@onready var _list: VBoxContainer = %List
@onready var _enter: Button = %Enter
@onready var _back: Button = %Back


func _ready() -> void:
	_group.allow_unpress = false
	for choice: ApiProblem.TenantChoice in AuthService.choices:
		var card: Button = Button.new()
		card.toggle_mode = true
		card.button_group = _group
		card.button_pressed = false
		card.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		card.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		card.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		card.text = choice.name if choice.school_name.is_empty() else "%s — %s" % [choice.name, choice.school_name]
		card.toggled.connect(_on_card_toggled.bind(choice.tenant_id))
		_list.add_child(card)
	_enter.disabled = true
	_enter.pressed.connect(_on_enter_pressed)
	_back.pressed.connect(_on_back_pressed)


## Number of cards shown (tests).
func card_count() -> int:
	return _list.get_child_count()


func _on_card_toggled(pressed: bool, tenant_id: String) -> void:
	if pressed:
		_selected_tenant_id = tenant_id
	_enter.disabled = _selected_tenant_id.is_empty()


func _on_enter_pressed() -> void:
	if _selected_tenant_id.is_empty():
		return
	_enter.disabled = true
	AuthService.select_tenant(_selected_tenant_id)


func _on_back_pressed() -> void:
	AuthService.back_to_login()
