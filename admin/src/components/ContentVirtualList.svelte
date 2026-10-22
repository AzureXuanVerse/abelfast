<script lang="ts" generics="T">
  // A long list drawing only the rows in view, inside its own scroller
  // (class sets its height, e.g. max-h-[560px]). Rows may differ in
  // height: each is measured once drawn.
  import { untrack, type Snippet } from 'svelte'
  import { createVirtualizer } from '@tanstack/svelte-virtual'

  let {
    items,
    estimate,
    key,
    row,
    gap = 0,
    class: classes = '',
    label,
  }: {
    items: readonly T[]
    // a row's expected height in px
    estimate: number
    key: (item: T) => string | number
    row: Snippet<[T, number]>
    gap?: number
    class?: string
    label?: string
  } = $props()

  let scroller = $state<HTMLDivElement | null>(null)

  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    // svelte-ignore state_referenced_locally
    estimateSize: () => estimate,
    overscan: 6,
  })
  $effect(() => {
    // setOptions updates the store: read the inputs first, then update it
    // untracked, or the effect would rerun itself
    const list = items
    const element = scroller
    const keyOf = key
    const space = gap
    untrack(() => {
      $virtualizer.setOptions({ count: list.length, getScrollElement: () => element, getItemKey: (i) => keyOf(list[i]), gap: space })
      $virtualizer.measure()
    })
  })

  function measure(node: HTMLDivElement) {
    $virtualizer.measureElement(node)
  }
</script>

<div bind:this={scroller} class={['overflow-y-auto', classes]} role="list" aria-label={label}>
  <div class="relative w-full" style:height="{$virtualizer.getTotalSize()}px">
    {#each $virtualizer.getVirtualItems() as virtual (virtual.key)}
      {@const item = items[virtual.index]}
      {#if item !== undefined}
        <div role="listitem" data-index={virtual.index} use:measure class="absolute top-0 left-0 w-full" style:transform="translateY({virtual.start}px)">
          {@render row(item, virtual.index)}
        </div>
      {/if}
    {/each}
  </div>
</div>
