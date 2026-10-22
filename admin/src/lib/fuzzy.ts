// Fuzzy search: TanStack's match-sorter ranking (exact, prefix, word
// starts, acronyms, then letters in order), best matches first.

import { compareItems, rankItem, rankings } from '@tanstack/match-sorter-utils'

// fuzzy keeps the items whose text matches query, best first; an empty
// query keeps every item in its order. text gives the strings an item is
// found by (name, id, ...).
export function fuzzy<T>(items: readonly T[], query: string, text: (item: T) => (string | number | undefined)[]): T[] {
  const q = query.trim()
  if (!q) return items.slice()
  const ranked: { item: T; rank: ReturnType<typeof rankItem> }[] = []
  for (const item of items) {
    let best: ReturnType<typeof rankItem> | null = null
    for (const value of text(item)) {
      if (value === undefined || value === '') continue
      const rank = rankItem(String(value), q, { threshold: rankings.MATCHES })
      if (rank.passed && (!best || compareItems(rank, best) < 0)) best = rank
    }
    if (best) ranked.push({ item, rank: best })
  }
  ranked.sort((a, b) => compareItems(a.rank, b.rank))
  return ranked.map((entry) => entry.item)
}
