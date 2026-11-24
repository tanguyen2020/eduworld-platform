// SPDX-License-Identifier: MIT — EduWorld first-party asset (ADR-0102, assets/LICENSES.md).
// Soft inverted-hull outline, next pass of the toon presets (DESIGN_SYSTEM §5.3, F067). Baked meshes store smoothed
// normals in COLOR (`smooth_normals_in_color`) so hard low-poly edges do not crack the hull. The global
// `outline_fade_distance` comes from the quality preset (PERFORMANCE_BUDGET §5 "Outline toon", QualityPolicy):
// the outline thins out and disappears before that distance; 0 keeps it at every distance.
shader_type spatial;
render_mode unshaded, cull_front, depth_draw_opaque, shadows_disabled;

uniform vec4 outline_color : source_color;
uniform float outline_width;
uniform bool smooth_normals_in_color = false;
global uniform float outline_fade_distance;

void vertex() {
	vec3 n = smooth_normals_in_color ? normalize(COLOR.rgb * 2.0 - 1.0) : NORMAL;
	vec3 world = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
	float dist = length(world - CAMERA_POSITION_WORLD);
	// Grows a little with distance so far silhouettes stay readable, within [0.7, 2.2] × the token width.
	float width = outline_width * clamp(dist * 0.09, 0.7, 2.2);
	if (outline_fade_distance > 0.0) {
		width *= 1.0 - smoothstep(outline_fade_distance * 0.75, outline_fade_distance, dist);
	}
	VERTEX += n * width;
}

void fragment() {
	ALBEDO = outline_color.rgb;
}
