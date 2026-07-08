export const CATEGORY_PALETTE = [
  "#0EA5E9",
  "#10B981",
  "#14B8A6",
  "#8B5CF6",
  "#EC4899",
  "#F59E0B",
  "#EF4444",
  "#64748B",
] as const;

export type CategoryPaletteColor = (typeof CATEGORY_PALETTE)[number];

export const DEFAULT_CATEGORY_COLOR: CategoryPaletteColor = "#64748B";

export function isCategoryPaletteColor(
  color: string,
): color is CategoryPaletteColor {
  return CATEGORY_PALETTE.includes(color.toUpperCase() as CategoryPaletteColor);
}

export function categoryStatColor(
  categoryName: string,
  categoryColors: Record<string, string>,
): string {
  const color = categoryColors[categoryName];
  return color && isCategoryPaletteColor(color)
    ? color.toUpperCase()
    : DEFAULT_CATEGORY_COLOR;
}

export function categoryColorStyle(color: string | undefined) {
  const resolved = color && isCategoryPaletteColor(color)
    ? color.toUpperCase()
    : DEFAULT_CATEGORY_COLOR;

  return {
    borderColor: resolved,
    backgroundColor: `${resolved}26`,
    color: "var(--foreground)",
  } as const;
}
