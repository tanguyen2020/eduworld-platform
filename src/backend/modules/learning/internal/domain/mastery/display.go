package mastery

// Grade bands of the student UX (LEARNING_EXPERIENCE §7, §8).
const (
	BandG13 = "g1_3"
	BandG45 = "g4_5"
	BandG67 = "g6_7"
	BandG89 = "g8_9"
)

// GradeBand is the band of a grade (nil or out of range = g1_3, the most protective display) [Proposed].
func GradeBand(grade *int) string {
	if grade == nil {
		return BandG13
	}
	switch g := *grade; {
	case g >= 8:
		return BandG89
	case g >= 6:
		return BandG67
	case g >= 4:
		return BandG45
	}
	return BandG13
}

// ShowsValue reports whether the band sees the numeric mastery (grades 6–9 only, LEARNING_EXPERIENCE §7, F032-AC-34).
func ShowsValue(band string) bool { return band == BandG67 || band == BandG89 }

// IconCode is the icon of a level for a band ("<band>.<level>"): the game maps it to its art (stations, trees, stars),
// no TT27/TT22 label and no comparison with other students.
func IconCode(band string, level Level) string { return band + "." + string(level) }
