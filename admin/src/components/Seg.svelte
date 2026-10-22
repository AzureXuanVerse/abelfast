<script lang="ts" generics="T">
  // One choice among a few: a charcoal bar with a white pill on dark
  // surfaces, white boxes with a navy selection (boxes) on light cards.
  let {
    options,
    value = $bindable(),
    label,
    boxes = false,
    columns = 0,
    onchange,
  }: {
    options: { value: T; label: string }[]
    value: T
    label: string
    boxes?: boolean
    // boxes only: a grid of this many columns, 0 for one row
    columns?: number
    onchange?: (value: T) => void
  } = $props()

  function pick(next: T) {
    value = next
    onchange?.(next)
  }
</script>

<div
  role="group"
  aria-label={label}
  class={boxes ? (columns ? 'grid gap-2' : 'flex flex-wrap gap-2') : 'ip-seg'}
  style:grid-template-columns={boxes && columns ? `repeat(${columns}, minmax(0, 1fr))` : undefined}
>
  {#each options as option (option.value)}
    <button type="button" class={boxes ? 'ip-opt' : ''} aria-pressed={option.value === value} onclick={() => pick(option.value)}>{option.label}</button>
  {/each}
</div>
