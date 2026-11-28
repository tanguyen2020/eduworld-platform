class_name QuestGo
extends RefCounted
## "Đi" of the Event center (F111-AC-10, AC-15): hands a quest to QuestService (F072) — accepted first when it is still
## `available` and can be taken (the server checks grade, prerequisites, limits), then tracked so the Quest Tracker and
## the markers lead the way. The client decides nothing about the quest itself (ADR-0016, AGENTS R1).


## Coroutine: true when the quest is now tracked.
static func go(quest: QuestService, quest_id: String) -> bool:
	if quest == null or not Uuid.is_valid(quest_id):
		return false
	var q: QuestModel = quest.quest(quest_id)
	if q == null:
		await quest.fetch_detail(quest_id)
		q = quest.quest(quest_id)
	if q != null and q.status == QuestModel.STATUS_AVAILABLE and q.can_accept:
		var r: ApiResult = await quest.accept(quest_id)
		return r.ok
	if q != null and q.is_active():
		quest.track(quest_id)
		return true
	return false
