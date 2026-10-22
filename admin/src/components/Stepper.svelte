<script lang="ts">
  // A number with round white − / + buttons on a charcoal bar, kept in its
  // range (typed, stepped, and when the range moves).
  import { clamp as bound, uint32 } from '../lib/bounded'

  let {
    value = $bindable(),
    min = 0,
    max = uint32,
    step = 1,
    label,
  }: { value: number; min?: number; max?: number; step?: number; label: string } = $props()

  const clamp = (n: number) => bound(n, { min, max, step })
  $effect(() => {
    const fixed = clamp(value)
    if (fixed !== value) value = fixed
  })
</script>

<div class="flex h-[52px] items-center rounded-[26px] bg-strip px-1" role="group" aria-label={label}>
  <button type="button" aria-label="Less" class="grid size-11 cursor-pointer place-items-center rounded-full border-0 bg-white disabled:opacity-50" disabled={value <= min} onclick={() => (value = clamp(value - step))}>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="#3a3e47" stroke-width="3" stroke-linecap="round"><path d="M5 12h14" /></svg>
  </button>
  <input
    aria-label={label}
    inputmode="numeric"
    class="num h-10 w-0 min-w-16 flex-1 border-0 bg-transparent text-center text-[30px] text-white focus:outline-none"
    value={value}
    onchange={(event) => {
      value = clamp(Number(event.currentTarget.value) || min)
      event.currentTarget.value = String(value)
    }}
  />
  <button type="button" aria-label="More" class="grid size-11 cursor-pointer place-items-center rounded-full border-0 bg-white disabled:opacity-50" disabled={value >= max} onclick={() => (value = clamp(value + step))}>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="#3a3e47" stroke-width="3" stroke-linecap="round"><path d="M5 12h14M12 5v14" /></svg>
  </button>
</div>
