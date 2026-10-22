<script lang="ts">
  // Combat's Operation Siren section: the world's state, its maps, its
  // action points and its reset, then the details from /world: the
  // activation, the inventory, the counters, the achievements and the ports.
  import { createQuery } from '@tanstack/svelte-query'
  import Icon from '../../../components/Icon.svelte'
  import Picker, { type PickerItem } from '../../../components/Picker.svelte'
  import VirtualGrid from '../../../components/VirtualGrid.svelte'
  import Seg from '../../../components/Seg.svelte'
  import { bounded } from '../../../lib/bounded'
  import { count, day, short } from '../../../lib/format'
  import { InlineEdit } from '../../../lib/inline.svelte'
  import { createCommanderWrite, get, keys } from '../../../lib/query'
  import type { World } from '../../../lib/social'
  import type { WorldActivationShip, WorldDetail, WorldEntrance, WorldItem } from '../../../lib/world'

  let { id, world }: { id: number; world: World } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'world'),
    queryFn: get<WorldDetail>(`/commanders/${id}/world`),
  }))
  const detail = $derived(query.data)

  const { mutation: edit, run } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)

  // a number edited in place
  const inline = new InlineEdit()

  function resetWorld() {
    if (confirm('Reset Operation Siren now? The world inventory turns into vouchers and the maps start over.')) run('/world/reset', 'Operation Siren reset')
  }
  function saveStamina(event: SubmitEvent) {
    inline.save(event, world.stamina, (v) => run('/world/stamina', `Action points set to ${v}`, { stamina: v }))
  }

  // Inventory
  let itemPicker = $state<Picker>()
  // an item picked from the catalog, edited before it's in the inventory
  let adding = $state<WorldItem | null>(null)
  const catalogItems = $derived<PickerItem[]>(
    (detail?.catalog ?? []).map((item) => ({ id: item.id, name: item.name || `#${item.id}`, icon: item.icon, rarity: item.rarity, sub: `#${item.id}`, disabled: detail?.items.some((owned) => owned.id === item.id) })),
  )
  function pickItem(picked: PickerItem) {
    adding = detail?.catalog.find((item) => item.id === picked.id) ?? null
    if (adding) inline.start(`item:${adding.id}`, 1)
  }
  function saveItem(event: SubmitEvent, item: WorldItem) {
    adding = null
    inline.save(event, item.count, (v) => run(`/world/items/${item.id}`, v ? `${item.name} set to ${count(v)}` : `${item.name} removed`, { count: v }, 'PUT'))
  }

  // Counters
  function saveCounter(event: SubmitEvent, counter: string, label: string, current: number) {
    inline.save(event, current, (v) => run(`/world/counters/${counter}`, `${label} set to ${count(v)}`, { value: v }))
  }
  const resetCounter = (counter: string, label: string) => run(`/world/counters/${counter}/reset`, `${label} reset`)

  // Achievements
  let region = $state(0)
  const regionID = $derived(detail?.regions.some((r) => r.id === region) ? region : (detail?.regions[0]?.id ?? 0))
  const shownRegion = $derived(detail?.regions.find((r) => r.id === regionID))
  let open = $state<number | null>(null)
  function toggleStar(entrance: WorldEntrance, star: number, claimed: boolean) {
    run(`/world/entrances/${entrance.id}/stars/${star}`, claimed ? `${entrance.name} star ${star} unclaimed` : `${entrance.name} star ${star} claimed`, undefined, claimed ? 'DELETE' : 'POST')
  }

  // Activation: the fleets picked, vanguard and main apart
  interface Pick {
    vanguard: number[]
    main: number[]
    submarine: number[]
    commanders: { pos: number; id: number }[] | null
  }
  let fleets = $state<Pick[]>([])
  let submarine = $state<Pick>({ vanguard: [], main: [], submarine: [], commanders: null })
  const activation = $derived(detail && !detail.activated ? detail.activation : undefined)
  const shipByID = $derived(new Map((activation?.ships ?? []).map((ship) => [ship.id, ship])))
  const activationKey = $derived(activation ? `${id}:${activation.fleets}:${activation.defaults.map((f) => f.ships.join(',')).join('|')}` : '')
  $effect(() => {
    if (!activationKey || !activation) return
    const split = (ships: number[], commanders: Pick['commanders']): Pick => {
      const team = (t: string) => ships.filter((s) => shipByID.get(s)?.team === t).slice(0, 3)
      return { vanguard: team('vanguard'), main: team('main'), submarine: team('submarine'), commanders }
    }
    fleets = Array.from({ length: activation.fleets }, (_, i) => split(activation.defaults[i]?.ships ?? [], activation.defaults[i]?.commanders ?? null))
    submarine = split(activation.sub_defaults?.ships ?? [], activation.sub_defaults?.commanders ?? null)
  })
  let shipPicker = $state<Picker>()
  let picking = $state<{ fleet: number; team: 'vanguard' | 'main' | 'submarine'; index: number } | null>(null)
  const used = $derived(new Set([...fleets.flatMap((f) => [...f.vanguard, ...f.main]), ...submarine.submarine]))
  const candidates = $derived<PickerItem[]>(
    picking
      ? (activation?.ships ?? [])
          .filter((ship) => ship.team === picking!.team)
          .sort((a, b) => b.level - a.level || b.rarity - a.rarity)
          .map((ship) => ({ id: ship.id, name: ship.name, icon: ship.icon, rarity: ship.rarity, ship: true, sub: `Lv.${ship.level}`, disabled: used.has(ship.id) }))
      : [],
  )
  const fleetOf = (fleet: number) => (fleet < 0 ? submarine : fleets[fleet])
  function pickSlot(fleet: number, team: 'vanguard' | 'main' | 'submarine', index: number) {
    picking = { fleet, team, index }
    queueMicrotask(() => shipPicker?.show())
  }
  function pickShip(item: PickerItem) {
    if (!picking) return
    const row = fleetOf(picking.fleet)[picking.team]
    if (picking.index < row.length) row[picking.index] = item.id
    else row.push(item.id)
    picking = null
  }
  function removeSlot(fleet: number, team: 'vanguard' | 'main' | 'submarine', index: number) {
    fleetOf(fleet)[team].splice(index, 1)
  }
  const levels = (pick: Pick) => [...pick.vanguard, ...pick.main].reduce((sum, ship) => sum + (shipByID.get(ship)?.level ?? 0), 0)
  const fleetsValid = $derived(fleets.length > 0 && fleets.every((f) => f.vanguard.length > 0 && f.main.length > 0) && (!activation?.total_level || fleets.some((f) => levels(f) > activation.total_level)))
  function activate() {
    run('/world/activate', 'Operation Siren activated', {
      fleets: fleets.map((f) => ({ ships: [...f.main, ...f.vanguard], commanders: f.commanders })),
      submarine: submarine.submarine.length ? { ships: submarine.submarine, commanders: submarine.commanders } : null,
    })
  }
</script>

{#snippet numberForm(key: string, label: string, max: number, onsubmit: (event: SubmitEvent) => void)}
  <form class="ip-row flex flex-col gap-1.5 px-4 py-2.5" {onsubmit}>
    <span class="ip-sub">{label}</span>
    <span class="flex items-center gap-1.5">
      <!-- svelte-ignore a11y_autofocus -->
      <input class="ip-field num h-9 min-w-0 flex-1 text-lg" type="number" aria-label={label} autofocus bind:value={inline.value} use:bounded={{ min: 0, max }} onkeydown={inline.escape} />
      <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
    </span>
  </form>
{/snippet}

{#snippet counterTile(key: string, label: string, current: number, max: number, shown: string, reset: boolean)}
  {#if inline.editing === key}
    {@render numberForm(key, label, max, (e) => saveCounter(e, key, label, current))}
  {:else}
    <div class="ip-row relative flex hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]">
      <button type="button" aria-label="Edit {label}" disabled={busy} class="flex flex-1 cursor-pointer flex-col gap-1 border-0 bg-transparent px-4 py-2.5 text-left text-white" onclick={() => inline.start(key, current)}>
        <span class="ip-sub">{label}</span>
        <span class="num text-2xl">{shown}</span>
      </button>
      {#if reset}<button type="button" class="ip-btn2 ip-sm absolute top-2 right-2 h-8 px-2.5 text-sm" disabled={busy || current === 0} onclick={() => resetCounter(key, label)}>Reset</button>{/if}
    </div>
  {/if}
{/snippet}

{#snippet resetTile(key: string, label: string, shown: string, sub: string, empty: boolean)}
  <div class="ip-row relative flex flex-col gap-1 px-4 py-2.5">
    <span class="ip-sub">{label}</span>
    <span class="num text-2xl">{shown}</span>
    {#if sub}<span class="text-[13px] text-dim">{sub}</span>{/if}
    <button type="button" class="ip-btn2 ip-sm absolute top-2 right-2 h-8 px-2.5 text-sm" disabled={busy || empty} onclick={() => resetCounter(key, label)}>Reset</button>
  </div>
{/snippet}

{#snippet itemRow(item: WorldItem)}
  {#if inline.editing === `item:${item.id}`}
    <form class="ip-row flex items-center gap-3 px-2 py-1.5" onsubmit={(e) => saveItem(e, item)}>
      <Icon icon={item.icon} rarity={item.rarity} size={40} name={item.name} label={false} />
      <span class="min-w-0 flex-1 truncate text-[16px]">{item.name || `#${item.id}`}</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="ip-field num h-9 w-28 text-right text-lg" type="number" aria-label="{item.name} count" autofocus bind:value={inline.value} use:bounded={{ min: 0, max: detail?.count_max }} onkeydown={(e) => e.key === 'Escape' && (inline.stop(), (adding = null))} />
      <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
      {#if item.count}<button type="button" class="ip-danger ip-sm h-9" disabled={busy} onclick={() => (inline.stop(), run(`/world/items/${item.id}`, `${item.name} removed`, { count: 0 }, 'PUT'))}>Remove</button>{/if}
    </form>
  {:else}
    <button type="button" aria-label="Edit {item.name}" disabled={busy} class="ip-row flex cursor-pointer items-center gap-3 border-0 px-2 py-1.5 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start(`item:${item.id}`, item.count)}>
      <Icon icon={item.icon} rarity={item.rarity} size={40} name={item.name} label={false} />
      <span class="min-w-0 flex-1 truncate text-[16px]">{item.name || `#${item.id}`}</span>
      <span class="num text-xl">{count(item.count)}</span>
    </button>
  {/if}
{/snippet}

{#snippet slot(fleet: number, team: 'vanguard' | 'main' | 'submarine', index: number)}
  {@const shipID = fleetOf(fleet)[team][index]}
  {@const ship: WorldActivationShip | undefined = shipID ? shipByID.get(shipID) : undefined}
  <span class="relative">
    {#if ship}
      <button type="button" aria-label="{ship.name}, level {ship.level}" class="cursor-pointer rounded border-0 bg-transparent p-0 hover:shadow-[0_0_0_2px_var(--color-cyan)]" onclick={() => pickSlot(fleet, team, index)}>
        <Icon icon={ship.icon} rarity={ship.rarity} ship count={ship.level} size={64} name={ship.name} />
      </button>
      <button type="button" aria-label="Remove {ship.name}" class="absolute -top-2 -right-2 grid size-6 cursor-pointer place-items-center rounded-full border-0 bg-[#6b7386] text-sm text-white hover:bg-red" onclick={() => removeSlot(fleet, team, index)}>×</button>
    {:else}
      <button type="button" aria-label="Add a {team} ship" disabled={index > fleetOf(fleet)[team].length} class="grid size-16 cursor-pointer place-items-center rounded border-2 border-dashed border-white/35 bg-white/6 text-white/70 hover:border-cyan hover:text-white disabled:cursor-default disabled:opacity-40" onclick={() => pickSlot(fleet, team, fleetOf(fleet)[team].length)}>
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-7" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
      </button>
    {/if}
  </span>
{/snippet}

{#snippet star(filled: boolean)}
  <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill={filled ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="2" stroke-linejoin="round"><path d="M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z" /></svg>
{/snippet}

<section aria-label="Operation Siren" class="ip-panel">
  <div class="ip-head items-center">
    <h2>Operation Siren</h2>
    <span class="text-[15px] text-soft"><span class="num text-xl text-white">{world.maps.length}</span> maps entered</span>
    <button type="button" class="ip-danger ip-sm ml-auto" disabled={busy || !world.resettable} onclick={resetWorld}>Reset</button>
  </div>
  <div class="grid grid-cols-4 gap-3 px-5 pt-4">
    <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
      <span class="ip-sub">State</span>
      <span class={['text-lg', world.activated ? 'text-ok' : 'text-dim']}>{world.activated ? 'Active' : world.started ? 'Inactive' : 'Not started'}</span>
    </div>
    <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
      <span class="ip-sub">Progress</span>
      <span><span class="num text-2xl">{world.progress}</span><span class="ml-2 text-sm text-dim">reset at <span class="num">{world.reset_stage}</span></span></span>
    </div>
    {#if inline.editing === 'stamina'}
      <form class="ip-row flex flex-col gap-1.5 px-4 py-2.5" onsubmit={saveStamina}>
        <span class="ip-sub">Action points</span>
        <span class="flex items-center gap-1.5">
          <!-- svelte-ignore a11y_autofocus -->
          <input class="ip-field num h-9 min-w-0 flex-1 text-lg" type="number" aria-label="Action points" autofocus bind:value={inline.value} use:bounded={{ min: 0 }} onkeydown={inline.escape} />
          <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
        </span>
      </form>
    {:else}
      <button type="button" aria-label="Edit the action points" disabled={!world.activated} class="ip-row flex cursor-pointer flex-col gap-1 border-0 px-4 py-2.5 text-left text-white enabled:hover:shadow-[inset_0_0_0_2px_var(--color-cyan)] disabled:cursor-default" onclick={() => inline.start('stamina', world.stamina)}>
        <span class="ip-sub">Action points</span>
        <span><span class="num text-2xl">{count(world.stamina)}</span><span class="num text-base text-dim"> /{world.stamina_max}</span></span>
      </button>
    {/if}
    <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
      <span class="ip-sub">Reset due</span>
      <span class="num text-2xl">{world.expired_time ? day(world.expired_time) : '-'}</span>
    </div>
  </div>
  <VirtualGrid items={world.maps} rowHeight={46} key={(m) => m.random_id} label="Maps" class="max-h-[420px] px-2.5 pt-3 pb-2.5">
    {#snippet cell(m)}
      <div class={['ip-row flex items-center gap-4 px-3', m.active && 'ip-sel']}>
        <span class="num w-16 text-xl text-soft">{m.random_id}</span>
        <span class="truncate text-[17px]">{m.name || `Map ${m.random_id}`}</span>
        <span class={['ml-auto text-[15px]', m.active ? 'text-link' : m.clear ? 'text-ok' : 'text-soft']}>{m.active ? 'Fleet here' : m.clear ? 'Cleared' : 'Entered'}</span>
      </div>
    {/snippet}
    {#snippet empty()}
      <div class="ip-row px-3 py-2.5 text-dim">No map entered</div>
    {/snippet}
  </VirtualGrid>

  {#if query.error}
    <p class="m-0 px-5 pb-4 text-bad">{query.error.message}</p>
  {:else if detail}
    <div class="flex flex-col gap-6 px-5 pt-3 pb-5">
      {#if activation}
        <div class="flex flex-col gap-3">
          <h3 class="m-0 flex items-baseline gap-2.5 text-xl">
            Activation
            <span class="text-[15px] text-soft">{activation.map_name || `Map ${activation.map}`}</span>
            <button type="button" class="ip-btn ip-sm ml-auto" disabled={busy || !fleetsValid} onclick={activate}>Activate</button>
          </h3>
          {#each fleets as fleet, f (f)}
            <div class="ip-row flex flex-wrap items-center gap-4 px-4 py-3">
              <span class="w-28 text-lg">Fleet <span class="num">{f + 1}</span></span>
              <span class="flex items-center gap-2"><span class="ip-sub w-24">Vanguard</span>{#each [0, 1, 2] as i (i)}{@render slot(f, 'vanguard', i)}{/each}</span>
              <span class="flex items-center gap-2"><span class="ip-sub w-14">Main</span>{#each [0, 1, 2] as i (i)}{@render slot(f, 'main', i)}{/each}</span>
              <span class="ml-auto text-sm text-dim">Lv. <span class={['num text-lg', activation.total_level && levels(fleet) > activation.total_level ? 'text-ok' : 'text-soft']}>{levels(fleet)}</span></span>
            </div>
          {/each}
          {#if activation.submarine}
            <div class="ip-row flex flex-wrap items-center gap-4 px-4 py-3">
              <span class="w-28 text-lg">Submarine</span>
              <span class="flex items-center gap-2">{#each [0, 1, 2] as i (i)}{@render slot(-1, 'submarine', i)}{/each}</span>
            </div>
          {/if}
          <Picker bind:this={shipPicker} trigger={false} label={picking ? `${picking.team[0].toUpperCase()}${picking.team.slice(1)} ship` : 'Ship'} items={candidates} placeholder="Name or id" value={null} onselect={pickShip} />
        </div>
      {/if}

      <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-5">
        <div class="flex min-w-0 flex-col gap-2.5">
          <h3 class="m-0 flex items-baseline gap-2.5 text-xl">
            Inventory <span class="num text-lg text-soft">{detail.items.length}</span>
            <button type="button" class="ip-btn2 ip-sm ml-auto" disabled={busy || !detail.started} onclick={() => itemPicker?.show()}>Add</button>
          </h3>
          <div class="flex max-h-[380px] flex-col gap-1.5 overflow-y-auto">
            {#if adding}{@render itemRow(adding)}{/if}
            {#each detail.items as item (item.id)}
              {@render itemRow(item)}
            {:else}
              {#if !adding}<div class="ip-row px-3 py-2.5 text-dim">Empty</div>{/if}
            {/each}
          </div>
          <Picker bind:this={itemPicker} trigger={false} label="World item" items={catalogItems} placeholder="Name or id" value={null} onselect={pickItem} />
        </div>

        <div class="flex min-w-0 flex-col gap-2.5">
          <h3 class="m-0 text-xl">Counters</h3>
          {#if detail.started}
            {@const c = detail.counters}
            <div class="grid grid-cols-2 gap-2.5">
              {@render counterTile('exchanges', 'Oil exchanges', c.exchanges, c.exchanges_max, `${c.exchanges} / ${c.exchanges_max}`, true)}
              {@render resetTile('daily', 'Daily queue', `${c.daily_queued} / ${c.daily_limit}`, c.daily_refresh ? `Next ${short(c.daily_refresh, '/')}` : '', c.daily_queued === 0 && c.daily_refresh === 0)}
              {@render counterTile('task_finish', 'Tasks submitted', c.task_finish, detail.count_max, count(c.task_finish), true)}
              {@render counterTile('steps', 'Mileage', c.steps, detail.count_max, count(c.steps), true)}
              {@render counterTile('treasures', 'Treasures', c.treasures, detail.count_max, count(c.treasures), true)}
              {@render resetTile('cooldowns', 'Order cooldowns', String(c.cooldowns), '', c.cooldowns === 0)}
              {@render resetTile('new_shop', 'Port shop purchases', count(c.new_shop_bought), '', c.new_shop_bought === 0)}
              {@render resetTile('out_shop', 'Voucher shop purchases', count(c.out_shop_bought), '', c.out_shop_bought === 0)}
            </div>
          {:else}
            <div class="ip-row px-3 py-2.5 text-dim">Not started</div>
          {/if}
        </div>
      </div>

      <div class="flex flex-col gap-2.5">
        <h3 class="m-0 flex items-baseline gap-2.5 text-xl">
          Achievements
          {#if shownRegion}
            <span class="text-[15px] text-soft"><span class="num text-lg text-white">{shownRegion.achieved}</span> / <span class="num">{shownRegion.total}</span> achieved · <span class="num text-lg text-white">{shownRegion.claimed}</span> / <span class="num">{shownRegion.stars}</span> stars</span>
          {/if}
        </h3>
        <Seg label="Region" options={detail.regions.map((r) => ({ value: r.id, label: r.name }))} value={regionID} onchange={(next) => ((region = next), (open = null))} />
        <div class="flex max-h-[520px] flex-col gap-1.5 overflow-y-auto">
          {#each shownRegion?.entrances ?? [] as entrance (entrance.id)}
            <div class={['ip-row flex flex-col', open === entrance.id && 'ip-sel']}>
              <div class="flex items-center gap-3 px-3 py-1.5">
                <button type="button" aria-expanded={open === entrance.id} class="flex min-w-0 flex-1 cursor-pointer items-center gap-3 border-0 bg-transparent p-0 text-left text-white" onclick={() => (open = open === entrance.id ? null : entrance.id)}>
                  <span class="min-w-0 flex-1 truncate text-[17px]">{entrance.name}</span>
                  <span class="text-sm text-dim"><span class="num text-lg text-soft">{entrance.achieved}</span> / <span class="num">{entrance.targets.length}</span></span>
                </button>
                <span class="flex gap-1">
                  {#each entrance.stars as s (s.star)}
                    <button
                      type="button"
                      aria-label="Star {s.star}: {s.drop.name} ×{s.drop.count}, {s.claimed ? 'claimed' : s.earned ? 'earned' : 'not earned'}"
                      title="{s.drop.name} ×{count(s.drop.count)}"
                      aria-pressed={s.claimed}
                      disabled={busy || (!s.claimed && !s.earned)}
                      class={['grid size-8 cursor-pointer place-items-center rounded border-0 bg-black/25 hover:bg-white/15 disabled:cursor-default disabled:hover:bg-black/25', s.claimed ? 'text-gold' : s.earned ? 'text-gold/80' : 'text-white/25']}
                      onclick={() => toggleStar(entrance, s.star, s.claimed)}
                    >
                      {@render star(s.claimed)}
                    </button>
                  {/each}
                </span>
              </div>
              {#if open === entrance.id}
                <div class="flex flex-col gap-1 px-3 pb-2.5">
                  {#each entrance.targets as target (target.id)}
                    <div class="flex items-center gap-3 rounded-[3px] bg-black/20 px-3 py-1.5">
                      {#if target.cryptic}<span class="ip-tag bg-navy text-white">CRYPTIC</span>{/if}
                      <span class="min-w-0 flex-1 truncate text-[15px]">{target.desc}</span>
                      <span class={['num w-20 text-right text-lg', target.achieved ? 'text-ok' : 'text-soft']}>{target.count} / {target.need}</span>
                      {#if target.achieved || target.count > 0}
                        <button type="button" class="ip-btn2 ip-sm h-8 w-20 px-2.5 text-sm" disabled={busy} onclick={() => run(`/world/targets/${target.id}`, `${entrance.name} achievement reset`, undefined, 'DELETE')}>Reset</button>
                      {/if}
                      {#if !target.achieved}
                        <button type="button" class="ip-btn ip-sm h-8 w-20 px-2.5 text-sm" disabled={busy} onclick={() => run(`/world/targets/${target.id}/achieve`, `${entrance.name} achievement marked`)}>Achieve</button>
                      {/if}
                    </div>
                  {/each}
                  <div class="flex flex-wrap gap-3 pt-1.5">
                    {#each entrance.stars as s (s.star)}
                      <span class={['flex items-center gap-2 text-sm', s.claimed ? 'text-gold' : 'text-dim']}>
                        <span class="num text-base">{s.star}</span>
                        <Icon icon={s.drop.icon} rarity={s.drop.rarity} count={s.drop.count} size={44} name={s.drop.name} />
                      </span>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>

      <div class="flex flex-col gap-2.5">
        <h3 class="m-0 text-xl">Ports</h3>
        {#each detail.ports as port (port.id)}
          {@const left = port.goods.reduce((sum, g) => sum + g.left, 0)}
          {@const max = port.goods.reduce((sum, g) => sum + g.max, 0)}
          <div class="ip-row flex flex-wrap items-center gap-4 px-3 py-2">
            <span class="w-56 truncate text-[17px]">{port.name || `Port ${port.id}`}</span>
            <span class="flex flex-1 flex-wrap gap-2">
              {#each port.goods as goods (goods.id)}
                <span class={goods.left ? '' : 'opacity-35'}><Icon icon={goods.item.icon} rarity={goods.item.rarity} count={goods.left} size={40} name={goods.item.name} /></span>
              {:else}
                <span class="text-[15px] text-dim">No goods</span>
              {/each}
            </span>
            <span class="text-sm text-dim">Stock <span class="num text-lg text-soft">{left} / {max}</span></span>
            <span class="text-sm text-dim">Refresh <span class="num text-base text-soft">{port.next_refresh ? short(port.next_refresh, '/') : 'now'}</span></span>
            <span class="flex gap-2">
              <button type="button" class="ip-btn2 ip-sm h-8 px-2.5 text-sm" disabled={busy || left === max} onclick={() => run(`/world/ports/${port.id}/restock`, `${port.name} restocked`)}>Restock</button>
              <button type="button" class="ip-btn2 ip-sm h-8 px-2.5 text-sm" disabled={busy || port.next_refresh === 0} onclick={() => run(`/world/ports/${port.id}/refresh`, `${port.name} refreshed`)}>Refresh</button>
            </span>
          </div>
        {:else}
          <div class="ip-row px-3 py-2.5 text-dim">No port visited</div>
        {/each}
      </div>
    </div>
  {/if}
</section>
