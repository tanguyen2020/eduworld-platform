// Package names generates fake Vietnamese full names (TEST_STRATEGY §5): a family name, a middle name and a given
// name drawn from short fixed lists. The lists are common words, not taken from any real roster; combined with the
// perf-* tenant they never identify a real child (AGENTS R16).
package names

import "math/rand/v2"

var (
	family = []string{"Nguyễn", "Trần", "Lê", "Phạm", "Hoàng", "Huỳnh", "Phan", "Vũ", "Võ", "Đặng", "Bùi", "Đỗ", "Hồ", "Ngô", "Dương", "Lý"}
	middle = []string{"Văn", "Thị", "Minh", "Ngọc", "Gia", "Bảo", "Thanh", "Đức", "Quỳnh", "Hữu", "Khánh", "Anh"}
	given  = []string{"An", "Bình", "Châu", "Dũng", "Giang", "Hà", "Hải", "Hạnh", "Hiếu", "Hoa", "Huy", "Khang", "Khoa", "Lan",
		"Linh", "Long", "Mai", "Minh", "Nam", "Ngân", "Nhi", "Phong", "Phúc", "Quân", "Sơn", "Tâm", "Thảo", "Trang", "Trí", "Tú",
		"Uyên", "Việt", "Vy", "Yến"}
)

// Full returns one fake full name from r.
func Full(r *rand.Rand) string {
	return family[r.IntN(len(family))] + " " + middle[r.IntN(len(middle))] + " " + given[r.IntN(len(given))]
}
