<script lang="ts">
  // The commander's events: the list of what they keep state of, then the
  // event picked as its page shows it in the game, each part with its own
  // edits: tasks, the cruise pass, login calendars, a Cheater's Tavern
  // season, Juustagram. Other systems show their stored rows.
  import { untrack } from 'svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import Seg from '../../components/Seg.svelte'
  import { api } from '../../lib/api'
  import { bounded, uint32 } from '../../lib/bounded'
  import { count, pad, stamp } from '../../lib/format'
  import { fuzzy } from '../../lib/fuzzy'
  import { link } from '../../lib/router.svelte'
  import { InlineEdit } from '../../lib/inline.svelte'
  import { categoryColors, type LifeActivity, type LifeStatus, type LifeTable } from '../../lib/life'
  import { shipFramed } from '../../lib/players'
  import { createCommanderMutation, createCommanderWrite, done, get, keys, type Kicked } from '../../lib/query'
  import type { ActivityDetail, ActivityGroup, ActivityLevel, ActivityReward, ActivitySigns, ActivityTask } from '../../lib/activities'

  let { id }: { id: number } = $props()

  const activities = createQuery(() => ({
    queryKey: keys.screen(id, 'activities'),
    queryFn: get<LifeActivity[]>(`/commanders/${id}/activities`),
  }))
  const list = $derived(activities.data)

  let filter = $state<'all' | LifeStatus>('all')
  let query = $state('')
  let picked = $state('')
  const [debounced] = createDebouncedValue(() => query, { wait: 150 })

  const shown = $derived(
    fuzzy(
      (list ?? []).filter((a) => filter === 'all' || a.status === filter),
      debounced(),
      (a) => [a.name, a.activity, a.kind, a.category, a.progress],
    ),
  )
  // the activity picked, or the first one
  const selected = $derived(list?.find((a) => a.key === picked) ?? list?.[0])
  const filters = $derived([
    { value: 'all' as const, label: `All ${list?.length ?? 0}` },
    { value: 'running' as const, label: 'Running' },
    { value: 'permanent' as const, label: 'Permanent' },
    { value: 'finished' as const, label: 'Finished' },
  ])

  const lights: Record<LifeStatus, string> = { running: 'bg-[#4cd07d]', permanent: 'bg-cyan', finished: 'bg-[#6b7386]' }
  const statusTags: Record<LifeStatus, string> = { running: 'bg-[#2f9e5c] text-white', permanent: 'bg-cyan text-[#0b2a3a]', finished: 'bg-[#6b7386] text-white' }

  // the events list, virtualized: a commander can keep hundreds
  const rowHeight = 47
  let scroller = $state<HTMLDivElement | null>(null)
  const virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: 0,
    getScrollElement: () => scroller,
    estimateSize: () => rowHeight,
    overscan: 8,
  })
  $effect(() => {
    const count = shown.length
    const element = scroller
    untrack(() => {
      $virtualizer.setOptions({ count, getScrollElement: () => element })
      $virtualizer.measure()
    })
  })
  $effect(() => {
    void [debounced(), filter]
    scroller?.scrollTo({ top: 0 })
  })
  const columns = 'grid grid-cols-[minmax(0,1.4fr)_150px_minmax(0,1fr)_104px] items-center'

  // the selected event as its page reads it
  const detailQuery = createQuery(() => ({
    queryKey: keys.screen(id, 'activities', selected?.key ?? ''),
    queryFn: get<ActivityDetail>(`/commanders/${id}/activities/detail?key=${encodeURIComponent(selected?.key ?? '')}`),
    enabled: !!selected,
  }))
  const detail = $derived(detailQuery.data)

  // unix times kept in fields named so
  function value(table: LifeTable, column: number, v: string | number | boolean): string {
    const name = table.columns[column]
    if (typeof v === 'number' && v > 1_000_000_000 && v < 4_000_000_000 && /(time|_at|day|week|month|refresh|timestamp)$/.test(name)) return stamp(v)
    if (typeof v === 'boolean') return v ? 'yes' : 'no'
    return String(v)
  }

  const clear = createCommanderMutation(
    () => id,
    ({ path, activity }: { path: 'reset' | 'delete'; activity: LifeActivity }) =>
      api<Kicked>('POST', `/commanders/${id}/activities/${path}`, { key: activity.key }),
    { success: (result, { path, activity }) => done(path === 'reset' ? `${activity.name} progress reset` : `${activity.name} state deleted`)(result) },
  )

  function act(path: 'reset' | 'delete') {
    if (!selected) return
    const what = path === 'reset' ? `Reset the task progress of ${selected.name}?` : `Delete everything ${selected.name} stored for this commander?`
    if (confirm(what)) clear.mutate({ path, activity: selected })
  }

  // the detail's edits
  const { mutation: edit, run: send } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending || clear.isPending)
  const run = (path: string, message: string, body: Record<string, unknown>) => selected && send(`/activities/${path}`, message, { key: selected.key, ...body })

  // a value edited in place, closed when another event is picked
  const inline = new InlineEdit({ reset: () => selected?.key })

  // Tasks
  const shortDesc = (task: ActivityTask) => (task.desc || `Task ${task.id}`).replace(/\s*\n\s*/g, ' ')
  function saveProgress(event: SubmitEvent, task: ActivityTask) {
    inline.submit(event)
    if (inline.value !== task.progress || !task.held) run('task', `${shortDesc(task)}: ${inline.value} / ${task.target}`, { task: task.id, progress: inline.value, claimed: false })
  }
  const finish = (task: ActivityTask) => run('task', `${shortDesc(task)} finished`, { task: task.id, progress: task.target, claimed: false })
  const claim = (task: ActivityTask, claimed: boolean) =>
    run('task', claimed ? `${shortDesc(task)} claimed` : `${shortDesc(task)} unclaimed`, { task: task.id, progress: Math.min(task.progress, task.target), claimed })
  const resetTask = (task: ActivityTask) => run('task/reset', `${shortDesc(task)} reset`, { task: task.id })

  // the tab of tasks shown (a cruise week, a task list day)
  let tab = $derived.by<number | null>(() => (selected?.key, null))
  const groups = $derived(detail?.groups ?? [])
  const shownGroup = $derived(groups.find((g) => g.id === tab) ?? groups.find((g) => !g.locked && g.tasks.some((t) => !t.claimed)) ?? groups[0])
  const groupDone = (group: ActivityGroup) => group.tasks.filter((t) => t.claimed).length
  // the rewards column fits the most rewards a task gives
  const awardsWidth = $derived(Math.max(44, ...groups.flatMap((g) => g.tasks.map((t) => t.awards.length * 44 - 4))))

  // Cruise pass
  const levelOf = (levels: ActivityLevel[], pt: number) => levels.filter((l) => l.pt <= pt).length
  function savePass(event: SubmitEvent, shown: number) {
    inline.save(event, shown, (v) => run('cruise', `Pass set to ${count(v)} pt`, { pt: v }))
  }
  function pickLevel(levels: ActivityLevel[], level: number) {
    inline.value = level === 0 ? 0 : (levels[level - 1]?.pt ?? inline.value)
  }
  const toggleLevel = (level: ActivityLevel) =>
    run('cruise/award', level.free_claimed ? `Phase ${level.level} unclaimed` : `Phase ${level.level} claimed`, { level: level.level, claimed: !level.free_claimed })

  // Sign-ins
  function signDay(signs: ActivitySigns, day: number) {
    const next = day <= signs.count ? day - 1 : day
    run('signs', next ? `Signed through day ${next}` : 'No day signed', { count: next })
  }
  function saveSigns(event: SubmitEvent, signs: ActivitySigns) {
    inline.save(event, signs.count, (v) => run('signs', `${v} days signed`, { count: v }))
  }

  // Cheater's Tavern
  function saveScore(event: SubmitEvent, score: number, best: number) {
    inline.submit(event)
    const { value, second } = inline
    if (value !== score || second !== best) run('score', `Score ${count(value)}, best ${count(Math.max(value, second))}`, { score: value, best: Math.max(value, second) })
  }
</script>

{#snippet reward(drop: ActivityReward, size = 40)}
  <Icon icon={drop.icon} rarity={drop.rarity} ship={shipFramed(drop.type)} name={drop.name || `${drop.type}:${drop.id}`} count={drop.count} {size} label={false} />
{/snippet}

{#snippet taskRow(task: ActivityTask, locked: boolean, day = '')}
  {@const ready = task.held && !task.claimed && task.progress >= task.target}
  <div class={['ip-row grid items-center gap-3 px-2.5 py-1.5', locked && 'opacity-60']} style:grid-template-columns="{day ? '72px ' : ''}{awardsWidth}px minmax(0,1fr) minmax(150px,200px) 212px">
    {#if day}
      <span class="flex flex-col">
        <span class="text-[15px]">{day}</span>
        {#if locked}<span class="ip-tag self-start bg-[#6b7386] text-white uppercase">Locked</span>{/if}
      </span>
    {/if}
    <span class="flex gap-1">
      {#each task.awards as drop, i (i)}{@render reward(drop)}{/each}
    </span>
    <span class="flex min-w-0 flex-col">
      <span class="truncate text-[16px]" title={task.desc}>{shortDesc(task)}</span>
      <span class="flex items-center gap-2 text-xs text-dim">
        <span class="num">{task.id}</span>
        {#if task.claimed}<span class="ip-tag bg-[#2f9e5c] text-white uppercase">{detail?.labels.got || 'Claimed'}</span>
        {:else if ready}<span class="ip-tag bg-cyan text-[#0b2a3a] uppercase">{detail?.labels.get || 'Ready'}</span>
        {:else if !task.held}<span class="ip-tag bg-[#6b7386] text-white uppercase">Not given</span>{/if}
      </span>
    </span>
    {#if inline.editing === `task:${task.id}`}
      <form class="flex items-center gap-1.5" onsubmit={(e) => saveProgress(e, task)}>
        <!-- svelte-ignore a11y_autofocus -->
        <input class="ip-field num h-9 w-24 min-w-0 text-lg" type="number" aria-label="Progress" autofocus bind:value={inline.value} use:bounded={{ min: 0, max: task.target }} onkeydown={inline.escape} />
        <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
      </form>
    {:else}
      <button type="button" aria-label="Edit the progress of {shortDesc(task)}" disabled={busy || task.claimed} class="flex cursor-pointer flex-col gap-1 rounded border-0 bg-transparent px-1 py-0.5 text-left text-white enabled:hover:shadow-[inset_0_0_0_2px_var(--color-cyan)] disabled:cursor-default" onclick={() => inline.start(`task:${task.id}`, Math.min(task.progress, task.target))}>
        <span><span class="num text-xl">{count(task.progress)}</span><span class="num text-sm text-dim"> / {count(task.target)}</span></span>
        <span aria-hidden="true" class="block h-1.5 w-full rounded-[3px] bg-black/40"><span class={['block h-full rounded-[3px]', task.claimed ? 'bg-[#2f9e5c]' : 'bg-cyan']} style:width="{task.target ? Math.min(100, (task.progress / task.target) * 100) : 0}%"></span></span>
      </button>
    {/if}
    <span class="flex justify-end gap-1.5">
      {#if !task.claimed && task.progress < task.target}
        <button type="button" class="ip-btn2 ip-sm h-8 px-2.5 text-sm" disabled={busy} onclick={() => finish(task)}>Finish</button>
      {/if}
      <button type="button" class={[task.claimed ? 'ip-btn2' : 'ip-btn', 'ip-sm h-8 px-2.5 text-sm']} disabled={busy} onclick={() => claim(task, !task.claimed)}>{task.claimed ? 'Unclaim' : 'Claim'}</button>
      <button type="button" class="ip-btn2 ip-sm h-8 px-2.5 text-sm" disabled={busy || (!task.claimed && task.progress === 0)} onclick={() => resetTask(task)}>Reset</button>
    </span>
  </div>
{/snippet}

{#snippet taskGroups(title: string)}
  {#if detail?.kind === 'tasklist'}
    <div class="flex flex-col gap-2.5">
      <h3 class="m-0 flex items-center gap-2.5 text-xl">
        {title}
        <span class="text-[15px] text-soft"><span class="num text-lg text-white">{groups.reduce((n, g) => n + groupDone(g), 0)}</span> / <span class="num">{groups.reduce((n, g) => n + g.tasks.length, 0)}</span></span>
      </h3>
      <div class="flex max-h-[640px] flex-col gap-1.5 overflow-y-auto pr-1">
        {#each groups as group (group.id)}
          {#each group.tasks as task (task.id)}{@render taskRow(task, group.locked, group.label)}{/each}
        {/each}
      </div>
    </div>
  {:else if groups.length > 1}
    <div class="flex flex-col gap-2.5">
      <h3 class="m-0 flex flex-wrap items-center gap-2.5 text-xl">
        {title}
        <span class="text-[15px] text-soft"><span class="num text-lg text-white">{groups.reduce((n, g) => n + groupDone(g), 0)}</span> / <span class="num">{groups.reduce((n, g) => n + g.tasks.length, 0)}</span></span>
      </h3>
      <div role="group" aria-label={title} class="flex flex-wrap gap-1.5">
        {#each groups as group (group.id)}
          <button type="button" aria-pressed={group.id === shownGroup?.id} class={['flex cursor-pointer items-center gap-2 rounded-[3px] border-0 px-3 py-1.5 text-[15px] text-white', group.id === shownGroup?.id ? 'bg-cyan text-[#0b2a3a]' : 'bg-white/8 hover:bg-white/14', group.locked && 'opacity-60']} onclick={() => (tab = group.id)}>
            {group.label}
            <span class="num text-sm">{groupDone(group)}/{group.tasks.length}</span>
            {#if group.locked}<svg aria-label="Locked" viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="currentColor" stroke-width="2.6"><rect x="5" y="11" width="14" height="9" rx="1.5" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></svg>{/if}
          </button>
        {/each}
      </div>
      <div class="flex max-h-[640px] flex-col gap-1.5 overflow-y-auto pr-1">
        {#each shownGroup?.tasks ?? [] as task (task.id)}{@render taskRow(task, shownGroup?.locked ?? false)}{/each}
      </div>
    </div>
  {:else}
    <div class="flex flex-col gap-2.5">
      <h3 class="m-0 flex items-center gap-2.5 text-xl">
        {title}
        <span class="text-[15px] text-soft"><span class="num text-lg text-white">{groups[0] ? groupDone(groups[0]) : 0}</span> / <span class="num">{groups[0]?.tasks.length ?? 0}</span></span>
      </h3>
      <div class="flex max-h-[640px] flex-col gap-1.5 overflow-y-auto pr-1">
        {#each groups[0]?.tasks ?? [] as task (task.id)}{@render taskRow(task, groups[0].locked)}{:else}<div class="ip-row px-3 py-2.5 text-dim">No task</div>{/each}
      </div>
    </div>
  {/if}
{/snippet}

{#snippet signsView(signs: ActivitySigns, monthly: boolean)}
  <div class="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
    {#if inline.editing === 'signs'}
      <form class="ip-row flex flex-col gap-1.5 px-4 py-2.5" onsubmit={(e) => saveSigns(e, signs)}>
        <span class="ip-sub">Signed</span>
        <span class="flex items-center gap-1.5">
          <!-- svelte-ignore a11y_autofocus -->
          <input class="ip-field num h-9 min-w-0 flex-1 text-lg" type="number" aria-label="Days signed" autofocus bind:value={inline.value} use:bounded={{ min: 0, max: signs.max }} onkeydown={inline.escape} />
          <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
        </span>
      </form>
    {:else}
      <button type="button" aria-label="Edit the days signed" disabled={busy} class="ip-row flex cursor-pointer flex-col gap-1 border-0 px-4 py-2.5 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start('signs', signs.count)}>
        <span class="ip-sub">Signed</span>
        <span><span class="num text-2xl">{signs.count}</span><span class="num text-base text-dim"> / {signs.max}</span></span>
      </button>
    {/if}
    {#if monthly}
      <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
        <span class="ip-sub">Month</span>
        <span class="num text-2xl">{signs.year}-{pad(signs.month ?? 0)}</span>
      </div>
      <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
        <span class="ip-sub">Dates</span>
        <span class="num truncate text-lg" title={signs.dates.join(', ')}>{signs.dates.length ? signs.dates.join(', ') : '-'}</span>
      </div>
      <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
        <span class="ip-sub">Re-signs</span>
        <span class="num text-2xl">{signs.resigns}</span>
      </div>
    {:else}
      <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
        <span class="ip-sub">Last sign</span>
        <span class="num text-2xl">{signs.last ? stamp(signs.last) : '-'}</span>
      </div>
    {/if}
  </div>
  <div class="grid grid-cols-[repeat(auto-fill,minmax(92px,1fr))] gap-1.5">
    {#each signs.days as drops, i (i)}
      {@const day = i + 1}
      {@const signed = day <= signs.count}
      <button
        type="button"
        aria-pressed={signed}
        aria-label="Day {day}"
        disabled={busy}
        class={['flex cursor-pointer flex-col items-center gap-1 rounded-[3px] border-0 px-1.5 pt-1.5 pb-2 text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]', signed ? 'bg-[#2f9a55]' : 'bg-black/30', day === signs.count && 'shadow-[inset_0_0_0_2px_var(--color-gold)]']}
        onclick={() => signDay(signs, day)}
      >
        <span class="text-sm">Day <span class="num">{day}</span></span>
        {#each drops.slice(0, 1) as drop, d (d)}{@render reward(drop, 52)}{/each}
      </button>
    {/each}
  </div>
{/snippet}

<CommanderPage {id} title="Activities">
  {#snippet children()}
    {#if list}
      <div class="grid items-start gap-5 min-[1800px]:grid-cols-[640px_minmax(0,1fr)]">
        <section aria-label="Events" class="ip-panel min-w-0 overflow-hidden">
          <div class="ip-head flex-wrap items-center gap-y-2.5">
            <h2>Events</h2>
            <div class="ml-auto"><Seg label="Show" options={filters} bind:value={filter} /></div>
            <label class="relative basis-full">
              <span class="sr-only">Find an event</span>
              <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute top-1/2 left-3 size-[18px] -translate-y-1/2" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
              <input type="search" class="ip-field h-10 w-full pl-[38px]" placeholder="Event" bind:value={query} />
            </label>
          </div>
          {#if shown.length === 0}
            <p class="m-0 px-5 py-6 text-soft">{list.length ? 'No match' : 'No stored state'}</p>
          {:else}
            <div role="table" aria-label="Events" aria-rowcount={shown.length + 1}>
              <div role="row" class={[columns, 'mx-2.5 pt-2 pb-0.5']}>
                <span role="columnheader" class="ip-sub px-3">EVENT</span>
                <span role="columnheader" class="ip-sub px-2.5">KIND</span>
                <span role="columnheader" class="ip-sub px-2.5">PROGRESS</span>
                <span role="columnheader" class="ip-sub px-3">ENDS</span>
              </div>
              <div bind:this={scroller} role="rowgroup" class="max-h-[420px] overflow-y-auto px-2.5 pb-3 min-[1800px]:max-h-[760px]">
                <div class="relative w-full" style:height="{$virtualizer.getTotalSize()}px">
                  {#each $virtualizer.getVirtualItems() as item (item.key)}
                    {@const row = shown[item.index]}
                    {#if row}
                      {@const colors = categoryColors[row.category] ?? categoryColors.event}
                      <div role="row" aria-rowindex={item.index + 2} class="absolute top-0 left-0 w-full pt-[5px]" style:height="{rowHeight}px" style:transform="translateY({item.start}px)">
                        <div class={[columns, 'h-full rounded-[3px] bg-white/6 hover:bg-white/10', row.key === selected?.key && 'ip-sel']}>
                          <span role="rowheader" class="min-w-0 px-3 font-bold">
                            <button type="button" class="flex w-full min-w-0 cursor-pointer items-center gap-2.5 border-0 bg-transparent p-0 text-left font-game font-bold text-white" aria-pressed={row.key === selected?.key} onclick={() => (picked = row.key)}>
                              <span aria-hidden="true" class={['h-6 w-[7px] shrink-0', lights[row.status]]}></span>
                              <span class="truncate" title={row.name}>{row.name}{#if row.activity}<span class="num ml-2 text-sm text-dim">{row.activity}</span>{/if}</span>
                            </button>
                          </span>
                          <span role="cell" class="min-w-0 px-2.5"><span class="ip-tag inline-block max-w-full truncate align-middle uppercase" title={row.kind} style:background={colors[0]} style:color={colors[1]}>{row.kind}</span></span>
                          <span role="cell" class="truncate px-2.5 text-soft" title={row.progress}>{row.progress || '-'}</span>
                          <span role="cell" class="num px-3 text-lg whitespace-nowrap text-soft">{row.end ? stamp(row.end).slice(5, 10) : '-'}</span>
                        </div>
                      </div>
                    {/if}
                  {/each}
                </div>
              </div>
            </div>
          {/if}
        </section>

        {#if selected}
          <section aria-label={selected.name} class="ip-panel min-w-0">
            <div class="ip-head flex-wrap items-center gap-y-2">
              <h2>{selected.name}</h2>
              <span class={['ip-tag uppercase', statusTags[selected.status]]}>{selected.status}</span>
              {#if selected.activity}<span class="num text-lg text-dim">{selected.activity}</span>{/if}
              {#if selected.start}<span class="text-[15px] text-soft"><span class="num text-lg text-white">{stamp(selected.start).slice(0, 10)}</span> to <span class="num text-lg text-white">{stamp(selected.end).slice(0, 10)}</span></span>{/if}
              <span class="ml-auto flex gap-2">
                {#if selected.tasks > 0}
                  <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => act('reset')}>Reset progress</button>
                {/if}
                <button type="button" class="ip-danger ip-sm" disabled={busy} onclick={() => act('delete')}>Delete state</button>
              </span>
            </div>
            <div class="flex flex-col gap-6 px-5 pt-4 pb-5">
              {#if detailQuery.isError}
                <p class="m-0 text-bad">{detailQuery.error.message}</p>
              {:else if !detail}
                <p class="m-0 text-dim">Loading</p>
              {:else if detail.kind === 'cruise' && detail.cruise}
                {@const pass = detail.cruise}
                {@const reached = pass.levels.filter((l) => l.pt <= pass.shown).length}
                {@const claimed = pass.levels.filter((l) => l.free_claimed).length}
                {@const next = pass.levels[pass.level]}
                {@const from = pass.levels[pass.level - 1]?.pt ?? 0}
                <div class="grid grid-cols-[minmax(0,2fr)_repeat(3,minmax(0,1fr))] gap-3">
                  {#if inline.editing === 'pass'}
                    <form class="ip-row flex flex-col gap-2 px-4 py-3" onsubmit={(e) => savePass(e, pass.shown)}>
                      <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-2">
                        <!-- svelte-ignore a11y_autofocus -->
                        <input class="ip-field num h-10 text-xl" type="number" aria-label={pass.item.name || 'Pt'} autofocus bind:value={inline.value} use:bounded={{ min: 0, max: pass.max }} onkeydown={inline.escape} />
                        <select class="ip-field h-10" aria-label={detail.labels.phase || 'Phase'} value={levelOf(pass.levels, inline.value)} onchange={(e) => pickLevel(pass.levels, Number(e.currentTarget.value))}>
                          <option value={0}>{detail.labels.phase} 0</option>
                          {#each pass.levels as level (level.level)}<option value={level.level}>{detail.labels.phase} {level.level}</option>{/each}
                        </select>
                      </div>
                      <div class="flex justify-end gap-2">
                        <button type="button" class="ip-btn2 ip-sm" onclick={() => inline.stop()}>Cancel</button>
                        <button type="submit" class="ip-btn ip-sm" disabled={busy}>Save</button>
                      </div>
                    </form>
                  {:else}
                    <button type="button" aria-label="Edit the pass" disabled={busy} class="ip-row flex cursor-pointer flex-col gap-1.5 border-0 px-4 py-3 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start('pass', pass.shown)}>
                      <span class="ip-sub">{detail.labels.phase}</span>
                      <span class="flex flex-wrap items-center gap-x-2.5">
                        <span class="num text-[30px] text-gold">{pass.level}</span>
                        <span class="num text-base text-dim">/ {pass.levels.length}</span>
                        <span class="ml-auto flex items-center gap-2">
                          <Icon icon={pass.item.icon} rarity={pass.item.rarity} name={pass.item.name} size={30} label={false} />
                          <span class="num text-[22px]">{count(pass.shown)}</span>
                        </span>
                      </span>
                      <span aria-hidden="true" class="block h-1.5 w-full rounded-[3px] bg-black/40"><span class="block h-full rounded-[3px] bg-cyan" style:width="{next ? Math.max(0, Math.min(100, ((pass.shown - from) / Math.max(1, next.pt - from)) * 100)) : 100}%"></span></span>
                      <span class="text-[13px] text-dim">{#if next}{detail.labels.phase} {next.level} at <span class="num text-base text-soft">{count(next.pt)}</span>{:else}{count(pass.max)}{/if}{#if pass.pt !== pass.shown} · Counter <span class="num text-base text-soft">{count(pass.pt)}</span>{/if}</span>
                    </button>
                  {/if}
                  <div class="ip-row flex flex-col gap-1.5 px-4 py-3">
                    <span class="ip-sub">{detail.labels.bonus}</span>
                    <span class={['text-xl', pass.paid ? 'text-ok' : 'text-dim']}>{pass.paid ? 'Unlocked' : 'Locked'}</span>
                    <button type="button" class={[pass.paid ? 'ip-btn2' : 'ip-btn', 'ip-sm h-8 self-start px-2.5 text-sm']} disabled={busy} onclick={() => run('cruise', pass.paid ? `${detail.labels.bonus} locked` : `${detail.labels.bonus} unlocked`, { paid: !pass.paid })}>{pass.paid ? 'Lock' : 'Unlock'}</button>
                  </div>
                  <div class="ip-row flex flex-col gap-1 px-4 py-3">
                    <span class="ip-sub">{detail.labels.base}</span>
                    <span><span class="num text-[30px]">{claimed}</span><span class="num text-base text-dim"> / {reached}</span></span>
                  </div>
                  <div class="ip-row flex flex-col gap-1 px-4 py-3">
                    <span class="ip-sub">Missions</span>
                    <span><span class="num text-[30px]">{groups.reduce((n, g) => n + groupDone(g), 0)}</span><span class="num text-base text-dim"> / {groups.reduce((n, g) => n + g.tasks.length, 0)}</span></span>
                  </div>
                </div>

                <div class="flex flex-col gap-2.5">
                  <h3 class="m-0 flex flex-wrap items-center gap-2.5 text-xl">
                    Rewards
                    <span class="flex items-center gap-3 text-[15px] text-soft">
                      <span class="flex items-center gap-1.5"><span aria-hidden="true" class="size-3 rounded-sm bg-[#2f9a55]"></span>{detail.labels.base}</span>
                      <span class="flex items-center gap-1.5"><span aria-hidden="true" class="size-3 rounded-sm bg-[#8a6a1c]"></span>{detail.labels.bonus}</span>
                    </span>
                    <span class="ml-auto flex gap-2">
                      <button type="button" class="ip-btn ip-sm" disabled={busy || claimed >= reached} onclick={() => run('cruise/award', `${detail.labels.base}: phases 1 to ${reached} claimed`, { level: 0, claimed: true })}>Claim all</button>
                      <button type="button" class="ip-btn2 ip-sm" disabled={busy || claimed === 0} onclick={() => run('cruise/award', `${detail.labels.base} unclaimed`, { level: 0, claimed: false })}>Unclaim all</button>
                    </span>
                  </h3>
                  <div class="grid max-h-[460px] grid-cols-[repeat(auto-fill,minmax(148px,1fr))] gap-1.5 overflow-y-auto pr-1">
                    {#each pass.levels as level (level.level)}
                      {@const got = level.pt <= pass.shown}
                      <div class={['flex items-center gap-2 rounded-[3px] px-2 py-1.5', got ? 'bg-white/8' : 'bg-black/25', level.key && 'shadow-[inset_0_0_0_1px_var(--color-gold)]']}>
                        <span class="flex w-9 flex-col items-start">
                          <span class={['num text-lg leading-none', level.key ? 'text-gold' : got ? 'text-white' : 'text-dim']}>{level.level}</span>
                          <span class="num text-xs text-dim">{count(level.pt)}</span>
                        </span>
                        <button
                          type="button"
                          aria-pressed={level.free_claimed}
                          aria-label="{detail.labels.phase} {level.level} {detail.labels.base}: {level.free.name ?? ''} x{level.free.count}"
                          disabled={busy || (!level.free_claimed && !got)}
                          class={['cursor-pointer rounded-[3px] border-0 p-[3px] hover:shadow-[inset_0_0_0_2px_var(--color-cyan)] disabled:cursor-not-allowed', level.free_claimed ? 'bg-[#2f9a55]' : 'bg-transparent', !got && 'opacity-45']}
                          onclick={() => toggleLevel(level)}
                        >
                          {@render reward(level.free)}
                        </button>
                        <span class={['rounded-[3px] p-[3px]', pass.paid && got ? 'bg-[#8a6a1c]' : '', (!pass.paid || !got) && 'opacity-45']} title="{detail.labels.bonus}: {level.paid.name ?? ''}">
                          {@render reward(level.paid)}
                        </span>
                      </div>
                    {/each}
                  </div>
                </div>

                {@render taskGroups('Missions')}
              {:else if detail.kind === 'tasks' || detail.kind === 'tasklist'}
                {@render taskGroups(detail.kind === 'tasklist' ? 'Days' : 'Tasks')}
              {:else if (detail.kind === 'login7' || detail.kind === 'monthsign') && detail.signs}
                {@render signsView(detail.signs, detail.kind === 'monthsign')}
              {:else if detail.kind === 'score' && detail.score}
                {@const score = detail.score}
                <div class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-3">
                  {#if inline.editing === 'score'}
                    <form class="ip-row col-span-2 flex flex-col gap-2 px-4 py-3" onsubmit={(e) => saveScore(e, score.score, score.best)}>
                      <div class="grid grid-cols-2 gap-2">
                        <label class="flex flex-col gap-1"><span class="ip-sub">Score</span>
                          <!-- svelte-ignore a11y_autofocus -->
                          <input class="ip-field num h-10 text-xl" type="number" autofocus bind:value={inline.value} use:bounded={{ min: 0, max: uint32 }} onkeydown={inline.escape} />
                        </label>
                        <label class="flex flex-col gap-1"><span class="ip-sub">Best</span>
                          <input class="ip-field num h-10 text-xl" type="number" bind:value={inline.second} use:bounded={{ min: inline.value, max: uint32 }} onkeydown={inline.escape} />
                        </label>
                      </div>
                      <div class="flex justify-end gap-2">
                        <button type="button" class="ip-btn2 ip-sm" onclick={() => inline.stop()}>Cancel</button>
                        <button type="submit" class="ip-btn ip-sm" disabled={busy}>Save</button>
                      </div>
                    </form>
                  {:else}
                    <button type="button" aria-label="Edit the score" disabled={busy} class="ip-row flex cursor-pointer flex-col gap-1 border-0 px-4 py-2.5 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start('score', score.score, score.best)}>
                      <span class="ip-sub">Score</span>
                      <span class="num text-[30px]">{count(score.score)}</span>
                    </button>
                    <button type="button" aria-label="Edit the best score" disabled={busy} class="ip-row flex cursor-pointer flex-col gap-1 border-0 px-4 py-2.5 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => inline.start('score', score.score, score.best)}>
                      <span class="ip-sub">Best</span>
                      <span class="num text-[30px]">{count(score.best)}</span>
                    </button>
                  {/if}
                  <div class="ip-row flex flex-col gap-1 px-4 py-2.5">
                    <span class="ip-sub">Rank</span>
                    <span class="num text-[30px] text-gold">{score.rank}</span>
                  </div>
                </div>
                <div class="flex flex-col gap-1.5">
                  {#each score.brackets as bracket (bracket.id)}
                    <div class={['ip-row flex items-center gap-4 px-3 py-2', bracket.id === score.rank && 'ip-sel']}>
                      <span class="w-24 text-lg">Rank <span class="num">{bracket.id}</span></span>
                      <span class="num text-lg text-soft">{count(bracket.lower)}{bracket.upper > bracket.lower ? ` - ${count(bracket.upper)}` : '+'}</span>
                    </div>
                  {/each}
                </div>
              {:else if detail.kind === 'juus' && detail.juus}
                {@const juus = detail.juus}
                <div class="grid grid-cols-[repeat(auto-fit,minmax(380px,1fr))] gap-5">
                  <div class="flex min-w-0 flex-col gap-2.5">
                    <h3 class="m-0 flex items-baseline gap-2.5 text-xl">Posts <span class="num text-lg text-soft">{juus.posts.length}</span></h3>
                    <div class="flex max-h-[560px] flex-col gap-1.5 overflow-y-auto">
                      {#each juus.posts as post (post.id)}
                        <div class="ip-row flex items-center gap-3 px-2.5 py-1.5">
                          {#if post.icon}<Icon icon={post.icon} rarity={post.rarity} ship name={post.ship} size={40} label={false} />{/if}
                          <span class="flex min-w-0 flex-1 flex-col">
                            <span class="truncate text-[16px]" title={post.title || post.ship}>{post.title || post.ship || `Post ${post.id}`}</span>
                            <span class="num text-xs text-dim">{post.id}</span>
                          </span>
                          <span class="flex gap-1">
                            {#if post.read}<span class="ip-tag bg-white/10 text-soft uppercase">Read</span>{/if}
                            {#if post.liked}<span class="ip-tag bg-[#e0559b] text-white uppercase">Liked</span>{/if}
                            {#if post.commented}<span class="ip-tag bg-cyan text-[#0b2a3a] uppercase">Replied</span>{/if}
                          </span>
                        </div>
                      {:else}
                        <div class="ip-row px-3 py-2.5 text-dim">None</div>
                      {/each}
                    </div>
                  </div>
                  <div class="flex min-w-0 flex-col gap-2.5">
                    <h3 class="m-0 flex items-baseline gap-2.5 text-xl">Chats <span class="num text-lg text-soft">{juus.topics.length}</span></h3>
                    <div class="flex max-h-[560px] flex-col gap-1.5 overflow-y-auto">
                      {#each juus.topics as topic (topic.id)}
                        <div class="ip-row flex items-center gap-3 px-2.5 py-1.5">
                          <Icon icon={topic.icon} rarity={topic.rarity} ship name={topic.ship} size={40} label={false} />
                          <span class="flex min-w-0 flex-1 flex-col">
                            <span class="truncate text-[16px]" title={topic.name}>{topic.name || `Topic ${topic.id}`}</span>
                            <span class="truncate text-xs text-dim">{topic.ship}</span>
                          </span>
                          <span class="num text-sm text-soft">{stamp(topic.time).slice(0, 10)}</span>
                          {#if topic.read}<span class="ip-tag bg-white/10 text-soft uppercase">Read</span>{/if}
                        </div>
                      {:else}
                        <div class="ip-row px-3 py-2.5 text-dim">None</div>
                      {/each}
                    </div>
                  </div>
                </div>
              {:else}
                {#if selected.key === 'arcade'}
                  <a href="/commanders/{id}/shops" use:link class="self-start text-base text-link">Prize shop in Shops</a>
                {/if}
                {#each selected.tables as table (table.table)}
                  <div class="flex min-w-0 flex-col gap-2">
                    <h3 class="m-0 flex items-baseline gap-2.5 text-lg">{table.label}{#if table.total > 1}<span class="num text-base text-soft">{table.total}</span>{/if}</h3>
                    {#if table.rows.length === 1}
                      <div class="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2">
                        {#each table.columns as column, i (column)}
                          <div class="ip-row flex min-w-0 flex-col gap-0.5 px-3 py-2">
                            <span class="ip-sub truncate">{column}</span>
                            <span class="num truncate text-lg" title={value(table, i, table.rows[0][i])}>{value(table, i, table.rows[0][i])}</span>
                          </div>
                        {/each}
                      </div>
                    {:else}
                      <div class="ip-row max-h-[320px] overflow-auto">
                        <table class="w-full border-collapse text-sm">
                          <thead class="sticky top-0 bg-[#262b38]">
                            <tr>{#each table.columns as column (column)}<th scope="col" class="ip-sub px-2.5 py-1.5 text-left whitespace-nowrap">{column}</th>{/each}</tr>
                          </thead>
                          <tbody>
                            {#each table.rows as row, r (r)}
                              <tr class="border-t border-white/6">
                                {#each row as v, i (i)}<td class="num max-w-[220px] truncate px-2.5 py-1 text-base" title={value(table, i, v)}>{value(table, i, v)}</td>{/each}
                              </tr>
                            {/each}
                          </tbody>
                        </table>
                      </div>
                      {#if table.total > table.rows.length}<span class="text-[13px] text-dim">First {table.rows.length} of {table.total}</span>{/if}
                    {/if}
                  </div>
                {:else}
                  <div class="ip-row px-3 py-2.5 text-dim">No stored state</div>
                {/each}
              {/if}
            </div>
          </section>
        {/if}
      </div>
    {/if}
  {/snippet}
</CommanderPage>
