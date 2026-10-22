<script lang="ts">
  // Combat's META section: the boss summoned, the coordinates, every META
  // ship's Sync Points and reward steps.
  import { createQuery } from '@tanstack/svelte-query'
  import Icon from '../../../components/Icon.svelte'
  import Modal from '../../../components/Modal.svelte'
  import { bounded } from '../../../lib/bounded'
  import { count, short } from '../../../lib/format'
  import { supports, type MetaBossChoice, type MetaScreen, type MetaShip, type MetaStep } from '../../../lib/meta'
  import { InlineEdit } from '../../../lib/inline.svelte'
  import { shipFramed } from '../../../lib/players'
  import { createCommanderWrite, get, keys } from '../../../lib/query'
  import type { Meta } from '../../../lib/social'

  let { id }: { id: number; meta?: Meta } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'meta'),
    queryFn: get<MetaScreen>(`/commanders/${id}/meta`),
  }))
  const data = $derived(query.data)

  const { mutation: edit, run } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)

  // a number edited in place
  const inline = new InlineEdit()

  // the ships whose reward steps are shown
  let open = $state<Record<number, boolean>>({})

  function toggleStep(ship: MetaShip, step: MetaStep) {
    const done = step.claimed ? `${ship.name}: rewards from ${count(step.target)} unclaimed` : `${ship.name}: rewards up to ${count(step.target)} claimed`
    run('/meta/award', done, { group: ship.group, target: step.target, claimed: !step.claimed })
  }

  // the summon dialog
  let summonModal = $state<Modal>()
  let choice = $state<MetaBossChoice | null>(null)
  let level = $state(1)
  function openSummon(screen: MetaScreen) {
    pick(screen.bosses.find((boss) => boss.current) ?? screen.bosses[0] ?? null, screen)
    summonModal?.show()
  }
  function pick(boss: MetaBossChoice | null, screen: MetaScreen) {
    choice = boss
    if (boss) level = Math.min(Math.max(boss.current ? screen.self_boss_lv : boss.max_level, boss.min_level), boss.max_level)
  }
  function summon(event: SubmitEvent) {
    event.preventDefault()
    if (!choice) return
    run('/meta/boss', `${choice.name} Lv.${level} summoned`, { template: choice.template, level }, 'POST', () => summonModal?.close())
  }
</script>

{#snippet number(key: string, current: number, max: number, label: string, path: string, done: (v: number) => string, body: (v: number) => Record<string, unknown>, size = 'text-xl')}
  {#if inline.editing === key}
    <form class="flex items-center gap-1.5" onsubmit={(e) => inline.save(e, current, (v) => run(path, done(v), body(v)))}>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="ip-field num h-9 w-32 min-w-0 text-lg" type="number" aria-label={label} autofocus bind:value={inline.value} use:bounded={{ min: 0, max }} onkeydown={inline.escape} />
      <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
    </form>
  {:else}
    <button type="button" aria-label="Edit {label}" class="cursor-pointer rounded border-0 bg-transparent px-1 text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start(key, current)}>
      <span class={['num', size]}>{count(current)}</span>
    </button>
  {/if}
{/snippet}

<section aria-label="META" class="ip-panel">
  <div class="ip-head items-center">
    <h2>META</h2>
  </div>
  {#if !data}
    <div class="px-3.5 py-3.5 text-dim">{query.isError ? query.error.message : 'Loading'}</div>
  {:else}
    {@const screen = data}
    <div class="grid grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)] gap-3.5 px-3.5 pt-3.5">
      <div class="ip-row flex flex-col gap-2.5 px-3.5 py-3.5">
        {#if screen.boss}
          {@const boss = screen.boss}
          {@const hp = boss.max_hp ? Math.round((boss.hp / boss.max_hp) * 100) : 0}
          <span class="flex items-baseline gap-2.5">
            <span class="ip-sub">Boss</span>
            <span class="truncate text-xl">{screen.boss_name || `#${boss.template_id}`}</span>
            <span class="text-sm text-dim">Lv.<span class="num">{boss.level}</span></span>
            <button type="button" class="ip-btn2 ip-sm ml-auto self-center" disabled={busy || !screen.bosses.length} onclick={() => openSummon(screen)}>Summon</button>
          </span>
          <span class="flex items-center gap-3">
            <span class="ip-sub w-14">HP</span>
            <span aria-hidden="true" class="h-2 flex-1 rounded bg-black/40"><span class="block h-full rounded bg-red" style:width="{hp}%"></span></span>
            {@render number('hp', boss.hp, boss.max_hp, 'Boss HP', '/meta/boss/hp', (v) => (v ? `Boss HP ${count(v)}` : 'Boss killed'), (v) => ({ hp: v }), 'text-lg')}
            <span class="num text-sm text-dim">/{count(boss.max_hp)}</span>
            <button type="button" class="ip-danger ip-sm" disabled={busy || boss.hp === 0} onclick={() => run('/meta/boss/hp', 'Boss killed', { hp: 0 })}>Kill</button>
          </span>
          <span class="flex items-center gap-3 text-[15px] text-soft">
            <span class="ip-sub w-14">Fought</span>
            <span class="num text-lg text-white">{boss.fights}</span>
            <span>Damage <span class="num text-lg text-white">{count(boss.damage)}</span></span>
            <button type="button" class="ip-btn2 ip-sm ml-auto" disabled={busy || boss.fights === 0} onclick={() => run('/meta/boss/fights/reset', 'Fights reset')}>Reset fights</button>
          </span>
          <span class="flex items-center gap-3 text-[15px] text-soft">
            <span class="ip-sub w-14">Ends</span>
            <span class="num text-lg text-white">{short(boss.expires_at, '/')}</span>
            <span class="ml-auto flex gap-1.5">
              <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => run('/meta/boss/timer', 'Timer extended', { extend: true })}>Extend</button>
              <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => run('/meta/boss/timer', 'Timer reset', { extend: false })}>Reset timer</button>
            </span>
          </span>
          <span class="flex flex-wrap gap-1.5">
            {#each supports as support (support.type)}
              {@const on = boss[support.key]}
              <button type="button" aria-pressed={on} class={['ip-tag cursor-pointer border-0 text-[13px] hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]', on ? 'bg-[#2f9a55] text-white' : 'bg-white/10 text-dim']} disabled={busy} onclick={() => run('/meta/boss/support', `${support.label} ${on ? 'off' : 'on'}`, { type: support.type, on: !on })}>{support.label}</button>
            {/each}
          </span>
        {:else}
          <span class="flex items-baseline gap-2.5">
            <span class="ip-sub">Boss</span>
            <span class="text-lg text-dim">None summoned</span>
            <button type="button" class="ip-btn2 ip-sm ml-auto self-center" disabled={busy || !screen.bosses.length} onclick={() => openSummon(screen)}>Summon</button>
          </span>
        {/if}
        {#each screen.bosses.filter((boss) => boss.current) as season (season.template)}
          <span class="text-[15px] text-soft">Season <span class="text-white">{season.name}</span> · next Lv.<span class="num text-lg text-white">{screen.self_boss_lv}</span></span>
        {/each}
      </div>
      <div class="ip-row flex flex-col gap-3 self-start px-3.5 py-3.5">
        <span class="flex items-center gap-3">
          <span class="ip-sub w-28">Coordinates</span>
          {@render number('summon_pt', screen.summon_pt, screen.summon_pt_max, 'META Coordinates', '/meta/summon-pt', (v) => `${count(v)} Coordinates`, (v) => ({ value: v, past: false }))}
          <span class="num text-sm text-dim">/{count(screen.summon_pt_max)}</span>
        </span>
        <span class="flex items-center gap-3">
          <span class="ip-sub w-28">Dossier</span>
          {@render number('summon_pt_old', screen.summon_pt_old, screen.summon_pt_old_max, 'Dossier Coordinate Data', '/meta/summon-pt', (v) => `${count(v)} Dossier Coordinate Data`, (v) => ({ value: v, past: true }))}
          <span class="num text-sm text-dim">/{count(screen.summon_pt_old_max)}</span>
        </span>
      </div>
    </div>

    <div class="flex flex-col gap-1.5 px-3.5 pt-3.5 pb-3.5">
      <div class="grid grid-cols-[minmax(0,1.6fr)_minmax(0,1.4fr)_96px] gap-3 px-3 text-dim">
        <span class="ip-sub">Ship</span><span class="ip-sub">Sync points</span><span class="ip-sub text-right">Rewards</span>
      </div>
      {#each screen.ships as ship (ship.group)}
        {@const claimed = ship.steps.filter((step) => step.claimed).length}
        <div class="ip-row">
          <div class="grid grid-cols-[minmax(0,1.6fr)_minmax(0,1.4fr)_96px] items-center gap-3 px-3 py-1.5">
            <span class="flex min-w-0 items-center gap-3">
              <Icon icon={ship.icon} rarity={ship.rarity} ship name={ship.name} size={40} />
              <span class="truncate text-[17px]">{ship.name || `#${ship.group}`}</span>
              {#if ship.owned}<span class="ip-tag bg-cyan text-white">Owned</span>{/if}
            </span>
            {#if inline.editing === `pt:${ship.group}`}
              <form class="flex items-center gap-1.5" onsubmit={(e) => inline.save(e, ship.pt, (v) => run('/meta/pt', `${ship.name}: ${v} Sync Points`, { group: ship.group, pt: v }))}>
                <!-- svelte-ignore a11y_autofocus -->
                <input class="ip-field num h-9 min-w-0 flex-1 text-lg" type="number" aria-label="{ship.name} Sync Points" autofocus bind:value={inline.value} use:bounded={{ min: 0, max: ship.max }} onkeydown={inline.escape} />
                <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
              </form>
            {:else}
              <button type="button" aria-label="Edit {ship.name}'s Sync Points" class="flex cursor-pointer items-center gap-2.5 rounded border-0 bg-transparent p-1 text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start(`pt:${ship.group}`, ship.pt)}>
                <span class="num w-16 text-right text-xl">{count(ship.pt)}</span>
                <span aria-hidden="true" class="h-1.5 flex-1 rounded-[3px] bg-black/40"><span class="block h-full rounded-[3px] bg-cyan" style:width="{ship.max ? Math.min(100, (ship.pt / ship.max) * 100) : 0}%"></span></span>
              </button>
            {/if}
            <button type="button" aria-expanded={!!open[ship.group]} aria-label="{ship.name}'s rewards" class="flex cursor-pointer items-center justify-end gap-1.5 rounded border-0 bg-transparent p-1 text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => (open[ship.group] = !open[ship.group])}>
              <span class="num text-xl">{claimed}</span><span class="num text-sm text-dim">/{ship.steps.length}</span>
              <svg aria-hidden="true" viewBox="0 0 24 24" class={['size-4 text-dim transition-transform', open[ship.group] && 'rotate-180']} fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M6 9l6 6 6-6" /></svg>
            </button>
          </div>
          {#if open[ship.group]}
            <div class="grid grid-cols-[repeat(auto-fill,minmax(92px,1fr))] gap-1.5 border-t border-white/8 px-3 py-2.5">
              {#each ship.steps as step (step.target)}
                <button
                  type="button"
                  aria-pressed={step.claimed}
                  aria-label="{count(step.target)}: {step.award.name ?? ''} x{step.award.count}"
                  class={['flex cursor-pointer items-center gap-2 rounded-[3px] border-0 px-1.5 py-1 text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)] disabled:cursor-not-allowed disabled:opacity-40', step.claimed ? 'bg-[#2f9a55]' : 'bg-black/30']}
                  disabled={busy || (!step.claimed && ship.pt < step.target)}
                  onclick={() => toggleStep(ship, step)}
                >
                  <Icon icon={step.award.icon} rarity={step.award.rarity} ship={shipFramed(step.award.type)} name={step.award.name} count={step.award.count} size={34} label={false} />
                  <span class="num text-[15px]">{count(step.target)}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <Modal bind:this={summonModal} title="Summon" width="w-[min(640px,92vw)]">
      <form class="flex flex-col gap-4" onsubmit={summon}>
        <div class="grid grid-cols-[repeat(auto-fill,minmax(168px,1fr))] gap-1.5">
          {#each screen.bosses as boss (boss.template)}
            <button type="button" aria-pressed={choice?.template === boss.template} class={['flex cursor-pointer items-center gap-2.5 rounded-[3px] border-0 bg-white/6 px-2 py-1.5 text-left text-white hover:bg-white/10', choice?.template === boss.template && 'ip-sel']} onclick={() => pick(boss, screen)}>
              <Icon icon={boss.icon} rarity={boss.rarity} ship name={boss.name} size={40} />
              <span class="flex min-w-0 flex-col">
                <span class="truncate text-[15px]">{boss.name || `#${boss.group}`}</span>
                <span class="ip-sub">{boss.current ? 'Season' : 'Dossier'}</span>
              </span>
            </button>
          {/each}
        </div>
        {#if choice}
          <div class="flex items-center gap-3">
            <label class="flex items-center gap-3">
              <span class="ip-sub">Level</span>
              <input class="ip-field num h-10 w-24 text-lg" type="number" aria-label="Level" bind:value={level} disabled={choice.min_level === choice.max_level} use:bounded={{ min: choice.min_level, max: choice.max_level }} />
            </label>
            <button type="submit" class="ip-btn ip-sm ml-auto" disabled={busy}>Summon</button>
          </div>
        {/if}
      </form>
    </Modal>
  {/if}
</section>
