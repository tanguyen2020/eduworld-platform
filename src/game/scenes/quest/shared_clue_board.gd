class_name SharedClueBoard
extends VBoxContainer
## The board of an `interdependent` step (F110-AC-16, AC-18): every clue card a member shared, in a wrapping flow — the
## same cards on every member's screen once the server says so (`quest.clue.shared` → `getQuestStepClues`, ≤ 1 s);
## one card per role, never twice (spec edge case). Shared cards stay while the step runs. UI only.

var _cards: HFlowContainer = null
var _empty: Label = null


func _init() -> void:
	name = "SharedClueBoard"
	theme_type_variation = &"FormBox"
	add_child(FriendsUi.group_title("ui.quest.clue.board", "pin"))
	_empty = UiBuild.keep_words(UiBuild.label("ui.quest.clue.board_empty", &"MutedLabel"))
	_empty.name = "BoardEmpty"
	add_child(_empty)
	_cards = HFlowContainer.new()
	_cards.name = "Cards"
	add_child(_cards)


func show_cards(cards: Array[ClueCardModel]) -> void:
	UiBuild.clear(_cards)
	var seen: Dictionary = {}
	for c: ClueCardModel in cards:
		if not c.shared or seen.has(c.role):
			continue
		seen[c.role] = true
		var card: ClueCard = ClueCard.new()
		card.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_280)
		card.show_card(c, false)
		_cards.add_child(card)
	_empty.visible = seen.is_empty()


func card_count() -> int:
	return _cards.get_child_count()


func roles() -> PackedStringArray:
	var out: PackedStringArray = []
	for c: Node in _cards.get_children():
		out.append((c as ClueCard).role)
	return out
