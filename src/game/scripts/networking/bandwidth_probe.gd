class_name BandwidthProbe
extends RefCounted
## Measured download speed (F133 T037, F133-AC-20, PERFORMANCE_BUDGET §10.1 "bật tự động khi đo băng thông thấp"):
## every finished content download records its bytes and duration; the estimate is an exponential moving average
## (half old, half new) so one slow or fast transfer moves it quickly. Below `threshold_bps` (bits per second; project
## setting `eduworld/content/low_bandwidth_bps`, default = the minimum playable network of PERFORMANCE_BUDGET §10,
## 1 Mbps) the device is in low data mode. No sample yet = not low.

const SMOOTHING: float = 0.5
const MIN_SAMPLE_BYTES: int = 16 * 1024

var threshold_bps: int = 1_000_000
var estimate_bps: float = -1.0


func _init(threshold: int = -1) -> void:
	threshold_bps = threshold if threshold > 0 else int(ProjectSettings.get_setting(
			"eduworld/content/low_bandwidth_bps", 1_000_000))


## One finished transfer of `bytes` in `msec` milliseconds. Tiny transfers (< 16 KiB) say nothing about speed.
func record(bytes: int, msec: int) -> void:
	if bytes < MIN_SAMPLE_BYTES or msec <= 0:
		return
	var bps: float = float(bytes) * 8.0 * 1000.0 / float(msec)
	estimate_bps = bps if estimate_bps < 0.0 else estimate_bps * (1.0 - SMOOTHING) + bps * SMOOTHING


func is_low() -> bool:
	return estimate_bps >= 0.0 and estimate_bps < float(threshold_bps)
