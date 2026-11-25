// SPDX-License-Identifier: MIT — EduWorld first-party asset (ADR-0102, assets/LICENSES.md).
// Blob shadow under a character where real-time sun shadows are off (Compatibility / Web, preset Low; F068, F067-3):
// a soft round dark spot on a flat quad. Colour and opacity come from the tokens `light.blob_shadow_color` /
// `light.blob_shadow_opacity` set by BlobShadow (R32); only features shared by the Mobile and Compatibility renderers.
shader_type spatial;
render_mode unshaded, blend_mix, depth_draw_never, cull_disabled, shadows_disabled;

uniform vec4 shadow_color : source_color;
uniform float opacity;

void fragment() {
	float d = length(UV * 2.0 - 1.0);
	ALBEDO = shadow_color.rgb;
	ALPHA = opacity * (1.0 - smoothstep(0.45, 1.0, d));
}
