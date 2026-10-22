<script lang="ts" module>
  // A thing a Picker offers: its thumbnail (an API icon bundle path) on its
  // rarity frame, its name and a line under it.
  export interface PickerItem {
    id: number
    name: string
    icon?: string
    rarity?: number
    // ships' rarity starts at 2 (see tier)
    ship?: boolean
    sub?: string
    // more strings it's found by (e.g. the ship of a skin)
    search?: (string | number | undefined)[]
    // shown but not pickable
    disabled?: boolean
    // a skin with a Live2D model
    live2d?: boolean
  }

  // A filter the dialog offers over the items: one select listing the
  // values items have (hull type, faction, owned...).
  export interface PickerFacet {
    label: string
    value: (item: PickerItem) => string | undefined
  }
</script>

<script lang="ts">
  // Picks one thing out of many, as the game's own selection screens do: a
  // field showing the pick's thumbnail opens a dialog with a grid of
  // thumbnails, a fuzzy search and filters. The grid is virtualized, so
  // thousands of ships or skins stay fast. Keyboard: arrows move in the
  // grid, Enter picks, Escape closes. Without its field (trigger false) it
  // is opened by calling show().
  import { untrack } from 'svelte'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import { fuzzy } from '../lib/fuzzy'
  import Icon from './Icon.svelte'
  import Live2DBadge from './Live2DBadge.svelte'

  let {
    items,
    value = $bindable(null),
    label,
    placeholder = 'Search',
    loading = false,
    facets = [],
    trigger = true,
    onselect,
  }: {
    items: PickerItem[]
    value?: number | null
    label: string
    placeholder?: string
    loading?: boolean
    facets?: PickerFacet[]
    trigger?: boolean
    onselect?: (item: PickerItem) => void
  } = $props()

  const uid = $props.id()
  let dialog = $state<HTMLDialogElement>()
  let open = $state(false)
  let query = $state('')
  let active = $state(0)
  let input = $state<HTMLInputElement>()
  let scroller = $state<HTMLDivElement | null>(null)
  let gridWidth = $state(0)
  // the chosen value of each facet, '' for all
  let picked = $state<string[]>([])

  const [debounced] = createDebouncedValue(() => query, { wait: 120 })
  const selected = $derived(items.find((item) => item.id === value))
  const facetValues = $derived(facets.map((facet) => [...new Set(items.map(facet.value).filter((v): v is string => !!v))].sort()))
  const shown = $derived.by(() => {
    const kept = items.filter((item) => facets.every((facet, i) => !picked[i] || facet.value(item) === picked[i]))
    return fuzzy(kept, debounced(), (item) => [item.name, item.id, ...(item.search ?? [])])
  })

  // the grid: rows of cards, as many columns of at least minCard as fit,
  // stretched to fill the row (gridWidth includes the 2 x 20px padding)
  const minCard = 132
  const rowHeight = 176
  const gap = 12
  const inner = $derived(Math.max(0, gridWidth - 40))
  const columns = $derived(Math.max(1, Math.floor((inner + gap) / (minCard + gap))))
  const cardWidth = $derived(Math.floor((inner - gap * (columns - 1)) / columns))
  const rows = $derived(Math.ceil(shown.length / columns))

  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: () => rowHeight + gap,
    overscan: 3,
  })
  $effect(() => {
    // the virtualizer is a store updated by setOptions: track only the count
    // and the scroll element, or the effect would rerun itself
    const count = rows
    const element = scroller
    untrack(() => {
      $virtualizer.setOptions({ count, getScrollElement: () => element })
      $virtualizer.measure()
    })
  })
  $effect(() => {
    // a new search starts at its best match
    void shown
    active = 0
    scroller?.scrollTo({ top: 0 })
  })

  export function show() {
    query = ''
    picked = facets.map(() => '')
    open = true
    dialog?.showModal()
    // start on the current pick
    const at = shown.findIndex((item) => item.id === value)
    queueMicrotask(() => {
      input?.focus()
      if (at >= 0) {
        active = at
        $virtualizer.scrollToIndex(Math.floor(at / columns), { align: 'center' })
      }
    })
  }

  function close() {
    open = false
    dialog?.close()
  }

  function pick(item: PickerItem | undefined) {
    if (!item || item.disabled) return
    value = item.id
    close()
    onselect?.(item)
  }

  function move(step: number) {
    if (!shown.length) return
    active = Math.min(shown.length - 1, Math.max(0, active + step))
    $virtualizer.scrollToIndex(Math.floor(active / columns), { align: 'auto' })
  }

  function keydown(event: KeyboardEvent) {
    if (event.key === 'ArrowRight') move(1)
    else if (event.key === 'ArrowLeft') move(-1)
    else if (event.key === 'ArrowDown') move(columns)
    else if (event.key === 'ArrowUp') move(-columns)
    else if (event.key === 'Enter') pick(shown[active])
    else return
    event.preventDefault()
  }
</script>

{#if trigger}<button
  type="button"
  class="ip-field flex w-full cursor-pointer items-center gap-3 px-2 text-left"
  aria-haspopup="dialog"
  aria-expanded={open}
  aria-label={selected ? `${label}: ${selected.name}` : label}
  onclick={show}
>
  {#if selected}
    <Icon icon={selected.icon} rarity={selected.rarity} ship={selected.ship} size={34} name={selected.name} label={false} />
    <span class="min-w-0 flex-1 truncate">{selected.name}</span>
  {:else}
    <span class="flex-1 px-1.5 text-[#8a93a3]">{loading ? 'Loading…' : label}</span>
  {/if}
  <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M4 7h16M4 12h16M4 17h10" /></svg>
</button>{/if}

<dialog
  bind:this={dialog}
  aria-labelledby="{uid}-title"
  class="m-auto h-[min(760px,88vh)] w-[min(1080px,92vw)] overflow-hidden rounded border-0 bg-transparent p-0 text-white shadow-[0_16px_48px_rgb(0_0_0/0.55)] backdrop:bg-[rgb(8_10_18/0.7)]"
  onclose={() => (open = false)}
  onclick={(event) => event.target === dialog && close()}
>
  {#if open}
    <div class="flex h-full flex-col bg-[rgb(20_24_34/0.97)]">
      <div class="flex items-center gap-3 border-b-4 border-cyan bg-[#2b2e35] px-5 py-3">
        <h2 id="{uid}-title" class="m-0 rounded-[18px] bg-pill px-7 py-1 text-xl">{label}</h2>
        <span class="num ml-1 text-xl text-soft">{shown.length}</span>
        <button type="button" class="ml-auto grid size-10 cursor-pointer place-items-center rounded-full border-0 bg-white/10 text-white hover:bg-white/20" aria-label="Close" onclick={close}>
          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18" /></svg>
        </button>
      </div>

      <div class="flex flex-wrap items-center gap-3 px-5 py-3">
        <input
          bind:this={input}
          bind:value={query}
          class="ip-field min-w-0 flex-1"
          aria-label="Search"
          aria-controls="{uid}-grid"
          {placeholder}
          onkeydown={keydown}
        />
        {#each facets as facet, i (facet.label)}
          <select class="ip-field w-auto" aria-label={facet.label} bind:value={picked[i]}>
            <option value="">{facet.label}: all</option>
            {#each facetValues[i] as option (option)}
              <option value={option}>{option}</option>
            {/each}
          </select>
        {/each}
      </div>

      <div bind:this={scroller} bind:clientWidth={gridWidth} id="{uid}-grid" role="listbox" aria-label={label} class="min-h-0 flex-1 overflow-y-auto px-5 pb-5">
        {#if shown.length === 0}
          <p class="m-0 py-6 text-lg text-dim">{loading ? 'Loading…' : 'No match'}</p>
        {:else}
          <div class="relative w-full" style:height="{$virtualizer.getTotalSize()}px">
            {#each $virtualizer.getVirtualItems() as row (row.key)}
              <div class="absolute top-0 left-0 flex w-full" style:gap="{gap}px" style:transform="translateY({row.start}px)">
                {#each shown.slice(row.index * columns, row.index * columns + columns) as item, col (item.id)}
                  {@const index = row.index * columns + col}
                  <button
                    type="button"
                    role="option"
                    id="{uid}-{item.id}"
                    aria-selected={item.id === value}
                    aria-disabled={item.disabled || undefined}
                    tabindex="-1"
                    class={[
                      'flex shrink-0 flex-col items-center gap-2 rounded border-0 bg-white px-2 pt-3 pb-2 text-ink shadow-[0_4px_10px_rgb(0_0_0/0.3)]',
                      item.disabled ? 'cursor-not-allowed opacity-40' : 'cursor-pointer hover:bg-[#eef7fc]',
                      index === active && 'shadow-[inset_0_0_0_3px_var(--color-cyan)]',
                      item.id === value && 'bg-[#e2f5fd]',
                    ]}
                    style:width="{cardWidth}px"
                    style:height="{rowHeight}px"
                    onpointerenter={() => (active = index)}
                    onclick={() => pick(item)}
                  >
                    <span class="relative">
                      <Icon icon={item.icon} rarity={item.rarity} ship={item.ship} size={92} name={item.name} />
                      {#if item.live2d}<Live2DBadge />{/if}
                    </span>
                    <span class="line-clamp-2 w-full text-center text-[14px] leading-tight">{item.name}</span>
                    {#if item.sub}<span class="w-full truncate text-center text-[12px] text-[#6b7386]">{item.sub}</span>{/if}
                  </button>
                {/each}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</dialog>
