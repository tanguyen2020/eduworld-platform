extends MiniGameQuestionScene
## `puzzle` (MINIGAME_ENGINE §2): each answer the server grades correct uncovers one piece of the picture board. A piece
## opens only from the server's verdict (`show_feedback`, or at the end when the effects are deferred) — the scene never
## decides that an answer is right (ADR-0016). One piece per item of the attempt.

const PIECE_HIDDEN: String = "?"
## F096-AC-15: an uncovered piece shows the star icon of design/icons/game (no "★" glyph on the Web build).
const PIECE_SHOWN: Texture2D = preload("res://resources/ui/icons/star.svg")

var pieces: Array[Button] = []
var revealed: int = 0

@onready var _board: GridContainer = %Board


func _on_setup() -> void:
	for child: Node in _board.get_children():
		_board.remove_child(child)
		child.queue_free()
	pieces.clear()
	revealed = 0
	var count: int = session.items().size() if session != null else 0
	_board.columns = maxi(1, ceili(sqrt(float(count))))
	for i: int in count:
		# A flat, non-interactive tile (a Button only for its centred icon + text layout).
		var piece: Button = Button.new()
		piece.flat = true
		piece.disabled = true
		piece.focus_mode = Control.FOCUS_NONE
		piece.mouse_filter = Control.MOUSE_FILTER_IGNORE
		piece.text = PIECE_HIDDEN
		piece.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		piece.alignment = HORIZONTAL_ALIGNMENT_CENTER
		piece.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
		piece.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_board.add_child(piece)
		pieces.append(piece)
	_board.visible = count > 0


func _show_effect(kind: String, result: ResponseResultModel) -> void:
	super._show_effect(kind, result)
	if kind == MiniGameFeedbackSfx.KIND_CORRECT:
		_reveal()


func _on_finish(summary: MiniGameSummaryModel, marks: Array[ResponseResultModel]) -> void:
	for r: ResponseResultModel in marks:
		if r.has_verdict and r.is_correct:
			_reveal()
	super._on_finish(summary, marks)
	_board.visible = not pieces.is_empty()


func _reveal() -> void:
	if revealed < pieces.size():
		pieces[revealed].text = ""
		pieces[revealed].icon = PIECE_SHOWN
		revealed += 1
