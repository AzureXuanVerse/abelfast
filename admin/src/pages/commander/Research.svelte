<script lang="ts">
  import { untrack } from 'svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import Card from '../../components/Card.svelte'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import Panel from '../../components/Panel.svelte'
  import VirtualGrid from '../../components/VirtualGrid.svelte'
  import Seg from '../../components/Seg.svelte'
  import { api } from '../../lib/api'
  import { count, short } from '../../lib/format'
  import { createCommanderMutation, createCommanderWrite, done, get, keys, type Kicked } from '../../lib/query'
  import { createNow } from '../../lib/clock.svelte'
  import { left, pct, techAttrs, type BlueprintShip, type ResearchView } from '../../lib/progress'
  import { bounded } from '../../lib/bounded'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'research'),
    queryFn: get<ResearchView>(`/commanders/${id}/research`),
  }))
  const data = $derived(query.data)

  // the server's clock, ticking here
  const ticker = createNow(1000, () => data?.now)
  const now = $derived(ticker.now)
  let pickedID = $state(0)
  let devLevel = $state(0)
  let fate = $state(0)

  $effect(() => {
    if (!data) return
    const keep = untrack(() => pickedID) || ((data.blueprints.find((b) => b.dev) ?? data.blueprints[0])?.id ?? 0)
    pick(data.blueprints.find((b) => b.id === keep))
  })

  function pick(ship: BlueprintShip | undefined) {
    if (!ship) return
    pickedID = ship.id
    devLevel = ship.dev?.level ?? 0
    fate = Math.max(0, ship.dev?.fate ?? 0)
  }

  const { mutation: edit, run } = createCommanderWrite(() => id)
  // the running queue entries, finished one after the other
  const completeQueue = createCommanderMutation(
    () => id,
    async (entries: number[]) => {
      let kicked = false
      for (const key of entries) kicked = (await api<Kicked>('POST', `/commanders/${id}/timers/finish`, { kind: 'queue', key })).kicked || kicked
      return { kicked }
    },
    { success: done('Queue complete') },
  )
  const busy = $derived(edit.isPending || completeQueue.isPending)
  const finish = (kind: string, key: number, name: string) => run('/timers/finish', `${name} complete`, { kind, key })

  const picked = $derived(data?.blueprints.find((b) => b.id === pickedID))
  // the development as stored: the level, fate phases past the max level
  const target = $derived(picked?.dev ? (devLevel >= picked.dev.max_level ? picked.dev.max_level + fate : devLevel) : 0)
  const current = $derived(picked?.dev ? picked.dev.level + Math.max(0, picked.dev.fate) : 0)

  const stateLabel: Record<string, string> = { locked: 'Locked', stopped: 'Stopped', running: 'Researching', done: 'Built' }

  // the bonuses by ship type, the attrs any type has as columns
  const attrColumns = $derived([...new Set((data?.fleet_tech_attrs ?? []).map((a) => a.attr))].sort((a, b) => a - b))
  const attrRows = $derived.by(() => {
    const rows = new Map<number, { name: string; attrs: Map<number, { max: number; set: number }> }>()
    for (const attr of data?.fleet_tech_attrs ?? []) {
      const row = rows.get(attr.ship_type) ?? { name: attr.type_name, attrs: new Map() }
      row.attrs.set(attr.attr, { max: attr.max, set: attr.set })
      rows.set(attr.ship_type, row)
    }
    return [...rows.entries()].map(([type, row]) => ({ type, ...row, lowered: [...row.attrs.values()].some((a) => a.set < a.max) }))
  })

  // what Max all raises: the factions below their last level (not studying)
  // and the ship types with a bonus lowered
  const belowMax = $derived((data?.fleet_tech ?? []).filter((g) => g.level < g.max_level && !(g.study > 0)))
  const lowered = $derived(attrRows.filter((row) => row.lowered))
  async function maxAll() {
    try {
      for (const g of belowMax) await edit.mutateAsync({ path: '/research/fleet-tech', body: { group: g.id, level: g.max_level }, done: `${g.name} Lv.${g.max_level}` })
      for (const row of lowered) await edit.mutateAsync({ path: '/research/fleet-tech/reset', body: { ship_type: row.type }, done: `${row.name} bonuses at max` })
    } catch {
      // the toast has the server's answer
    }
  }
</script>

<CommanderPage {id} title="Research">
  {#snippet children()}
    {#if data}
      <div class="grid grid-cols-1 items-start gap-[18px] xl:grid-cols-[minmax(0,1fr)_340px]">
        <div class="flex min-w-0 flex-col gap-[18px]">
          <Panel title="Projects" sub="Offered">
            {#snippet actions()}
              <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Refreshed <span class="num text-[22px] text-white">{data!.refreshed_at ? short(data!.refreshed_at) : '—'}</span></span>
            {/snippet}
            <div class="grid grid-cols-2 gap-2.5 p-2.5 lg:grid-cols-5">
              {#each data.offers as offer (offer.pos)}
                {@const running = offer.finish > now}
                <div class={['ip-row flex flex-col gap-1.5 p-3', running && 'ip-sel']}>
                  <span class="flex items-center justify-between gap-2">
                    <span class="ip-tag bg-strip text-soft">{offer.sub}</span>
                    <span class="num text-dim">PR{offer.version}</span>
                  </span>
                  <span class="num text-[26px]">{offer.name}</span>
                  {#if running}
                    <span class="text-sm text-soft">Running <span class="num text-lg text-white">{left(offer.finish - now)}</span></span>
                    <button type="button" class="ip-btn ip-sm mt-auto" disabled={busy} onclick={() => finish('research', offer.pos, offer.name)}>Complete</button>
                  {:else if offer.finish}
                    <span class="text-sm text-ok">Ready</span>
                  {:else}
                    <span class="text-sm text-soft">Takes <span class="num text-lg text-white">{left(offer.time)}</span></span>
                  {/if}
                </div>
              {:else}
                <span class="px-2 py-1 text-dim">No projects offered</span>
              {/each}
            </div>
          </Panel>

          <div class="grid grid-cols-1 items-start gap-[18px] lg:grid-cols-2">
            <Panel title="Queue" sub="Running">
              {#snippet actions()}
                <button type="button" class="ip-btn ip-sm" disabled={busy || !data!.queue.some((q) => q.finish > now)} onclick={() => completeQueue.mutate(data!.queue.filter((q) => q.finish > now).map((q) => q.id))}>Complete all</button>
              {/snippet}
              <ul class="m-0 flex list-none flex-col gap-1.5 p-2.5">
                {#each data.queue as entry, i (entry.id)}
                  <li class="ip-row grid grid-cols-[20px_minmax(0,1fr)_auto_auto] items-center gap-3 px-3 py-2">
                    <span class="num text-dim">{i + 1}</span>
                    <span class="flex min-w-0 flex-col">
                      <span class="num text-lg">{entry.name}</span>
                      <span class="truncate text-[13px] text-dim">{entry.sub}</span>
                    </span>
                    {#if entry.finish > now}
                      <span class="num text-xl">{left(entry.finish - now)}</span>
                      <button type="button" class="ip-btn ip-sm" disabled={busy} onclick={() => finish('queue', entry.id, entry.name)}>Complete</button>
                    {:else}
                      <span class="num text-xl text-ok">Ready</span>
                      <span></span>
                    {/if}
                  </li>
                {:else}
                  <li class="px-3 py-2 text-dim">Empty</li>
                {/each}
              </ul>
            </Panel>

            <Panel title="Catch-up">
              {#snippet actions()}
                {#if data!.catchup_target}<span class="flex items-baseline gap-2 text-sm text-soft">Target <span class="text-lg text-white">{data!.catchup_target}</span></span>{/if}
              {/snippet}
              <ul class="m-0 flex list-none flex-col gap-1.5 p-2.5">
                {#each data.catchups as row (row.version + ':' + row.group)}
                  <li class="ip-row grid grid-cols-[52px_minmax(0,1fr)_auto] items-center gap-3 px-3 py-2">
                    <span class="num text-xl">PR{row.version}</span>
                    <span class="truncate text-sm text-soft">{row.group ? row.name : 'Priority'}</span>
                    <span class="num text-xl">{row.number}<span class="text-dim">/{row.max}</span></span>
                  </li>
                {:else}
                  <li class="px-3 py-2 text-dim">No catch-up blueprints</li>
                {/each}
              </ul>
            </Panel>
          </div>

          <Panel title="Blueprint ships" sub="PR / DR">
            {#snippet actions()}
              <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Catch-up bought <span class="num text-[22px] text-white">{data!.pursued}</span></span>
              <span class="flex items-baseline gap-2 text-sm whitespace-nowrap text-soft">Decisive <span class="num text-[22px] text-white">{data!.pursued_ur}</span></span>
            {/snippet}
            <VirtualGrid items={data.blueprints} columns={2} rowHeight={64} key={(ship) => ship.id} label="Blueprint ships" class="max-h-[520px] p-2.5">
              {#snippet cell(ship)}
                <button
                  type="button"
                  aria-pressed={ship.id === pickedID}
                  class={['ip-row grid cursor-pointer grid-cols-[48px_minmax(0,1fr)_auto_96px] items-center gap-3 border-0 px-2.5 py-1.5 text-left text-white', ship.id === pickedID && 'ip-sel']}
                  onclick={() => pick(ship)}
                >
                  <Icon icon={ship.icon} rarity={ship.rarity} ship name={ship.name} size={48} />
                  <span class="flex min-w-0 flex-col">
                    <span class="flex items-center gap-2"><span class="truncate text-[17px]">{ship.name}</span><span class={['ip-tag', ship.rarity >= 6 ? 'bg-tier-5' : 'bg-tier-2']}>{ship.rarity >= 6 ? 'DR' : 'PR'}{ship.version}</span></span>
                    <span class={['truncate text-[13px]', ship.state === 'locked' ? 'text-dim' : 'text-soft']}>{stateLabel[ship.state]}{ship.state === 'running' && ship.start ? ` since ${short(ship.start)}` : ''}</span>
                  </span>
                  <span class="text-sm text-soft">{#if ship.dev}Dev <span class="num text-lg text-white">{ship.dev.level}</span>{/if}</span>
                  <span class="flex flex-col gap-1">
                    {#if ship.dev}
                      <span class="num text-right text-sm">{ship.dev.need_exp ? `${ship.dev.exp}/${ship.dev.need_exp}` : 'Max'}</span>
                      <span aria-hidden="true" class="h-1.5 rounded-[3px] bg-black/45"><span class="block h-full rounded-[3px] bg-tier-3" style:width="{(ship.dev.level / ship.dev.max_level) * 100}%"></span></span>
                    {/if}
                  </span>
                </button>
              {/snippet}
            </VirtualGrid>
          </Panel>

          <Panel title="Fleet tech" sub="Tech points">
            {#snippet actions()}
              {#if belowMax.length || lowered.length}
                <button type="button" class="ip-btn ip-sm" disabled={busy} onclick={maxAll}>Max all</button>
              {/if}
            {/snippet}
            <div class="grid grid-cols-1 gap-2.5 p-2.5 sm:grid-cols-2 lg:grid-cols-4">
              {#each data.fleet_tech as group (group.id)}
                {@const studying = group.study > 0}
                <div class="ip-row flex flex-col gap-2 p-3">
                  <span class="flex items-baseline justify-between gap-2"><span class="text-lg">{group.name}</span><span class="num text-link">{count(group.points)}</span></span>
                  <span class="flex h-9 items-center justify-between rounded-[18px] bg-strip px-[3px]">
                    <button type="button" aria-label="Lower {group.name}" class="grid size-[30px] cursor-pointer place-items-center rounded-full border-0 bg-white disabled:cursor-not-allowed disabled:opacity-50" disabled={busy || studying || group.level <= 0} onclick={() => run('/research/fleet-tech', `${group.name} Lv.${group.level - 1}`, { group: group.id, level: group.level - 1 })}>
                      <svg aria-hidden="true" viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="#3a3e47" stroke-width="3" stroke-linecap="round"><path d="M5 12h14" /></svg>
                    </button>
                    <span class="text-[13px] text-soft">Lv.<span class="num text-xl text-white">{group.level}</span>/{group.max_level}</span>
                    <button type="button" aria-label="Raise {group.name}" class="grid size-[30px] cursor-pointer place-items-center rounded-full border-0 bg-white disabled:cursor-not-allowed disabled:opacity-50" disabled={busy || studying || group.level >= group.max_level} onclick={() => run('/research/fleet-tech', `${group.name} Lv.${group.level + 1}`, { group: group.id, level: group.level + 1 })}>
                      <svg aria-hidden="true" viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="#3a3e47" stroke-width="3" stroke-linecap="round"><path d="M5 12h14M12 5v14" /></svg>
                    </button>
                  </span>
                  {#if studying}
                    <span class="flex items-center justify-between gap-2 text-[13px] text-soft">Studying Lv.{group.study}<span class="num text-base text-white">{left(group.study_end - now)}</span></span>
                    <span aria-hidden="true" class="h-1.5 rounded-[3px] bg-black/45"><span class="block h-full rounded-[3px] bg-cyan" style:width="{pct(group.study_end - group.study_time, group.study_end, now)}%"></span></span>
                  {:else}
                    <span class="text-[13px] text-soft">{group.level >= group.max_level ? 'Max' : `Next at ${count(group.next_points)} pt`}</span>
                  {/if}
                  <span class={['text-[13px]', group.rewarded < group.level ? 'text-gold' : 'text-soft']}>
                    {group.rewarded < group.level ? `Lv.${group.level} reward waiting` : group.rewarded ? `Lv.${group.rewarded} reward taken` : 'No reward yet'}
                  </span>
                </div>
              {/each}
            </div>
            {#if attrRows.length}
              <div class="overflow-x-auto px-2.5 pb-2.5">
                <table class="w-full border-separate border-spacing-y-1.5 text-[15px]">
                  <thead>
                    <tr class="ip-sub text-left">
                      <th class="px-3 font-bold">Ship type</th>
                      {#each attrColumns as attr (attr)}<th class="px-2 text-right font-bold">{techAttrs[attr]}</th>{/each}
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each attrRows as row (row.type)}
                      <tr class="bg-white/6">
                        <td class="rounded-l-[3px] px-3 py-1.5">{row.name}</td>
                        {#each attrColumns as attr (attr)}
                          {@const value = row.attrs.get(attr)}
                          <td class={['num px-2 text-right', value && value.set < value.max ? 'text-gold' : '']}>{value ? `+${value.set}/${value.max}` : '—'}</td>
                        {/each}
                        <td class="rounded-r-[3px] px-2 py-1 text-right">
                          {#if row.lowered}
                            <button type="button" class="ip-btn ip-sm h-8" disabled={busy} onclick={() => run('/research/fleet-tech/reset', `${row.name} bonuses at max`, { ship_type: row.type })}>Max</button>
                          {:else}
                            <span class="text-sm text-soft">At max</span>
                          {/if}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          </Panel>
        </div>

        {#if picked}
          <div class="xl:sticky xl:top-4">
            <Card title={picked.name}>
              <div class="flex items-center gap-4">
                <Icon icon={picked.icon} rarity={picked.rarity} ship name={picked.name} size={72} />
                <div class="ip-strip flex-1">Research</div>
              </div>
              <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1 text-[15px] text-ink-soft">
                <span>{stateLabel[picked.state]}</span>
                {#if picked.ship_id}<span>Ship Lv.<span class="num text-lg text-ink">{picked.ship_level}</span></span>{/if}
                {#if data.blueprint_cold > now}<span>Next start in <span class="num text-lg text-ink">{left(data.blueprint_cold - now)}</span></span>{/if}
              </div>
              {#if picked.state !== 'done'}
                <button type="button" class="ip-btn" disabled={busy} onclick={() => run('/research/blueprint/unlock', `${picked!.name} unlocked`, { blueprint: picked!.id })}>Unlock {picked.name}</button>
              {/if}
              {#if picked.dev}
                {@const dev = picked.dev}
                <label class="ip-lab">
                  Dev level
                  <input class="ip-field num text-2xl" type="number" min={dev.level} max={dev.max_level} bind:value={devLevel} use:bounded={{ min: dev.level, max: dev.max_level }} />
                </label>
                {#if dev.max_fate > 0 && devLevel >= dev.max_level}
                  <div class="ip-strip">Fate simulation</div>
                  <Seg boxes label="Fate simulation" options={Array.from({ length: dev.max_fate + 1 }, (_, n) => ({ value: n, label: String(n) }))} bind:value={fate} />
                {/if}
                <div class="grid grid-cols-[1fr_1.4fr] gap-3">
                  <button type="button" class="ip-btn2" disabled={busy} onclick={() => pick(picked)}>Reset</button>
                  <button type="button" class="ip-btn" disabled={busy || target <= current || devLevel > dev.max_level} onclick={() => run('/research/blueprint', `${picked!.name} developed`, { blueprint: picked!.id, level: target })}>Save</button>
                </div>
              {/if}
            </Card>
          </div>
        {/if}
      </div>
    {/if}
  {/snippet}
</CommanderPage>
