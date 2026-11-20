package profile

// MergeSettings applies a JSON merge patch (RFC 7396) to the game settings: a null value removes the key, an object
// is merged recursively, anything else replaces the value. base is not modified.
func MergeSettings(base, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		switch pv := v.(type) {
		case nil:
			delete(out, k)
		case map[string]any:
			cur, _ := out[k].(map[string]any)
			out[k] = MergeSettings(cur, pv)
		default:
			out[k] = v
		}
	}
	return out
}
