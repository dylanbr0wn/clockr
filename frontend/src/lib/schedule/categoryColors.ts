const CATEGORY_PALETTE = [
  { dot: "bg-emerald-500", bar: "bg-emerald-500" },
  { dot: "bg-rose-500", bar: "bg-rose-500" },
  { dot: "bg-sky-500", bar: "bg-sky-500" },
  { dot: "bg-violet-500", bar: "bg-violet-500" },
  { dot: "bg-amber-500", bar: "bg-amber-500" },
] as const;

function hashString(value: string) {
  let hash = 0;

  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }

  return hash;
}

export function categoryColor(category: string) {
  return CATEGORY_PALETTE[hashString(category) % CATEGORY_PALETTE.length];
}
