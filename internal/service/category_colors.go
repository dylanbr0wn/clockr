package service

import (
	"fmt"
	"strings"
)

// CategoryPalette is the preset hex palette users pick from in settings.
var CategoryPalette = []string{
	"#0EA5E9", // sky
	"#10B981", // emerald
	"#14B8A6", // teal
	"#8B5CF6", // violet
	"#EC4899", // pink
	"#F59E0B", // amber
	"#EF4444", // red
	"#64748B", // slate
}

// DefaultCategoryColor is assigned when no color is specified.
const DefaultCategoryColor = "#64748B"

// ValidateCategoryColor reports whether color is one of the preset palette values.
func ValidateCategoryColor(color string) error {
	normalized := strings.ToUpper(strings.TrimSpace(color))
	for _, allowed := range CategoryPalette {
		if normalized == strings.ToUpper(allowed) {
			return nil
		}
	}
	return fmt.Errorf("invalid category color %q: must be a preset palette value", color)
}

// NormalizeCategoryColor uppercases a validated palette color for storage.
func NormalizeCategoryColor(color string) string {
	return strings.ToUpper(strings.TrimSpace(color))
}
