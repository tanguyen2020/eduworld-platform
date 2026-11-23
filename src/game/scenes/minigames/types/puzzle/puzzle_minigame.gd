extends MiniGameQuestionScene
## `puzzle` (MINIGAME_ENGINE §2): each answer the server grades correct uncovers one piece of the picture board. A piece
## opens only from the server's verdict (`show_feedback`, or at the end when the effects are deferred) — the scene never
## decides that an answer is right (ADR-0016). One piece per item of the attempt.

const PIECE_HIDDEN: String = "?"
const PIECE_SHOWN: String = "★"

var pieces: Array[Label] = []
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
		var piece: Label = Label.new()
		piece.text = PIECE_HIDDEN
		piece.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		piece.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
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
		pieces[revealed].text = PIECE_SHOWN
		revealed += 1
