<script lang="ts">
  // Dock's Skins view, every skin of the game as the game's wardrobe shows
  // them: the selected
  // skin's art over most of the screen, its name and a card over it (edit
  // an owned skin, grant one the commander lacks), and below a search,
  // filters and a carousel of the skins. The commander's ships of the
  // skin's group link to the dock.
  import { createQuery } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import type { Snippet } from 'svelte'
  import Carousel, { pickLinked } from '../../components/Carousel.svelte'
  import Seg from '../../components/Seg.svelte'
  import SkinArt from '../../components/SkinArt.svelte'
  import Live2DBadge from '../../components/Live2DBadge.svelte'
  import { createLive2D } from '../../lib/live2d.svelte'
  import { fuzzy } from '../../lib/fuzzy'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import type { Dock } from '../../lib/dock'
  import { link, param } from '../../lib/router.svelte'
  import type { CatalogSkin } from '../../components/SkinSelect.svelte'
  import { cardArt, drop, playerKeys, type PlayerWardrobe } from '../../lib/players'
  import { stamp } from '../../lib/format'

  // commander: the commander's name, switcher: the Ships / Skins switch
  let { id, commander, switcher }: { id: number; commander: string; switcher: Snippet } = $props()

  const wardrobe = createQuery(() => ({
    queryKey: playerKeys.wardrobe(id),
    queryFn: get<PlayerWardrobe>(`/commanders/${id}/wardrobe`),
  }))

  const catalog = createQuery(() => ({
    queryKey: keys.catalog('skins'),
    queryFn: get<CatalogSkin[]>('/catalog/skins'),
    staleTime: Infinity,
  }))

  const dock = createQuery(() => ({
    queryKey: keys.screen(id, 'dock'),
    queryFn: get<Dock>(`/commanders/${id}/dock`),
  }))

  // a skin of the game, with the commander's end time when owned
  interface Skin {
    id: number
    name: string
    ship: string
    group?: number
    icon?: string
    owned: boolean
    end_time: number
  }

  const now = Date.now() / 1000
  let show = $state<'all' | 'permanent' | 'timed'>('all')
  let ownedOnly = $state(false)
  let query = $state('')
  const [search] = createDebouncedValue(() => query, { wait: 150 })
  const skins = $derived.by((): Skin[] => {
    const mine = new Map((wardrobe.data?.skins ?? []).map((s) => [s.id, s]))
    const all: Skin[] = (catalog.data ?? []).map((c) => ({ id: c.id, name: c.name, ship: c.ship_name, group: c.group, icon: c.icon, owned: mine.has(c.id), end_time: mine.get(c.id)?.end_time ?? 0 }))
    // owned skins the catalog doesn't name still show
    const known = new Set(all.map((s) => s.id))
    for (const s of mine.values()) if (!known.has(s.id)) all.push({ ...s, owned: true })
    return all
  })
  const ownedCount = $derived(skins.filter((s) => s.owned).length)
  const shown = $derived(
    fuzzy(
      skins.filter((s) => (!ownedOnly || s.owned) && (show === 'all' || (s.owned && (show === 'permanent') === !s.end_time))),
      search(),
      (s) => [s.name, s.ship, s.id],
    ),
  )

  // the skin in the URL (?skin=, from the dock), the clicked one, else the
  // first one shown, so the stage is never empty
  const live2d = createLive2D()
  let selectedID = $state<number | null>(null)
  pickLinked(() => param('skin'), (linked) => (selectedID = linked))
  const selected = $derived(shown.find((s) => s.id === selectedID) ?? shown[0] ?? null)
  // the commander's ships that can wear it (ship ids are group * 10 + n)
  const ships = $derived(selected?.group ? (dock.data?.ships ?? []).filter((ship) => Math.floor(ship.template_id / 10) === selected!.group) : [])
  const painting = $derived(selected?.icon?.split('/').pop() ?? '')

  // the selected skin's end, as a datetime-local value, '' for good
  const local = (unix: number) => stamp(unix).replace(' ', 'T')
  // the field starts at the skin's end, again after a save
  const skinID = $derived(selected?.id ?? 0)
  const end = $derived(selected?.end_time ?? 0)
  let ends = $derived.by(() => {
    void skinID
    return end ? local(end) : ''
  })

  const tag = (s: Skin) => (!s.owned ? 'NOT OWNED' : s.end_time ? (s.end_time > now ? `UNTIL ${stamp(s.end_time).slice(5, 10)}` : 'EXPIRED') : 'PERMANENT')
  const tagColor = (s: Skin) => (!s.owned ? 'bg-strip' : s.end_time ? (s.end_time > now ? 'bg-warn' : 'bg-red') : 'bg-charcoal/85')

  const { mutation: write } = createCommanderWrite(() => id)

  function saveSkin(endTime: number, commander: string) {
    if (!selected) return
    const what = endTime ? `${selected.name} until ${stamp(endTime)}` : `${selected.name} made permanent`
    write.mutate({ method: 'PUT', path: `/skins/${selected.id}`, body: { end_time: endTime }, done: what, commander })
  }

  // granting the selected skin: permanent, or for some days
  let duration = $state<'permanent' | '7' | '30'>('permanent')
  function grantSkin(commander: string) {
    if (!selected) return
    const timed = duration !== 'permanent'
    const drops = [timed ? { type: drop.skinTimed, id: selected.id, count: Number(duration) * 86400 } : { type: drop.skin, id: selected.id, count: 1 }]
    const what = timed ? `${selected.name} for ${duration} days` : `${selected.name} granted`
    write.mutate({ path: '/grant', body: { drops }, done: what, commander })
  }

  function removeSkin(commander: string) {
    if (!selected || !confirm(`Take ${selected.name} away?`)) return
    write.mutate({ method: 'DELETE', path: `/skins/${selected.id}`, done: `${selected.name} removed`, commander })
  }
</script>

<section aria-label="Skins" class="flex flex-col gap-3.5">
  {#if selected && painting}
    <SkinArt {painting} name={selected.name} class="h-[720px]">
      <!-- keeps the name readable over light art -->
      <div aria-hidden="true" class="pointer-events-none absolute inset-x-0 bottom-0 h-[300px] bg-[linear-gradient(0deg,rgb(12_15_24/0.8)_0%,rgb(12_15_24/0.45)_50%,transparent_100%)]"></div>
      <div class="absolute bottom-[22px] left-6 flex max-w-[420px] flex-col gap-1.5">
        <span class="text-lg text-soft [text-shadow:0_1px_2px_rgb(0_0_0/0.9),0_2px_12px_rgb(0_0_0/0.75)]">{selected.ship}</span>
        <h2 class="m-0 line-clamp-2 text-[34px] leading-tight [text-shadow:0_1px_2px_rgb(0_0_0/0.9),0_2px_12px_rgb(0_0_0/0.75)]" title={selected.name}>{selected.name || `#${selected.id}`}</h2>
        <span><span class={['ip-tag text-white', tagColor(selected)]}>{tag(selected)}</span></span>
        {#if ships.length}
          <div class="mt-1.5 flex flex-wrap gap-2">
            {#each ships as ship (ship.id)}
              <a
                href="/commanders/{id}/dock?ship={ship.id}"
                use:link
                class="flex items-center gap-2 rounded-3xl bg-[rgb(30_33_40/0.88)] py-1 pr-3.5 pl-1 text-[15px] text-white no-underline hover:bg-[rgb(50_54_64/0.95)]"
              >
                {#if ship.square_icon}<img src="/api/assets/{ship.square_icon}.png" alt="" class="size-8 rounded-full object-cover" />{/if}
                <span class="num text-lg">Lv.{ship.level}</span>
                {ship.skin_id === selected.id ? 'Wearing it' : 'In the dock'}
              </a>
            {/each}
          </div>
        {/if}
      </div>
      {#if selected.owned}
        <div class="absolute right-5 bottom-5 flex w-[340px] flex-col gap-3 rounded bg-[rgb(20_24_34/0.86)] px-5 py-[18px] shadow-[0_8px_24px_rgb(0_0_0/0.4)]">
          <label class="flex flex-col gap-1.5 text-[15px] text-soft">Ends at <input class="ip-field num text-xl" type="datetime-local" bind:value={ends} /></label>
          <button type="button" class="ip-btn2 h-[46px] text-[19px]" disabled={write.isPending || !selected.end_time} onclick={() => saveSkin(0, commander)}>Make permanent</button>
          <div class="grid grid-cols-2 gap-2.5">
            <button type="button" class="ip-danger h-[46px] text-[19px]" disabled={write.isPending} onclick={() => removeSkin(commander)}>Remove</button>
            <button
              type="button"
              class="ip-btn h-[46px] text-[19px]"
              disabled={write.isPending || !ends || (!!selected.end_time && ends === local(selected.end_time))}
              onclick={() => saveSkin(Math.round(new Date(ends).getTime() / 1000), commander)}>Save</button
            >
          </div>
        </div>
      {:else}
        <div class="absolute right-5 bottom-5 flex w-[340px] flex-col gap-3 rounded bg-[rgb(20_24_34/0.86)] px-5 py-[18px] shadow-[0_8px_24px_rgb(0_0_0/0.4)]">
          <div role="group" aria-label="Duration" class="grid grid-cols-[1.3fr_1fr_1fr] gap-2">
            {#each [{ value: 'permanent', label: 'Permanent' }, { value: '7', label: '7 days' }, { value: '30', label: '30 days' }] as option (option.value)}
              <button type="button" class="ip-opt px-2 text-base" aria-pressed={duration === option.value} onclick={() => (duration = option.value as typeof duration)}>{option.label}</button>
            {/each}
          </div>
          <button type="button" class="ip-btn h-[46px] text-[19px]" disabled={write.isPending} onclick={() => grantSkin(commander)}>Grant to {commander}</button>
        </div>
      {/if}
    </SkinArt>
  {:else}
    <div class="ip-panel grid h-[720px] place-items-center text-lg text-dim">
      {#if wardrobe.error || catalog.error}<span class="text-bad">{(wardrobe.error ?? catalog.error)?.message}</span>{:else if wardrobe.data && catalog.data}No skin{:else}Loading…{/if}
    </div>
  {/if}

  <div class="flex flex-wrap items-center gap-3">
    {@render switcher()}
    <label class="relative flex items-center">
      <span class="sr-only">Find a skin</span>
      <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
      <input type="search" class="ip-field w-[260px] pl-10" placeholder="Skin or ship" bind:value={query} />
    </label>
    <Seg
      label="Show"
      bind:value={show}
      options={[
        { value: 'all', label: 'All' },
        { value: 'permanent', label: 'Permanent' },
        { value: 'timed', label: 'Timed' },
      ]}
    />
    <label class="flex h-12 cursor-pointer items-center gap-2.5 rounded-3xl bg-[rgb(30_33_40/0.88)] px-4 text-base">
      <input type="checkbox" class="size-5 accent-cyan" bind:checked={ownedOnly} />
      Owned
    </label>
    <span class="ml-auto text-base text-soft"
      ><span class="num text-2xl text-white">{shown.length}</span> shown · <span class="num text-2xl text-white">{ownedCount}</span> owned</span
    >
  </div>

  <Carousel
    items={shown}
    key={(skin) => skin.id}
    bind:selected={selectedID}
    label="Skins"
    width={120}
    height={170}
    empty={wardrobe.data ? 'No match' : ''}
    cardLabel={(skin) => `${skin.ship}: ${skin.name}${skin.owned ? '' : ', not owned'}`}
    cardClass="flex h-full cursor-pointer flex-col overflow-hidden rounded-[3px] border-0 bg-card p-0 text-left text-ink shadow-[0_2px_8px_rgb(0_0_0/0.3)]"
    selectedClass="shadow-[0_0_0_3px_var(--color-cyan),0_2px_8px_rgb(0_0_0/0.3)]"
  >
    {#snippet card(skin)}
      <span class="relative block h-[120px] w-full flex-none bg-[#1b2238]">
        {#if skin.icon}<img src="/api/assets/{cardArt(skin.icon)}.png" alt="" loading="lazy" class={['size-full object-cover object-top', !skin.owned && 'opacity-55 grayscale-[0.4]']} />{/if}
        {#if live2d.has(skin.icon)}<Live2DBadge />{/if}
        {#if skin.owned}<span class={['ip-tag absolute bottom-1 left-1 text-white', tagColor(skin)]}>{skin.end_time ? tag(skin) : 'PERM.'}</span>{/if}
      </span>
      <span class="flex flex-col px-2 pt-1.5 pb-2 leading-tight">
        <span class="truncate text-[11px] text-[#6b7386]">{skin.ship}</span>
        <span class="line-clamp-2 h-8 text-[13px]">{skin.name || `#${skin.id}`}</span>
      </span>
    {/snippet}
  </Carousel>
</section>
