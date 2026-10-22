<script lang="ts">
  import { untrack } from 'svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import Picker, { type PickerItem } from '../../components/Picker.svelte'
  import { fleetName, type Meowfficers } from '../../lib/dock'
  import { createNow } from '../../lib/clock.svelte'
  import { tier } from '../../lib/format'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import { bounded } from '../../lib/bounded'
  import { param } from '../../lib/router.svelte'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'meowfficers'),
    queryFn: get<Meowfficers>(`/commanders/${id}/meowfficers`),
  }))
  const data = $derived(query.data)
  let show = $state<'all' | 'locked' | 'fleet'>('all')
  // ?cat= (from a fleet) picks the cat shown first
  let selected = $state(param('cat') ?? 0)
  const ticker = createNow(30_000)
  const now = $derived(ticker.now)

  // the editor's copy of the selected cat
  let name = $state('')
  let level = $state(1)
  let exp = $state(0)
  let locked = $state(false)
  let skillLevel = $state(1)
  let skillExp = $state(0)
  let grant = $state<number | null>(null)

  function pick(catID: number) {
    selected = catID
    const cat = data?.meowfficers.find((c) => c.id === catID)
    if (!cat) return
    name = cat.name
    level = cat.level
    exp = cat.exp
    locked = cat.locked
    skillLevel = cat.skill.count ?? 1
    skillExp = cat.skill_exp
  }

  // the editor follows the cats each time they are (re)loaded
  $effect(() => {
    const loaded = query.data
    if (!loaded) return
    untrack(() => pick(loaded.meowfficers.some((c) => c.id === selected) ? selected : (loaded.meowfficers[0]?.id ?? 0)))
  })

  const cat = $derived(data?.meowfficers.find((c) => c.id === selected))
  const shown = $derived(data?.meowfficers.filter((c) => show === 'all' || (show === 'locked' ? c.locked : c.fleet)) ?? [])
  const catName = (catID: number) => data?.meowfficers.find((c) => c.id === catID)?.name
  const templates = $derived<PickerItem[]>((data?.templates ?? []).map((t) => ({ id: t.id, name: t.name || `#${t.id}`, icon: t.icon, rarity: tier(t.rarity ?? 2, true) })))

  const { mutation: write, run } = createCommanderWrite(() => id)
  const busy = $derived(write.isPending)

  // the EXP a level holds: below what it takes to the next, none at the last
  // (a level some skills reach for 0 EXP holds none too)
  const expMax = (exps: number[], level: number) => Math.max(0, (exps[level - 1] ?? 1) - 1)

  function save() {
    if (!cat) return
    const body: Record<string, unknown> = {}
    if (name !== cat.name) body.name = name === cat.template_name ? '' : name
    if (level !== cat.level) body.level = level
    if (exp !== cat.exp) body.exp = exp
    if (locked !== cat.locked) body.locked = locked
    if (skillLevel !== cat.skill.count) body.skill_level = skillLevel
    if (skillExp !== cat.skill_exp) body.skill_exp = skillExp
    run(`/meowfficers/${cat.id}`, `${name} saved`, body)
  }

  const duration = (seconds: number) => {
    const s = Math.max(0, Math.round(seconds))
    const h = Math.floor(s / 3600)
    const m = Math.floor((s % 3600) / 60)
    return h ? `${h}h ${m}m` : `${m}m`
  }
  const clock = (unix: number) => new Date(unix * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
</script>

<CommanderPage {id} title="Meowfficers">
  {#snippet children()}
    {#if query.error}
      <p class="text-bad">{query.error.message}</p>
    {:else if data}
      <div class="flex flex-wrap items-start gap-5">
        <div class="flex min-w-0 flex-[1_1_600px] flex-col gap-5">
          <section aria-label="Meowfficers" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Meowfficers</h2>
              <span class="ip-sub">Roster</span>
              <span class="num text-2xl text-soft">{data.meowfficers.length}<span class="text-base text-dim">/{data.capacity}</span></span>
              <div role="group" aria-label="Show" class="ip-seg ml-auto">
                <button type="button" aria-pressed={show === 'all'} onclick={() => (show = 'all')}>All</button>
                <button type="button" aria-pressed={show === 'locked'} onclick={() => (show = 'locked')}>Locked</button>
                <button type="button" aria-pressed={show === 'fleet'} onclick={() => (show = 'fleet')}>In a fleet</button>
              </div>
            </div>
            <div class="grid grid-cols-[repeat(auto-fill,minmax(128px,1fr))] gap-3.5 p-[18px]">
              {#each shown as c (c.id)}
                <button type="button" aria-pressed={selected === c.id} class={['relative flex cursor-pointer flex-col items-center gap-1.5 rounded border-0 bg-[#f5f6f8] px-1.5 pt-3 pb-2.5 text-[#2b2f38] shadow-[0_4px_10px_rgb(0_0_0/0.3)]', selected === c.id && 'shadow-[0_0_0_3px_var(--color-cyan),0_4px_10px_rgb(0_0_0/0.3)]']} onclick={() => pick(c.id)}>
                  <span class="relative">
                    <Icon icon={c.icon} rarity={tier(c.rarity, true)} size={78} name={c.name} count="Lv.{c.level}" />
                    {#if c.locked}<svg role="img" aria-label="Locked" viewBox="0 0 24 24" class="absolute -top-1 -left-1 size-[18px] fill-[#3a3e47]"><path d="M7 10V8a5 5 0 0 1 10 0v2h1v11H6V10h1Zm2 0h6V8a3 3 0 0 0-6 0v2Z" /></svg>{/if}
                  </span>
                  <span class="max-w-full truncate text-base">{c.name}</span>
                  <span class="flex items-center gap-1 text-[11px] text-[#6b7386]">
                    <span class="truncate">{c.nation}</span>
                    {#if c.fleet}<span class="ip-tag bg-cyan text-white">{fleetName(c.fleet)}</span>{/if}
                  </span>
                </button>
              {:else}
                <p class="col-span-full text-soft">None</p>
              {/each}
            </div>
          </section>

          <section aria-label="Training boxes" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Training boxes</h2>
              <span class="ip-sub">Queue</span>
              <span class="ml-auto text-sm text-soft">Bought today <span class="num text-xl text-white">{data.bought_today}</span> · ever <span class="num text-xl text-white">{data.bought_total}</span></span>
            </div>
            <ul class="m-0 flex list-none flex-col gap-1.5 p-3">
              {#each data.boxes as box (box.id)}
                {@const state = box.ready ? 'Ready' : box.begin_time > now ? 'Queued' : 'Training'}
                <li class="ip-row grid grid-cols-[32px_48px_minmax(0,1fr)_150px_auto] items-center gap-3.5 px-3.5 py-2">
                  <span class="num text-[26px] text-dim">{box.id}</span>
                  <Icon icon={box.box.icon} rarity={box.box.rarity} size={44} name={box.box.name} />
                  <span class="flex min-w-0 flex-col gap-0.5">
                    <span class="truncate text-base">{box.box.name || 'Cat box'}</span>
                    <span class="truncate text-[13px] text-soft">Holds {box.holds}</span>
                  </span>
                  <span class="flex flex-col items-start gap-[3px]">
                    <span class={['ip-tag', box.ready ? 'bg-[#2f9e5c] text-white' : box.begin_time > now ? 'bg-soft text-ink' : 'bg-cyan text-white']}>{state}</span>
                    <span class="num text-lg text-[#dfe5f0]">{box.ready ? `Since ${clock(box.finish_time)}` : box.begin_time > now ? `Starts ${clock(box.begin_time)}` : `${duration(box.finish_time - now)} left`}</span>
                  </span>
                  <span class="flex gap-1.5">
                    <button type="button" class="ip-btn ip-sm" disabled={busy || box.ready} onclick={() => run(`/meowfficer-boxes/${box.id}/finish`, 'Box finished')}>Finish now</button>
                    <button type="button" class="ip-danger ip-sm" disabled={busy} onclick={() => confirm(`Empty box ${box.id}? The cat in it is lost.`) && run(`/meowfficer-boxes/${box.id}`, 'Box emptied', undefined, 'DELETE')}>Empty</button>
                  </span>
                </li>
              {:else}
                <li class="px-1 text-soft">No box training</li>
              {/each}
            </ul>
          </section>

          <div class="grid grid-cols-[minmax(0,3fr)_minmax(0,2fr)] gap-5">
            <section aria-label="Cattery" class="ip-panel">
              <div class="ip-head items-center">
                <h2>Cattery</h2>
                <span class="ip-sub">Comf-forts</span>
                <span class="ml-auto text-sm text-soft">Lv.<span class="num text-[22px] text-white">{data.cattery.level}</span></span>
              </div>
              <div class="flex flex-col gap-3 px-4 pt-3.5 pb-4">
                <div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2.5 text-sm text-soft">
                  <span>Exp</span>
                  <span aria-hidden="true" class="h-2 rounded bg-black/45"><span class="block h-full rounded bg-cyan" style:width="{Math.min(100, (100 * data.cattery.exp) / Math.max(1, data.cattery.exp_next))}%"></span></span>
                  <span class="num text-lg text-white">{data.cattery.exp}/{data.cattery.exp_next}</span>
                  <span>Clean</span>
                  <span aria-hidden="true" class="h-2 rounded bg-black/45"><span class="block h-full rounded bg-ok" style:width="{Math.min(100, (100 * data.cattery.clean) / Math.max(1, data.cattery.clean_max))}%"></span></span>
                  <span class="num text-lg text-white">{data.cattery.clean}/{data.cattery.clean_max}</span>
                </div>
                <ul class="m-0 grid list-none grid-cols-2 gap-1.5 p-0">
                  {#each data.cattery.slots as slot (slot.id)}
                    {@const done = [slot.fed && 'Fed', slot.played && 'played', slot.cleaned && 'cleaned'].filter(Boolean)}
                    <li class="ip-row flex flex-col gap-[3px] px-2.5 py-2">
                      <span class="flex items-baseline gap-1.5">
                        <span class="num text-lg text-dim">{slot.id}</span>
                        <span class={['truncate text-[15px]', !slot.meowfficer && 'text-[#7d879c]']}>{slot.meowfficer ? (catName(slot.meowfficer) ?? `#${slot.meowfficer}`) : 'Empty'}</span>
                        <span class="ip-sub ml-auto">Style {slot.style}</span>
                      </span>
                      <span class="text-xs text-soft">{done.length ? `${done.join(' · ')} today` : 'Nothing done today'}</span>
                    </li>
                  {/each}
                </ul>
                <div class="flex flex-wrap gap-2">
                  <button type="button" class="ip-btn ip-sm" disabled={busy || data.cattery.clean >= data.cattery.clean_max} onclick={() => run('/cattery/clean', 'Cleanliness filled')}>Fill cleanliness</button>
                  <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => run('/cattery/reset-ops', 'Daily ops reset')}>Reset daily ops</button>
                </div>
              </div>
            </section>

            <section aria-label="Presets" class="ip-panel">
              <div class="ip-head"><h2>Presets</h2><span class="ip-sub">Pairs</span></div>
              <ul class="m-0 flex list-none flex-col gap-1.5 p-3">
                {#each data.presets as preset (preset.id)}
                  {@const pair = preset.meowfficers.filter(Boolean).map((c) => catName(c) ?? `#${c}`)}
                  <li class="ip-row flex items-center gap-2.5 px-3 py-2">
                    <span class="num text-[22px] text-dim">{preset.id}</span>
                    <span class="flex min-w-0 flex-1 flex-col gap-px">
                      <span class="truncate text-[15px]">{preset.name || `Preset ${preset.id}`}</span>
                      <span class="truncate text-xs text-soft">{pair.length ? pair.join(' · ') : 'Empty'}</span>
                    </span>
                    <button type="button" class="ip-danger ip-sm" disabled={busy || !pair.length} onclick={() => run(`/meowfficer-presets/${preset.id}`, 'Preset cleared', undefined, 'DELETE')}>Clear</button>
                  </li>
                {/each}
              </ul>
            </section>
          </div>
        </div>

        <aside class="sticky top-4 flex min-w-[330px] flex-[0_1_380px] flex-col gap-5">
          {#if cat}
            <section aria-label={cat.name} class="ip-card">
              <div class="ip-card-head"><h2>{cat.name}</h2></div>
              <div class="ip-card-body">
                <label class="ip-lab">Name<input class="ip-field w-full" placeholder={cat.template_name} bind:value={name} /></label>
                <div class="grid grid-cols-2 gap-2.5">
                  <label class="ip-lab">Level <input class="ip-field num w-full text-[22px]" type="number" min="1" max={cat.max_level} bind:value={level} use:bounded={{ min: 1, max: cat.max_level }} /></label>
                  <label class="ip-lab">Exp <input class="ip-field num w-full text-[22px]" type="number" min="0" bind:value={exp} use:bounded={{ min: 0, max: expMax(cat.level_exps, level) }} /></label>
                </div>
                <div role="group" aria-label="Lock" class="grid grid-cols-2 gap-2">
                  <button type="button" class="ip-opt" aria-pressed={locked} onclick={() => (locked = true)}>Locked</button>
                  <button type="button" class="ip-opt" aria-pressed={!locked} onclick={() => (locked = false)}>Unlocked</button>
                </div>
                <div class="ip-strip">Skill</div>
                <span class="text-base">{cat.skill.name || `#${cat.skill.id}`}</span>
                <div class="grid grid-cols-2 gap-2.5">
                  <label class="ip-lab">Level <input class="ip-field num w-full text-[22px]" type="number" min="1" max={cat.skill_max_level} bind:value={skillLevel} use:bounded={{ min: 1, max: cat.skill_max_level }} /></label>
                  <label class="ip-lab"
                    >Exp{#if skillLevel === cat.skill.count && cat.skill_exp_next}<span class="num text-[#6b7386]"> /{cat.skill_exp_next}</span>{/if}
                    <input class="ip-field num w-full text-[22px]" type="number" min="0" disabled={skillLevel >= cat.skill_max_level} bind:value={skillExp} use:bounded={{ min: 0, max: expMax(cat.skill_level_exps, skillLevel) }} /></label
                  >
                </div>
                <div class="ip-strip">Talents · {cat.talents.length}</div>
                <ul class="m-0 flex list-none flex-col gap-1.5 p-0">
                  {#each cat.talents as talent (talent.id)}
                    <li class="rounded bg-white px-2 py-1.5 text-[15px] shadow-[inset_0_0_0_1px_#dfe3ea]">{talent.name || `#${talent.id}`}</li>
                  {/each}
                </ul>
                <button type="button" class="ip-btn" disabled={busy} onclick={save}>Save</button>
              </div>
            </section>
          {/if}
          <section aria-label="Grant a meowfficer" class="ip-card">
            <div class="ip-card-head"><h2>Grant a meowfficer</h2></div>
            <div class="flex gap-2 bg-card px-[22px] pt-[18px] pb-5">
              <div class="min-w-0 flex-1">
                <Picker label="Meowfficer" items={templates} placeholder="Meowfficer name or id" bind:value={grant} />
              </div>
              <button type="button" class="ip-btn h-[46px] text-[19px]" disabled={busy || !grant} onclick={() => run('/meowfficers', 'Meowfficer granted', { template_id: grant })}>Grant</button>
            </div>
          </section>
        </aside>
      </div>
    {/if}
  {/snippet}
</CommanderPage>
