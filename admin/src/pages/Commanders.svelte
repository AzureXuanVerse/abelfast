<script lang="ts">
  import { untrack } from 'svelte'
  import { MediaQuery } from 'svelte/reactivity'
  import { createQuery } from '@tanstack/svelte-query'
  import { createWindowVirtualizer } from '@tanstack/svelte-virtual'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import { rosterQuery } from '../lib/query'
  import { fuzzy } from '../lib/fuzzy'
  import { link } from '../lib/router.svelte'
  import { ago } from '../lib/format'
  import Page from '../components/Page.svelte'
  import Seg from '../components/Seg.svelte'

  const roster = createQuery(rosterQuery)
  const rows = $derived(roster.data)

  let query = $state('')
  let show = $state<'all' | 'online'>('all')
  // by level, highest first, or lowest first
  let order = $state<'desc' | 'asc'>('desc')
  const [debounced] = createDebouncedValue(() => query, { wait: 150 })

  // by level without a search, best match first with one
  const shown = $derived.by(() => {
    const kept = (rows ?? []).filter((row) => show === 'all' || row.online)
    if (debounced().trim()) return fuzzy(kept, debounced(), (row) => [row.name, row.id])
    return kept.sort((a, b) => (order === 'desc' ? b.level - a.level : a.level - b.level) || a.id - b.id)
  })

  // the cards, two a row on wide screens, rendered a row at a time
  const wide = new MediaQuery('min-width: 1024px')
  const columns = $derived(wide.current ? 2 : 1)
  const cardHeight = 120
  const gap = 14
  let grid = $state<HTMLElement>()
  let top = $state(0)
  $effect(() => {
    if (!grid) return
    const element = grid
    const measure = () => (top = element.getBoundingClientRect().top + scrollY)
    const observer = new ResizeObserver(measure)
    observer.observe(document.body)
    measure()
    return () => observer.disconnect()
  })
  const virtualizer = createWindowVirtualizer<HTMLDivElement>({
    count: 0,
    estimateSize: () => cardHeight + gap,
    overscan: 4,
  })
  $effect(() => {
    const count = Math.ceil(shown.length / columns)
    const margin = top
    untrack(() => {
      $virtualizer.setOptions({ count, scrollMargin: margin })
      $virtualizer.measure()
    })
  })
</script>

{#snippet actions()}
  <label class="relative flex items-center">
    <span class="sr-only">Find a commander</span>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
    <input type="search" class="ip-field w-[230px] pl-10" placeholder="Name or UID" bind:value={query} />
  </label>
  <Seg
    label="Show"
    bind:value={show}
    options={[
      { value: 'all', label: 'Everyone' },
      { value: 'online', label: 'Online' },
    ]}
  />
  <div class="ip-seg">
    <button type="button" aria-label={order === 'desc' ? 'Level, highest first' : 'Level, lowest first'} onclick={() => (order = order === 'desc' ? 'asc' : 'desc')}>
      Lv.
      <svg aria-hidden="true" viewBox="0 0 24 24" class={['size-4', order === 'asc' && 'rotate-180']} fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6" /></svg>
    </button>
  </div>
{/snippet}

<Page title="Commanders" sub="ROSTER" {actions}>
  <section aria-label="Commanders" class="pt-[18px] pr-10 pb-7 pl-16">
    {#if rows && shown.length === 0}
      <p class="ip-panel m-0 px-6 py-5 text-lg text-dim">No commander</p>
    {/if}
    <div bind:this={grid} class="relative w-full" style:height="{$virtualizer.getTotalSize()}px">
      {#each $virtualizer.getVirtualItems() as line (line.key)}
        <div class="absolute top-0 left-0 grid w-full grid-cols-1 gap-x-5 lg:grid-cols-2" style:transform="translateY({line.start - $virtualizer.options.scrollMargin}px)">
          {#each shown.slice(line.index * columns, line.index * columns + columns) as row (row.id)}
            <a href="/commanders/{row.id}" use:link class="ip-panel flex h-[120px] items-center gap-[18px] pr-[22px] pl-3.5 text-white no-underline hover:text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan),0_8px_24px_rgb(0_0_0/0.35)]">
              <div class="size-[92px] flex-none overflow-hidden rounded bg-[#151a26] shadow-[0_0_0_3px_#fff]">
                {#if row.icon}<img src="/api/assets/{row.icon}.png" alt="" loading="lazy" class="size-full object-cover" />{/if}
              </div>
              <span class="flex min-w-0 flex-1 flex-col gap-2">
                <span class="flex items-baseline gap-3">
                  <span class="truncate text-[26px] text-gold">{row.name}</span>
                  <span class="text-[15px] text-soft">UID:<span class="num text-lg">{row.id}</span></span>
                </span>
                <span class="flex items-center gap-2 text-base text-soft">
                  <span aria-hidden="true" class={['h-[26px] w-2 rounded-sm', row.online ? 'bg-ok' : 'bg-grey']}></span>
                  {row.online ? 'Online now' : row.last_online ? `Last seen ${ago(row.last_online)}` : 'Never played'}
                </span>
              </span>
              <span class="flex flex-col items-end leading-none">
                <span class="num text-5xl">{row.level}</span>
                <span class="ip-sub">LEVEL</span>
              </span>
            </a>
          {/each}
        </div>
      {/each}
    </div>
  </section>
</Page>
