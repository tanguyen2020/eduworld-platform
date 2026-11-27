//go:build anticheat

package scenarios

import "testing"

// F132-AC-21 (TC-LRN-007, TC-CMP-007), F132-AC-22 (TC-LRN-008, TC-CMP-009): duplicate submits, client scores, one
// rating update per match, XP decay.
func TestIdempotentSubmit(t *testing.T)     { run(t, "AC-CMP-201") }
func TestSecondSubmitConflict(t *testing.T) { run(t, "AC-CMP-202") }
func TestClientScoreIgnored(t *testing.T)   { run(t, "AC-CMP-203") }
func TestMatchCountedOnce(t *testing.T)     { run(t, "AC-CMP-206") }
func TestXPDecay(t *testing.T)              { run(t, "AC-CMP-209") }
