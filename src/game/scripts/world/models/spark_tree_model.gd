class_name SparkTreeModel
extends RefCounted
## Growth stage of the Spark Tree (Cây Tia Sáng) of the Main Hub from mastery levels (F068; WORLD_DESIGN §4.1, §11
## "Mastery visualization": grey → coloured → blooming / big tree along `beginning/developing/proficient/mastered`).
## Pure display mapping, no rule of the server: the levels are the contract enum `MasteryLevel` (learning.v1) of the
## data the client is given; the stage is the rounded mean level (0 = not_started … 4 = mastered). Without data the
## tree shows DEFAULT_STAGE [Proposed — F068 open question F068-1: no aggregate source of the grade's mastery yet].

## Contract enum `MasteryLevel` in order (learning.v1.yaml).
const LEVELS: PackedStringArray = ["not_started", "beginning", "developing", "proficient", "mastered"]
const STAGE_COUNT: int = 5
const DEFAULT_STAGE: int = 3


## Stage 0–4 for a list of mastery levels; unknown values are ignored; no known value → DEFAULT_STAGE.
static func stage_for(levels: PackedStringArray) -> int:
	var total: int = 0
	var known: int = 0
	for level: String in levels:
		var i: int = LEVELS.find(level)
		if i < 0:
			continue
		total += i
		known += 1
	if known == 0:
		return DEFAULT_STAGE
	return clampi(roundi(float(total) / float(known)), 0, STAGE_COUNT - 1)
