//go:build anticheat

package scenarios

import "testing"

// F132-AC-23 (TC-EXM-006): answer after the deadline, battle answer for a question never shown, resume within the
// grace, replayed WebSocket id.
func TestAnswerAfterDeadline(t *testing.T)  { run(t, "AC-CMP-204") }
func TestBattleAnswerNotShown(t *testing.T) { run(t, "AC-CMP-205") }
func TestBattleResume(t *testing.T)         { run(t, "AC-CMP-207") }
func TestWSReplay(t *testing.T)             { run(t, "AC-CMP-210") }
