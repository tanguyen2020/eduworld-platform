class_name FormColumn
extends VBoxContainer
## Centred column of the entry screens (F096; same width as the S01 `LoginCard`): `UiTokens.COMPONENT_FORM_CARD_WIDTH`
## wide so wrapped texts (Mây's speech bubble, notices) keep a readable line length on every window. Size from tokens
## only (AGENTS R32); layout only.


func _ready() -> void:
	custom_minimum_size.x = UiTokens.COMPONENT_FORM_CARD_WIDTH
