<script lang="ts">
  // The commander's builds as the build screen's slots (each ship hidden
  // until hovered), every other clock as one list, soonest first
  // (commissions, lessons, field upgrades, the handover, research,
  // cooldowns), each with Finish now; under them the daily counters and the
  // academy fields.
  import { createQuery } from '@tanstack/svelte-query'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import { count, short } from '../../lib/format'
  import { shipFrames } from '../../lib/dock'
  import { cardArt } from '../../lib/players'
  import { link } from '../../lib/router.svelte'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import { bounded, uint32 } from '../../lib/bounded'
  import { createNow } from '../../lib/clock.svelte'
  import { hours, left, pct, type TimerShip, type Timers } from '../../lib/progress'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'timers'),
    queryFn: get<Timers>(`/commanders/${id}/timers`),
  }))
  const data = $derived(query.data)

  // the server's clock, ticking here
  const ticker = createNow(1000, () => data?.now)
  const now = $derived(ticker.now)
  // the field starts at the commander's points, again after each fetch
  let buildPoints = $derived(data?.build_points ?? 0)

  const { mutation: edit, run } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)
  const finishTimer = (kind: string, key = 0) => run('/timers/finish', 'Finished', { kind, key })

  const fieldNames = ['', 'Oil field', 'Merchant', 'Lecture hall']
  const fieldStock = ['', 'Oil', 'Coins', 'EXP']
  const ticketKinds: Record<number, string> = { 1: 'Campaign', 2: 'Operation Siren', 3: 'Time' }
  const cooldownNames: Record<number, string> = { 1: 'Rename' }
  // a ticket that never expires
  const never = uint32

  // the handover running, its sorties as one
  const handover = $derived.by(() => {
    if (!data || data.auto_battles.length === 0) return null
    const battles = data.auto_battles
    const finish = Math.max(...battles.map((b) => b.finish))
    const start = Math.min(...battles.map((b) => b.finish - b.seconds))
    return { name: battles[0].name || `#${battles[0].chapter}`, type: battles[0].type, sorties: battles.length, start, finish }
  })

  // a clock of the list: what it times, its kind, how to finish it
  interface Clock {
    key: string
    kind: string
    title: string
    sub: string
    ships: TimerShip[]
    start: number
    finish: number
    // the /timers/finish kind and key, Clear for a wait rather than a job
    finishKind: string
    finishKey: number
    clear?: boolean
    // the screen it belongs to
    href?: string
  }
  const clocks = $derived.by((): Clock[] => {
    if (!data) return []
    const list: Clock[] = []
    for (const c of data.commissions) list.push({ key: `commission:${c.id}`, kind: 'Commission', title: c.name, sub: '', ships: c.ships, start: c.start, finish: c.finish, finishKind: 'commission', finishKey: c.id })
    for (const l of data.lessons) list.push({ key: `lesson:${l.room}`, kind: 'Lesson', title: l.skill, sub: `${l.ship.name} · room ${l.room} · ${l.book}`, ships: [l.ship], start: l.start, finish: l.finish, finishKind: 'lesson', finishKey: l.room })
    for (const f of data.fields) {
      if (f.upgrade_at > now) list.push({ key: `field:${f.key}`, kind: 'Upgrade', title: `${fieldNames[f.key]} Lv.${f.level + 1}`, sub: '', ships: [], start: f.upgrade_at - f.upgrade_time, finish: f.upgrade_at, finishKind: 'field', finishKey: f.key })
    }
    if (handover) list.push({ key: 'auto', kind: 'Handover', title: handover.name, sub: `${handover.type === 1 ? 'Campaign' : 'Operation Siren'} · ${handover.sorties} ${handover.sorties === 1 ? 'sortie' : 'sorties'}`, ships: [], start: handover.start, finish: handover.finish, finishKind: 'auto', finishKey: 0 })
    for (const r of data.research) list.push({ key: `research:${r.kind}:${r.key}`, kind: 'Research', title: r.name, sub: `${r.kind === 'queue' ? 'Queued' : 'Project'} · ${r.sub}`, ships: [], start: r.finish - r.time, finish: r.finish, finishKind: r.kind, finishKey: r.key, href: `/commanders/${id}/research` })
    for (const t of data.fleet_tech) list.push({ key: `tech:${t.group}`, kind: 'Fleet tech', title: `${t.name} Lv.${t.level}`, sub: '', ships: [], start: t.finish - t.time, finish: t.finish, finishKind: 'fleet_tech', finishKey: t.group, href: `/commanders/${id}/research` })
    if (data.blueprint_cold > now) list.push({ key: 'blueprint', kind: 'Cooldown', title: 'Next research ship', sub: '', ships: [], start: data.blueprint_cold - 86400, finish: data.blueprint_cold, finishKind: 'blueprint', finishKey: 0, clear: true })
    for (const c of data.cooldowns) {
      if (c.until > now) list.push({ key: `cooldown:${c.key}`, kind: 'Cooldown', title: cooldownNames[c.key] ?? `Cooldown ${c.key}`, sub: '', ships: [], start: now, finish: c.until, finishKind: 'cooldown', finishKey: c.key, clear: true })
    }
    if (data.guild_wait > now) list.push({ key: 'guild', kind: 'Cooldown', title: 'Guild re-join', sub: '', ships: [], start: now, finish: data.guild_wait, finishKind: 'guild', finishKey: 0, clear: true })
    // running soonest first, then the ready ones
    return list.sort((a, b) => Number(a.finish <= now) - Number(b.finish <= now) || a.finish - b.finish)
  })
  const running = $derived(clocks.filter((c) => c.finish > now).length)
  const ready = $derived(clocks.length - running)
  // the build slots, the builds first in their order, then the empty ones
  const slots = $derived(data ? Array.from({ length: Math.max(data.build_slots, data.builds.length) }, (_, i) => data!.builds[i]) : [])
  // the slots whose ship shows: hovered or focused, or clicked to keep it
  let peeking = $state<number | null>(null)
  let pinned = $state<number[]>([])
  let revealAll = $state(false)
  const revealed = (slot: number) => revealAll || peeking === slot || pinned.includes(slot)
  const building = $derived(data?.builds.filter((b) => b.finish > now).length ?? 0)

  const tickets = $derived.by(() => {
    const kinds = new Map<number, { count: number; expire: number }>()
    for (const ticket of data?.auto_tickets ?? []) {
      const kind = kinds.get(ticket.type) ?? { count: 0, expire: never }
      kind.count += ticket.count
      kind.expire = Math.min(kind.expire, ticket.expire_at)
      kinds.set(ticket.type, kind)
    }
    return [1, 2, 3].map((type) => ({ type, ...(kinds.get(type) ?? { count: 0, expire: never }) }))
  })

  function setLevel(field: number, level: number) {
    run('/fields/level', `${fieldNames[field]} set to Lv.${level}`, { field, level })
  }

  function setBuildPoints() {
    if (!data || buildPoints === data.build_points) return
    run('/build-points', 'Build points set', { points: buildPoints })
  }
</script>

{#snippet counter(label: string, value: string, extra?: string)}
  <span class="ip-sub">{label}</span>
  <span class="num text-[26px] leading-tight">{value}{#if extra}<span class="text-lg text-dim">{extra}</span>{/if}</span>
{/snippet}

<CommanderPage {id} title="Timers">
  {#snippet children()}
    {#if data}
      <div class="@container flex flex-col gap-[18px]">
        <div class="flex items-stretch gap-[18px]">
        <section aria-labelledby="clocks-h" class="ip-panel min-w-0 flex-1">
          <div class="ip-head flex-wrap items-center gap-x-6 gap-y-2">
            <h2 id="clocks-h">Clocks</h2>
            <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Running <span class="num text-[22px] text-white">{running}</span></span>
            <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Ready <span class="num text-[22px] text-white">{ready}</span></span>
            <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Server time <span class="num text-[22px] text-white">{short(now)}</span></span>
          </div>
          <ul class="m-0 flex list-none flex-col gap-1.5 p-2.5">
            {#each clocks as c (c.key)}
              {@const done = c.finish <= now}
              <li class="ip-row relative grid grid-cols-[96px_minmax(0,1fr)_auto_104px] items-center gap-3 overflow-hidden px-3 py-2">
                <span class={['ip-tag justify-self-start text-white', c.kind === 'Cooldown' ? 'bg-charcoal/85' : 'bg-[#3d5a80]']}>{c.kind.toUpperCase()}</span>
                <span class="flex min-w-0 items-center gap-3">
                  {#if c.ships.length}
                    <span class="flex flex-none gap-1">{#each c.ships.slice(0, 6) as ship (ship.id)}<Icon icon={ship.icon} rarity={ship.rarity} ship name={ship.name} size={c.ships.length > 1 ? 34 : 40} />{/each}</span>
                  {/if}
                  <span class="flex min-w-0 flex-col">
                    {#if c.href}<a href={c.href} use:link class="truncate text-[17px] text-white">{c.title}</a>{:else}<span class="truncate text-[17px]">{c.title}</span>{/if}
                    {#if c.sub}<span class="truncate text-[13px] text-dim">{c.sub}</span>{/if}
                  </span>
                </span>
                <span class={['num text-right text-[24px]', done && 'text-ok']}>{done ? 'Ready' : left(c.finish - now)}</span>
                <span aria-hidden="true" class="absolute inset-x-0 bottom-0 h-[3px] bg-black/45"><span class={['block h-full', done ? 'bg-ok' : 'bg-cyan']} style:width="{done ? 100 : pct(c.start, c.finish, now)}%"></span></span>
                {#if done}
                  <span></span>
                {:else}
                  <button type="button" class="ip-btn ip-sm h-9 text-[15px]" disabled={busy} aria-label="{c.clear ? 'Clear' : 'Finish'} {c.title} now" onclick={() => finishTimer(c.finishKind, c.finishKey)}>{c.clear ? 'Clear' : 'Finish now'}</button>
                {/if}
              </li>
            {:else}
              <li class="px-3 py-2 text-dim">Nothing running</li>
            {/each}
          </ul>
        </section>
        <section aria-labelledby="builds-h" class="ip-panel flex-none self-start">
          <div class="ip-head flex-wrap items-center gap-x-6 gap-y-2">
            <h2 id="builds-h">Builds</h2>
            <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Building <span class="num text-[22px] text-white">{building}</span><span class="num text-dim">/{data.build_slots}</span></span>
            <label class="flex cursor-pointer items-center gap-2 text-sm text-soft">
              <input type="checkbox" class="size-5 accent-cyan" bind:checked={revealAll} />
              Reveal ships
            </label>
          </div>
          <ul class="m-0 grid list-none grid-cols-[repeat(5,80px)] gap-3 p-3">
            {#each slots as build, i (build?.id ?? `empty:${i}`)}
              <li class="flex min-w-0 flex-col gap-1">
                {#if build}
                  {@const done = build.finish <= now}
                  {@const shown = revealed(i)}
                  <button
                    type="button"
                    aria-pressed={pinned.includes(i)}
                    aria-label={shown ? `Slot ${i + 1}: ${build.ship.name}` : `Slot ${i + 1}: reveal the ship`}
                    class={[
                      'relative aspect-[216/300] cursor-pointer overflow-hidden rounded-[3px] border-2 bg-[#1b2238] p-0 shadow-[0_4px_10px_rgb(0_0_0/0.3)] transition-colors',
                      shown ? (shipFrames[build.ship.rarity] ?? shipFrames[2]) : 'border-white/20',
                    ]}
                    onpointerenter={() => (peeking = i)}
                    onpointerleave={() => peeking === i && (peeking = null)}
                    onfocus={() => (peeking = i)}
                    onblur={() => peeking === i && (peeking = null)}
                    onclick={() => (pinned = pinned.includes(i) ? pinned.filter((n) => n !== i) : [...pinned, i])}
                  >
                    {#if build.ship.icon}
                      <img src="/api/assets/{cardArt(build.ship.icon)}.png" alt="" loading="lazy" class={['absolute inset-0 size-full object-cover object-top transition-[filter] duration-200', !shown && 'brightness-0']} />
                    {/if}
                    <span class="num absolute top-0 left-1 text-base text-white/60 [text-shadow:0_0_4px_#000]">{i + 1}</span>
                    {#if shown}
                      <span class="absolute inset-x-0 bottom-0 truncate bg-[linear-gradient(0deg,rgb(0_0_0/0.8),transparent)] px-0.5 pt-3 pb-0.5 text-center text-[11px] text-white">{build.ship.name}</span>
                    {:else}
                      <span aria-hidden="true" class="absolute inset-0 grid place-items-center text-2xl text-white/25">?</span>
                    {/if}
                  </button>
                  <span aria-hidden="true" class="h-1 rounded-[2px] bg-black/45"><span class={['block h-full rounded-[2px]', done ? 'bg-ok' : 'bg-cyan']} style:width="{done ? 100 : pct(build.start, build.finish, now)}%"></span></span>
                  <span class={['num text-center text-sm leading-tight', done && 'text-ok']}>{done ? 'Ready' : left(build.finish - now)}</span>
                  <span class="truncate text-center text-[10px] text-dim">{build.pool}</span>
                  {#if !done}<button type="button" class="ip-btn ip-sm h-6 px-1 text-[12px]" disabled={busy} aria-label="Finish slot {i + 1} now" onclick={() => finishTimer('build', build.id)}>Finish</button>{/if}
                {:else}
                  <div class="relative grid aspect-[216/300] place-items-center rounded-[3px] border-2 border-dashed border-white/20 bg-white/4">
                    <span class="num absolute top-0 left-1 text-base text-white/30">{i + 1}</span>
                  </div>
                  <span class="text-center text-[10px] text-dim">Empty</span>
                {/if}
              </li>
            {/each}
          </ul>
        </section>
        </div>

        <section aria-label="Counters" class="grid grid-cols-3 gap-2.5 @7xl:grid-cols-6">
          <div class="ip-panel relative flex flex-col gap-1 px-4 py-3">
            <span class="ip-sub">Build points</span>
            <span class="flex items-baseline gap-1.5">
              <input class="ip-field num h-10 w-24 text-right text-xl" type="number" aria-label="Build points" bind:value={buildPoints} use:bounded={{ min: 0, max: data.build_points_max }} onchange={setBuildPoints} />
              <span class="num text-lg text-dim">/{data.build_points_max}</span>
            </span>
          </div>
          <div class="ip-panel relative flex flex-col gap-1 px-4 py-3">
            {@render counter('Commissions today', String(data.daily_drawn), `/${data.daily_draws}`)}
            <button type="button" class="ip-btn2 ip-sm absolute right-3 bottom-3" disabled={busy || data.daily_drawn === 0} onclick={() => run('/timers/reset', 'Daily draws reset', { counter: 'commission_draws' })}>Reset</button>
          </div>
          <div class="ip-panel relative flex flex-col gap-1 px-4 py-3">{@render counter('Lesson rooms', String(data.lessons.length), `/${data.rooms}`)}</div>
          <div class="ip-panel relative flex flex-col gap-1 px-4 py-3">
            {@render counter('Quick finishes', String(data.quick_used), `/${data.quick_allowed}`)}
            <button type="button" class="ip-btn2 ip-sm absolute right-3 bottom-3" disabled={busy || data.quick_used === 0} onclick={() => run('/timers/reset', 'Quick finishes reset', { counter: 'quick_finishes' })}>Reset</button>
          </div>
          <div class="ip-panel relative flex flex-col gap-1 px-4 py-3">{@render counter('Handover today', hours(data.auto_used), ` / ${hours(data.auto_limit)}`)}</div>
          <div class="ip-panel relative flex flex-col gap-1 px-4 py-3">
            <span class="ip-sub">Handover tickets</span>
            <span class="flex flex-wrap items-baseline gap-x-4 text-[15px] text-soft">
              {#each tickets as ticket (ticket.type)}
                <span title={ticket.count && ticket.expire !== never ? `Until ${short(ticket.expire)}` : undefined}>{ticketKinds[ticket.type]} <span class="num text-[22px] text-white">{ticket.count}</span></span>
              {/each}
            </span>
          </div>
        </section>

        <section aria-labelledby="fields-h" class="ip-panel">
          <div class="ip-head"><h2 id="fields-h">Academy</h2></div>
          <ul class="m-0 grid list-none grid-cols-3 gap-1.5 p-2.5">
            {#each data.fields as field (field.key)}
              {@const upgrading = field.upgrade_at > now}
              <li class="ip-row flex items-center gap-3 px-3 py-2">
                <span class="flex min-w-0 flex-1 flex-col">
                  <span class="text-[17px]">{fieldNames[field.key]}</span>
                  <span class="truncate text-[13px] text-dim">{upgrading ? `Upgrading to Lv.${field.level + 1}` : `${fieldStock[field.key]} ${count(field.stored)} to collect`}</span>
                </span>
                <span class="flex h-9 flex-none items-center rounded-[18px] bg-strip px-[3px]">
                  <button type="button" aria-label="Lower {fieldNames[field.key]} level" class="grid size-[30px] cursor-pointer place-items-center rounded-full border-0 bg-white disabled:cursor-not-allowed disabled:opacity-50" disabled={busy || upgrading || field.level <= 1} onclick={() => setLevel(field.key, field.level - 1)}>
                    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="#3a3e47" stroke-width="3" stroke-linecap="round"><path d="M5 12h14" /></svg>
                  </button>
                  <span class="w-[66px] text-center text-[13px] text-soft">Lv.<span class="num text-xl text-white">{field.level}</span><span class="num text-dim">/{field.max_level}</span></span>
                  <button type="button" aria-label="Raise {fieldNames[field.key]} level" class="grid size-[30px] cursor-pointer place-items-center rounded-full border-0 bg-white disabled:cursor-not-allowed disabled:opacity-50" disabled={busy || upgrading || field.level >= field.max_level} onclick={() => setLevel(field.key, field.level + 1)}>
                    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="#3a3e47" stroke-width="3" stroke-linecap="round"><path d="M5 12h14M12 5v14" /></svg>
                  </button>
                </span>
              </li>
            {/each}
          </ul>
        </section>
      </div>
    {/if}
  {/snippet}
</CommanderPage>
