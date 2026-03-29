package utils

import "math"

// SecFromFloat 将秒的小数值四舍五入为整秒（用于入库与对外 JSON duration）。
func SecFromFloat(s float64) int {
	if s <= 0 {
		return 0
	}
	return int(math.Round(s))
}
