<script lang="ts" generics="T">
  // A long list in a scrolling box that only renders the rows in view: rows
  // of a fixed height, columns items per row, or as many columns of at least
  // min px as the box fits. The box grows with its rows up to its max height
  // (given in class), then scrolls.
  import { untrack, type Snippet } from 'svelte'
  import { createVirtualizer } from '@tanstack/svelte-virtual'

  let {
    items,
    key,
    rowHeight,
    columns: fixed = 1,
    min,
    gap = 6,
    pad = 0,
    overscan = 6,
    label,
    class: klass = '',
    cell,
    empty,
  }: {
    items: readonly T[]
    key: (item: T) => string | number
    rowHeight: number
    columns?: number
    // a column's least width, for as many columns as fit
    min?: number
    gap?: number
    // the box's padding, when not given in class
    pad?: number
    overscan?: number
    label?: string
    class?: string
    cell: Snippet<[T, number]>
    empty?: Snippet
  } = $props()

  let scroller = $state<HTMLDivElement | null>(null)
  let width = $state(0)
  const columns = $derived(min ? Math.max(1, Math.floor((width - 2 * pad + gap) / (min + gap))) : fixed)
  const rows = $derived(Math.ceil(items.length / columns))

  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: () => rowHeight + gap,
    // svelte-ignore state_referenced_locally
    overscan,
  })
  $effect(() => {
    // read the dependencies here, update the store untracked (it would
    // rerun this effect)
    const count = rows
    const element = scroller
    const size = rowHeight + gap
    const padding = pad
    untrack(() => {
      $virtualizer.setOptions({ count, getScrollElement: () => element, estimateSize: () => size, paddingStart: padding, paddingEnd: padding })
      $virtualizer.measure()
    })
  })

  // whether the box is scrolled to its last row, and scrolling it there
  export function atEnd(): boolean {
    return !scroller || scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 8
  }
  export function scrollToEnd() {
    if (rows) $virtualizer.scrollToIndex(rows - 1, { align: 'end' })
  }
</script>

<div bind:this={scroller} bind:clientWidth={width} role="list" aria-label={label} class={['overflow-y-auto', klass]} style:padding-inline={pad ? `${pad}px` : undefined}>
  {#if items.length === 0}
    {#if pad}<div style:padding-block="{pad}px">{@render empty?.()}</div>{:else}{@render empty?.()}{/if}
  {:else}
    <div class="relative w-full" style:height="{$virtualizer.getTotalSize() - gap}px">
      {#each $virtualizer.getVirtualItems() as line (line.key)}
        {@const first = line.index * columns}
        <div
          class="absolute top-0 left-0 grid w-full"
          style:height="{rowHeight}px"
          style:transform="translateY({line.start}px)"
          style:grid-template-columns="repeat({columns}, minmax(0, 1fr))"
          style:column-gap="{gap}px"
        >
          {#each items.slice(first, first + columns) as item, i (key(item))}
            <div role="listitem" class="grid min-w-0">{@render cell(item, first + i)}</div>
          {/each}
        </div>
      {/each}
    </div>
  {/if}
</div>
