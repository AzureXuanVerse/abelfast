// bounded keeps a number field inside its range: on change (typed past it,
// pasted, emptied) and when its range moves (an EXP field when its level
// changes), the value is clamped, rounded to its step, and an input event
// sent so bind:value follows. The browser's min/max only bound the arrows.
export interface Bounds {
  min?: number
  max?: number
  // 1 (whole numbers) unless given, e.g. 0.01 for affinity
  step?: number
}

// the API's counts are uint32
export const uint32 = 4294967295

export function clamp(value: number, { min = 0, max = uint32, step = 1 }: Bounds): number {
  const stepped = Math.round((Number(value) || 0) / step) * step
  return Math.min(Math.max(min, max), Math.max(min, Number(stepped.toFixed(6))))
}

export function bounded(node: HTMLInputElement, bounds: Bounds) {
  let current = bounds
  const fix = () => {
    if (node.value === '' && node.required) return
    const value = clamp(Number(node.value), current)
    if (String(value) === node.value) return
    node.value = String(value)
    node.dispatchEvent(new Event('input', { bubbles: true }))
  }
  node.addEventListener('change', fix)
  return {
    update(next: Bounds) {
      current = next
      if (node.value !== '') fix()
    },
    destroy: () => node.removeEventListener('change', fix),
  }
}
