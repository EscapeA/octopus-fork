package op

import (
	"github.com/lingyuins/octopus/internal/op/navorder"
)

// Deprecated: Use navorder.NormalizeNavOrder from internal/op/navorder instead.
func NormalizeNavOrder(raw string, defaults []string) []string {
	return navorder.NormalizeNavOrder(raw, defaults)
}
