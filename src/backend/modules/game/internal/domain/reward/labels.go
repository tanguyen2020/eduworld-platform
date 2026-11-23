package reward

// SourceLabels are the friendly names of the XP sources shown to the student (listXpHistory source_label,
// progression.xp.updated): never an id, a rule or a key (F074-AC-18). Keyed by source_code; vi and en (ADR-0056).
var SourceLabels = map[string]map[string]string{
	"quest.main_completed":       {"vi": "Hoàn thành nhiệm vụ chính", "en": "Main quest completed"},
	"quest.side_completed":       {"vi": "Hoàn thành nhiệm vụ phụ", "en": "Side quest completed"},
	"quest.daily_completed":      {"vi": "Hoàn thành nhiệm vụ hằng ngày", "en": "Daily quest completed"},
	"quest.weekly_completed":     {"vi": "Hoàn thành nhiệm vụ hằng tuần", "en": "Weekly quest completed"},
	"activity.completed":         {"vi": "Hoàn thành bài tập", "en": "Activity completed"},
	"mastery.objective_mastered": {"vi": "Thành thạo một mục tiêu học tập", "en": "Learning objective mastered"},
	"lesson.completed":           {"vi": "Hoàn thành bài học", "en": "Lesson completed"},
	"exploration.discovery":      {"vi": "Khám phá một điểm mới", "en": "New discovery"},
	"exploration.area_completed": {"vi": "Khám phá trọn một khu vực", "en": "Area fully explored"},
	"merge.adjustment":           {"vi": "Gộp tài khoản", "en": "Account merge"},
}

// SourceLabel is the label of a source code in lang ("vi" or "en"; vi by default); "" when unknown.
func SourceLabel(code, lang string) string {
	l, ok := SourceLabels[code]
	if !ok {
		return ""
	}
	if s := l[lang]; s != "" {
		return s
	}
	return l["vi"]
}
