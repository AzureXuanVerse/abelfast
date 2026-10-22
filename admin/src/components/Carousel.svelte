<script module lang="ts">
  import { untrack } from 'svelte'

  // pickLinked selects the id the URL links to (?ship=7) each time it
  // changes; called by the page before anything reads the selection.
  export function pickLinked(linked: () => number | null | undefined, pick: (id: number) => void) {
    $effect(() => {
      const id = linked()
      if (id) untrack(() => pick(id))
    })
  }
</script>

<script lang="ts" generics="T">
  // A row of cards picking one, as the dock's ships and skins: a horizontal
  // virtual list between two paging arrows. The arrow keys move the pick,
  // and a new list starts at its beginning, or with the picked card in view.
  import type { Snippet } from 'svelte'
  import { createVirtualizer } from '@tanstack/svelte-virtual'

  let {
    items,
    key,
    selected = $bindable(),
    label,
    width,
    height,
    empty = '',
    cardLabel,
    cardClass,
    selectedClass,
    card,
  }: {
    items: readonly T[]
    key: (item: T) => number
    // the id picked; the first card counts as picked when none is
    selected: number | null
    // the list's name, e.g. Ships ("Previous ships")
    label: string
    // a card's size in px
    width: number
    height: number
    // shown when there is no card
    empty?: string
    cardLabel: (item: T) => string
    cardClass: string
    selectedClass: string
    card: Snippet<[T]>
  } = $props()

  const gap = 12
  const current = $derived(items.find((item) => key(item) === selected) ?? items[0])
  const currentID = $derived(current === undefined ? null : key(current))

  let strip = $state<HTMLDivElement | null>(null)
  const carousel = createVirtualizer<HTMLDivElement, HTMLButtonElement>({
    horizontal: true,
    count: 0,
    getScrollElement: () => strip,
    // svelte-ignore state_referenced_locally
    estimateSize: () => width + gap,
    overscan: 6,
  })
  $effect(() => {
    // setOptions updates the store: track only the count and the element
    const count = items.length
    const element = strip
    untrack(() => {
      $carousel.setOptions({ count, getScrollElement: () => element })
      $carousel.measure()
    })
  })
  // a new search or filter starts at the beginning, a linked card in view
  $effect(() => {
    void items
    untrack(() => {
      const at = items.findIndex((item) => key(item) === selected)
      if (at > 0) queueMicrotask(() => $carousel.scrollToIndex(at, { align: 'center' }))
      else strip?.scrollTo({ left: 0 })
    })
  })

  function page(step: number) {
    strip?.scrollBy({ left: step * (strip.clientWidth - width), behavior: 'smooth' })
  }

  // arrows move the selection along the carousel
  function keydown(event: KeyboardEvent) {
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return
    event.preventDefault()
    const at = Math.max(0, items.findIndex((item) => key(item) === currentID))
    const next = Math.min(items.length - 1, Math.max(0, at + (event.key === 'ArrowRight' ? 1 : -1)))
    selected = items[next] === undefined ? null : key(items[next])
    $carousel.scrollToIndex(next, { align: 'auto' })
    queueMicrotask(() => strip?.querySelector<HTMLButtonElement>('[aria-selected="true"]')?.focus())
  }
</script>

<div class="ip-panel flex items-center gap-2.5 px-3 py-3.5">
  <button type="button" aria-label="Previous {label.toLowerCase()}" class="grid size-10 flex-none cursor-pointer place-items-center rounded-full border-0 bg-white/10 text-white hover:bg-white/20" onclick={() => page(-1)}>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M15 5l-7 7 7 7" /></svg>
  </button>
  <div bind:this={strip} role="listbox" aria-label={label} aria-orientation="horizontal" tabindex="-1" class="min-w-0 flex-1 overflow-x-auto py-1 [scrollbar-width:thin]" onkeydown={keydown}>
    {#if items.length === 0}
      <p class="m-0 content-center px-2 text-lg text-dim" style:height="{height}px">{empty}</p>
    {:else}
      <div class="relative" style:height="{height}px" style:width="{$carousel.getTotalSize()}px">
        {#each $carousel.getVirtualItems() as virtual (virtual.key)}
          {@const item = items[virtual.index]}
          {#if item !== undefined}
            {@const id = key(item)}
            <button
              type="button"
              role="option"
              aria-selected={currentID === id}
              aria-label={cardLabel(item)}
              tabindex={currentID === id ? 0 : -1}
              class={['absolute top-0 left-0', cardClass, currentID === id && selectedClass]}
              style:width="{width}px"
              style:transform="translateX({virtual.start}px)"
              onclick={() => (selected = id)}
            >
              {@render card(item)}
            </button>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
  <button type="button" aria-label="Next {label.toLowerCase()}" class="grid size-10 flex-none cursor-pointer place-items-center rounded-full border-0 bg-white/10 text-white hover:bg-white/20" onclick={() => page(1)}>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M9 5l7 7-7 7" /></svg>
  </button>
</div>
