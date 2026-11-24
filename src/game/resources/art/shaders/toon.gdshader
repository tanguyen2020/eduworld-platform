// SPDX-License-Identifier: MIT — EduWorld first-party asset (ADR-0102, assets/LICENSES.md).
// Shared toon shader of the stylized low-poly art (DESIGN_SYSTEM §5.3, WORLD_RENDERING §6.1, F067).
// - Colour: baked meshes address slot floor(UV.x) of the art palette (`palette`, set from the art.palette tokens by the
//   generated preset); faces with UV.y > 1 take `albedo` instead (per-avatar skin / hair / outfit tint set by AvatarBuilder from the tokens).
// - Light: `toon_steps` soft bands from the lit side to `shadow_tint`, a light rim; no specular, no SSAO/SSR, so the
//   Mobile and Compatibility (Web) renderers look the same (PERFORMANCE_BUDGET §4).
// Every value comes from a generated material preset (resources/art/generated/mat_toon*.tres): no colour here (R32).
shader_type spatial;
render_mode cull_back, depth_draw_opaque, specular_disabled;

// A uniform array (converted from sRGB on the CPU like every source_color uniform) keeps the Mobile and the
// Compatibility (Web) renderer identical (renderer parity, PERFORMANCE_BUDGET §4); 64 slots, `palette_size` used.
uniform vec4 palette[64] : source_color;
uniform int palette_size;
uniform vec4 albedo : source_color;
uniform float toon_steps;
uniform vec4 shadow_tint : source_color;
uniform float rim_strength;

void fragment() {
	int slot = clamp(int(UV.x), 0, max(palette_size - 1, 0));
	ALBEDO = UV.y > 1.0 ? albedo.rgb : palette[slot].rgb;
	ROUGHNESS = 1.0;
	SPECULAR = 0.0;
}

void light() {
	float steps = max(toon_steps, 1.0);
	float lit = clamp(dot(NORMAL, LIGHT), 0.0, 1.0) * ATTENUATION;
	// Soft band edges: a short smoothstep around each step instead of a hard floor (the "soft" toon look).
	float scaled = lit * steps;
	float band = (floor(scaled) + smoothstep(0.42, 0.58, fract(scaled))) / steps;
	vec3 shade = mix(shadow_tint.rgb, vec3(1.0), clamp(band, 0.0, 1.0));
	float rim = pow(1.0 - clamp(dot(NORMAL, VIEW), 0.0, 1.0), 4.0) * rim_strength * smoothstep(0.0, 0.3, lit);
	DIFFUSE_LIGHT += LIGHT_COLOR * (shade + vec3(rim)) / PI;
}
