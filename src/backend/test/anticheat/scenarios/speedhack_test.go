//go:build anticheat

package scenarios

import "testing"

// F132-AC-24: speed hack in the hub → hub.player.corrected + eduworld_rt_speed_violations_total; the learning record
// does not change.
func TestSpeedHack(t *testing.T) { run(t, "AC-CMP-208") }
