shader_type canvas_item;
// Greyscale of a 2D picture (F069-AC-23, INVENTORY_ECONOMY §6, STUDENT_GAME_UX_SOCIAL §4): a collection item the
// student does not own yet is the SAME picture drawn through this shader (no second texture), always with the words
// "Chưa có" beside it — never colour alone (NFR-A11Y-001). Rec. 709 luma, lightened so the outline stays readable.

uniform float lift : hint_range(0.0, 1.0) = 0.25;

void fragment() {
	vec4 c = COLOR;
	float luma = dot(c.rgb, vec3(0.2126, 0.7152, 0.0722));
	COLOR = vec4(vec3(mix(luma, 1.0, lift)), c.a);
}
