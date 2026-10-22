<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import Card from '../../components/Card.svelte'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import Panel from '../../components/Panel.svelte'
  import VirtualGrid from '../../components/VirtualGrid.svelte'
  import Seg from '../../components/Seg.svelte'
  import { count } from '../../lib/format'
  import { fuzzy } from '../../lib/fuzzy'
  import { link } from '../../lib/router.svelte'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import { left, missionTag, type Mission, type ProgressView, type ResearchView, type StageRow } from '../../lib/progress'
  import { bounded, clamp } from '../../lib/bounded'

  let { id }: { id: number } = $props()

  type Tab = 'campaign' | 'missions' | 'research' | 'collection' | 'tutorial'
  let tab = $state<Tab>('campaign')
  let hard = $state(false)
  let picked = $state(-1)
  // the chapters to clear: from..to, in which modes, with their stars
  let unlockFrom = $state(1)
  let unlockTo = $state(1)
  let unlockMode = $state<'normal' | 'hard' | 'both'>('normal')
  let unlockStars = $state(false)
  // the first end of a range being clicked on the chapter tiles
  let anchor = $state<number | null>(null)
  let guide = $state(1)
  let newGuide = $state(1)
  let edits = $state<Record<string, number>>({})
  let missionFilter = $state('All')
  let search = $state('')
  const [searched] = createDebouncedValue(() => search, { wait: 150 })

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'progress'),
    queryFn: get<ProgressView>(`/commanders/${id}/progress`),
  }))
  const research = createQuery(() => ({
    queryKey: keys.screen(id, 'research'),
    queryFn: get<ResearchView>(`/commanders/${id}/research`),
    enabled: tab === 'research',
  }))
  const data = $derived(query.data)

  $effect(() => {
    if (!data) return
    // the forms start from what's saved, again after each write
    guide = data.guide
    newGuide = data.new_guide
    edits = {}
  })
  $effect(() => {
    if (!data || picked >= 0) return
    // the first chapter not fully cleared
    const open = data.maps.findIndex((m) => m.normal.some((s) => s.progress < 100))
    picked = open < 0 ? data.maps.length - 1 : open
    unlockFrom = 1
    unlockTo = picked + 1
  })

  const { mutation: edit, run } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)
  const post = (path: string, body: unknown, done: string) => run(`/progress${path}`, done, body)

  // a tile click starts a range, the next one ends it
  function tile(n: number) {
    if (anchor === null) {
      unlockFrom = unlockTo = anchor = n
    } else {
      unlockFrom = Math.min(anchor, n)
      unlockTo = Math.max(anchor, n)
      anchor = null
    }
  }
  // the stages the clearing reaches, and how many are cleared already
  const unlockStages = $derived.by(() => {
    if (!data) return { total: 0, done: 0 }
    const stages = data.maps.slice(unlockFrom - 1, unlockTo).flatMap((m) => [...(unlockMode !== 'hard' ? m.normal : []), ...(unlockMode !== 'normal' ? m.hard : [])])
    return { total: stages.length, done: stages.filter((s) => s.progress >= 100 && (!unlockStars || s.stars.every((t) => t.done))).length }
  })
  function clearChapters() {
    const range = unlockFrom === unlockTo ? `Chapter ${unlockFrom}` : `Chapters ${unlockFrom}–${unlockTo}`
    const body = { from: unlockFrom, chapter: unlockTo, normal: unlockMode !== 'hard', hard: unlockMode !== 'normal', stars: unlockStars }
    post('/unlock', body, `${range} cleared${unlockStars ? ' with all stars' : ''}`)
  }

  const stagesOf = (i: number) => (data ? (hard ? data.maps[i].hard : data.maps[i].normal) : [])
  const cleared = (stages: StageRow[]) => stages.length > 0 && stages.every((s) => s.progress >= 100)
  const started = (stages: StageRow[]) => stages.some((s) => s.progress > 0 || s.defeat_count > 0)
  const stars = $derived(data ? data.maps.flatMap((m) => [...m.normal, ...m.hard]).flatMap((s) => s.stars) : [])
  const starsDone = $derived(stars.filter((s) => s.done).length)
  const chaptersCleared = $derived(data ? data.maps.filter((_, i) => cleared(stagesOf(i))).length : 0)
  const modeStars = $derived(data ? data.maps.flatMap((m) => (hard ? m.hard : m.normal)).flatMap((s) => s.stars) : [])
  const chapter = $derived(data?.maps[picked])
  const missionsDone = $derived(data ? data.missions.filter((m) => m.progress >= m.target).length : 0)
  const newVersion = $derived(guide <= newGuide)
  const missionTags = $derived(['All', ...new Set((data?.missions ?? []).map(missionTag))])
  const missions = $derived(
    fuzzy(
      (data?.missions ?? []).filter((m) => missionFilter === 'All' || missionTag(m) === missionFilter),
      searched(),
      (m) => [m.desc, m.name, m.id],
    ),
  )

  function setStage(stage: StageRow, body: { cleared?: boolean; stars?: boolean[]; wins?: number }, done: string) {
    post('/stage', { stage: stage.id, ...body }, done)
  }

  // the stage whose win count is being edited, and the count typed
  let editingWins = $state<number | null>(null)
  let wins = $state(0)
  function saveWins(stage: StageRow) {
    editingWins = null
    if (wins !== stage.defeat_count) setStage(stage, { wins }, `${stage.name}: won ${wins} times`)
  }

  const key = (m: Mission) => `${m.kind}:${m.act ?? 0}:${m.id}`

  function setMission(mission: Mission, progress: number) {
    post('/mission', { kind: mission.kind, act: mission.act ?? 0, id: mission.id, progress }, `${mission.desc || mission.name}: ${progress}/${mission.target}`)
  }

  const tabs: { value: Tab; label: string }[] = $derived([
    { value: 'campaign', label: `Campaign ${starsDone}★` },
    { value: 'missions', label: `Missions ${missionsDone}/${data?.missions.length ?? 0}` },
    { value: 'research', label: 'Research' },
    { value: 'collection', label: 'Collection' },
    { value: 'tutorial', label: 'Tutorial' },
  ])
</script>

<CommanderPage {id} title="Progress">
  {#snippet children(summary)}
    {#if data}
      <div class="flex flex-col gap-[18px]">
        <div class="ip-tabs rounded" role="group" aria-label="Sections">
          {#each tabs as t (t.value)}
            <button type="button" aria-pressed={tab === t.value} onclick={() => (tab = t.value)}>{t.label}</button>
          {/each}
        </div>

        {#if tab === 'campaign' && chapter}
          <div class="grid grid-cols-1 items-start gap-[18px] xl:grid-cols-[minmax(0,1fr)_380px]">
            <Panel title="Campaign" sub="Main story">
              {#snippet actions()}
                <Seg label="Mode" options={[{ value: false, label: 'Normal' }, { value: true, label: 'Hard' }]} bind:value={hard} />
              {/snippet}
              <div class="flex flex-col gap-4 p-5">
                <div class="flex flex-wrap items-baseline gap-x-7 gap-y-2">
                  <span class="flex items-baseline gap-2 text-sm text-soft">Chapters cleared <span class="num text-[26px] text-white">{chaptersCleared}</span><span class="num text-dim">/{data.maps.length}</span></span>
                  <span class="flex items-baseline gap-2 text-sm text-soft">Stars <span class="num text-[26px] text-gold">{modeStars.filter((s) => s.done).length}</span><span class="num text-dim">/{modeStars.length}</span></span>
                  <span class="ml-auto flex items-center gap-4 text-[13px] text-soft">
                    <span class="flex items-center gap-1.5"><span class="h-[3px] w-3 bg-ok"></span>Cleared</span>
                    <span class="flex items-center gap-1.5"><span class="h-[3px] w-3 bg-cyan"></span>In progress</span>
                    <span class="flex items-center gap-1.5"><span class="h-[3px] w-3 bg-dim"></span>Not started</span>
                  </span>
                </div>
                <div class="flex flex-wrap gap-1.5" role="group" aria-label="Chapters">
                  {#each data.maps as map, i (map.id)}
                    {@const stages = stagesOf(i)}
                    <button
                      type="button"
                      aria-pressed={picked === i}
                      aria-label="Chapter {map.title}"
                      class={[
                        'num h-12 w-[42px] cursor-pointer rounded-[3px] border-0 border-b-4 text-[26px] text-white',
                        picked === i ? 'bg-navy shadow-[inset_0_0_0_2px_var(--color-cyan)]' : 'bg-white/8 hover:bg-white/14',
                        cleared(stages) ? 'border-ok' : started(stages) ? 'border-cyan' : 'border-dim/60 text-white/60',
                      ]}
                      onclick={() => (picked = i)}>{map.title}</button
                    >
                  {/each}
                </div>
                <div class="flex flex-wrap items-baseline gap-3">
                  <span class="ip-sub text-cyan">Chapter {chapter.title}</span>
                  <h3 class="m-0 text-[26px]">{chapter.name}</h3>
                  <span class="ml-auto text-sm text-soft">{hard ? 'Hard' : 'Normal'} · Lv.{chapter.level_limit}</span>
                </div>
                <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
                  {#each stagesOf(picked) as stage (stage.id)}
                    {@const inSortie = data.sortie?.stage === stage.id}
                    {@const allStars = stage.stars.every((s) => s.done)}
                    <div class={['ip-row flex flex-col gap-2.5 p-4', inSortie && 'ip-sel']}>
                      <div class="flex items-baseline gap-3">
                        <span class="num text-[30px] leading-none">{stage.name}</span>
                        {#if inSortie}
                          <span class="text-warn">In sortie</span>
                        {:else if stage.progress >= 100}
                          <span class="text-ok">Cleared</span>
                        {:else}
                          <span class="text-dim">{stage.progress ? `${stage.progress}%` : 'Not cleared'}</span>
                        {/if}
                        {#if editingWins === stage.id}
                          <form class="ml-auto flex items-center gap-1.5 text-sm text-soft" onsubmit={(e) => (e.preventDefault(), saveWins(stage))}>
                            Won
                            <!-- svelte-ignore a11y_autofocus -->
                            <input class="ip-field num h-8 w-20 text-right text-lg" type="number" aria-label="Times {stage.name} was won" autofocus bind:value={wins} use:bounded={{ min: 0 }} onkeydown={(e) => e.key === 'Escape' && (editingWins = null)} />
                            times
                            <button type="submit" class="ip-btn ip-sm h-8" disabled={busy}>Save</button>
                          </form>
                        {:else}
                          <button
                            type="button"
                            aria-label="Edit how many times {stage.name} was won"
                            class="ml-auto cursor-pointer rounded border-0 bg-transparent px-1.5 py-0.5 text-sm text-soft hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]"
                            onclick={() => ((wins = stage.defeat_count), (editingWins = stage.id))}>Won <span class="num text-lg text-white">{stage.defeat_count}</span> times</button
                          >
                        {/if}
                      </div>
                      <span class="truncate text-sm text-soft">{stage.title}</span>
                      <ul class="m-0 flex list-none flex-col gap-1 p-0">
                        {#each stage.stars as star, n (n)}
                          <li class={['flex items-center gap-2 text-[15px]', !star.done && 'text-white/45']}>
                            <span aria-hidden="true" class={star.done ? 'text-gold' : 'text-white/30'}>★</span>
                            <span class="sr-only">{star.done ? 'Done:' : 'Not done:'}</span>
                            {star.label}
                          </li>
                        {/each}
                      </ul>
                      <div class="mt-auto flex flex-wrap justify-end gap-2">
                        {#if stage.progress < 100}
                          <button type="button" class="ip-btn ip-sm" disabled={busy} onclick={() => setStage(stage, { cleared: true }, `${stage.name} cleared`)}>Mark {stage.name} cleared</button>
                        {/if}
                        {#if !allStars}
                          <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => setStage(stage, { cleared: true, stars: stage.stars.map(() => true) }, `${stage.name} cleared with all stars`)}>Give all {stage.stars.length} stars</button>
                        {/if}
                        {#if stage.stars.some((s) => s.done)}
                          <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => setStage(stage, { stars: stage.stars.map(() => false) }, `${stage.name}: stars reset`)}>Reset stars</button>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              </div>
            </Panel>

            <div class="flex min-w-0 flex-col gap-[18px]">
              {#if data.sortie}
                {@const sortie = data.sortie}
                <section class="ip-panel ip-danger-zone flex flex-col gap-3 p-5" aria-label="Sortie">
                  <span class="ip-sub text-cyan">Sortie</span>
                  <span class="text-[22px]">In <span class="num">{sortie.name}</span> with {sortie.fleets.length > 1 ? 'fleets' : 'fleet'} {sortie.fleets.join(' and ')}</span>
                  <span class="text-sm text-soft">
                    Round <span class="num text-lg text-white">{sortie.round}</span> · ammo
                    <span class="num text-lg text-white">{sortie.ammo.join(', ')}/{sortie.ammo_total}</span> · kills
                    <span class="num text-lg text-white">{sortie.kills}</span>
                  </span>
                  <button type="button" class="ip-danger self-end" disabled={busy} onclick={() => post('/end-sortie', undefined, 'Sortie ended')}>End sortie</button>
                </section>
              {/if}
              <Card title="Clear chapters for {summary.name}">
                <div class="ip-strip">Chapters</div>
                <div role="group" aria-label="Chapters to clear" class="grid grid-cols-8 gap-1">
                  {#each data.maps as map, i (map.id)}
                    {@const n = i + 1}
                    {@const inRange = n >= unlockFrom && n <= unlockTo}
                    <button
                      type="button"
                      aria-pressed={inRange}
                      aria-label="Chapter {map.title}"
                      class={[
                        'num h-9 cursor-pointer rounded-[3px] border-0 text-xl',
                        inRange ? 'bg-cyan text-white' : 'bg-[#e6e9ef] text-ink hover:bg-[#d5dae3]',
                        anchor === n && 'shadow-[inset_0_0_0_2px_var(--color-navy)]',
                      ]}
                      onclick={() => tile(n)}>{map.title}</button
                    >
                  {/each}
                </div>
                <div class="flex items-end gap-2.5">
                  <label class="ip-lab min-w-0 flex-1">From <input class="ip-field num w-full text-2xl" type="number" bind:value={unlockFrom} use:bounded={{ min: 1, max: unlockTo }} /></label>
                  <label class="ip-lab min-w-0 flex-1">To <input class="ip-field num w-full text-2xl" type="number" bind:value={unlockTo} use:bounded={{ min: unlockFrom, max: data.maps.length }} /></label>
                </div>
                <div class="grid grid-cols-2 gap-2">
                  <button type="button" class="ip-opt" aria-pressed={unlockFrom === 1 && unlockTo === data.maps.length} onclick={() => ((unlockFrom = 1), (unlockTo = data!.maps.length), (anchor = null))}>All chapters</button>
                  <button type="button" class="ip-opt" aria-pressed={unlockFrom === picked + 1 && unlockTo === picked + 1} onclick={() => ((unlockFrom = unlockTo = picked + 1), (anchor = null))}>Chapter {chapter.title} only</button>
                </div>
                <div class="ip-strip">Mode</div>
                <div role="group" aria-label="Mode" class="grid grid-cols-3 gap-2">
                  {#each [{ value: 'normal', label: 'Normal' }, { value: 'hard', label: 'Hard' }, { value: 'both', label: 'Both' }] as option (option.value)}
                    <button type="button" class="ip-opt" aria-pressed={unlockMode === option.value} onclick={() => (unlockMode = option.value as typeof unlockMode)}>{option.label}</button>
                  {/each}
                </div>
                <label class="flex items-center gap-2.5 text-[17px]">
                  <input type="checkbox" class="size-5 accent-cyan" bind:checked={unlockStars} />
                  All stars
                </label>
                <span class="text-[15px] text-ink-soft"
                  ><span class="num text-xl text-ink">{unlockStages.total}</span> stages ·
                  <span class="num text-xl text-ink">{unlockStages.done}</span> already done</span
                >
                <button type="button" class="ip-btn" disabled={busy || !unlockStages.total || unlockStages.done === unlockStages.total} onclick={clearChapters}>
                  {unlockFrom === unlockTo ? `Clear chapter ${unlockFrom}` : `Clear chapters ${unlockFrom}–${unlockTo}`}
                </button>
              </Card>
            </div>
          </div>
        {:else if tab === 'missions'}
          <Panel title="Missions" sub="Tasks">
            {#snippet actions()}
              <label class="relative">
                <span class="sr-only">Search missions</span>
                <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute top-1/2 left-3 size-5 -translate-y-1/2" fill="none" stroke="#4a5163" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="6.5" /><path d="M16 16l4.5 4.5" /></svg>
                <input class="ip-field h-[42px] w-[220px] pl-10" placeholder="Search" bind:value={search} />
              </label>
              <Seg label="Kind" options={missionTags.map((t) => ({ value: t, label: t }))} bind:value={missionFilter} />
            {/snippet}
            <VirtualGrid items={missions} rowHeight={54} key={key} label="Missions" class="max-h-[calc(100vh-260px)] min-h-60 p-2.5">
              {#snippet cell(mission)}
                {@const value = edits[key(mission)] ?? mission.progress}
                <div class="ip-row grid grid-cols-[72px_minmax(0,1fr)_164px_auto_auto] items-center gap-3 px-3 py-2">
                  <span class="ip-tag bg-strip text-center text-soft">{missionTag(mission)}</span>
                  <span class="flex min-w-0 flex-col gap-1">
                    <span class="truncate text-[15px]">{mission.desc || mission.name}</span>
                    <span aria-hidden="true" class="h-1.5 rounded-[3px] bg-black/45"><span class="block h-full rounded-[3px] bg-cyan" style:width="{Math.min(100, (mission.progress / Math.max(1, mission.target)) * 100)}%"></span></span>
                  </span>
                  <span class="flex items-center gap-1.5">
                    <input
                      class="ip-field num h-9 w-[70px] text-right text-lg"
                      inputmode="numeric"
                      aria-label="Progress of {mission.desc || mission.name}"
                      {value}
                      use:bounded={{ min: 0, max: mission.target }}
                      onchange={(e) => (edits[key(mission)] = clamp(Number(e.currentTarget.value), { min: 0, max: mission.target }))}
                    />
                    <span class="num text-dim">/{mission.target}</span>
                  </span>
                  <button type="button" class="ip-btn2 ip-sm" disabled={busy || value === mission.progress} onclick={() => setMission(mission, value)}>Set</button>
                  <button type="button" class="ip-btn ip-sm" disabled={busy || mission.progress >= mission.target} onclick={() => setMission(mission, mission.target)}>Complete</button>
                </div>
              {/snippet}
              {#snippet empty()}
                <p class="m-0 px-3 py-2 text-dim">{search ? 'No match' : 'No missions running'}</p>
              {/snippet}
            </VirtualGrid>
          </Panel>
        {:else if tab === 'research'}
          {#if research.data}
            {@const view = research.data}
            {@const running = view.offers.filter((o) => o.finish > 0)}
            <div class="grid grid-cols-1 items-start gap-[18px] xl:grid-cols-2">
              <Panel title="Research" sub="Projects">
                {#snippet actions()}
                  <a href="/commanders/{id}/research" use:link class="text-base">Open research</a>
                {/snippet}
                <ul class="m-0 flex list-none flex-col gap-1.5 p-2.5">
                  {#each [...running, ...view.queue] as project (project.tech + ':' + project.id + ':' + project.pos)}
                    <li class="ip-row flex items-center gap-3 px-3 py-2">
                      <span class="num text-xl">{project.name}</span>
                      <span class="text-[13px] text-dim">{project.sub}</span>
                      <span class={['num ml-auto text-xl', project.finish <= view.now && 'text-ok']}>{project.finish > view.now ? left(project.finish - view.now) : 'Ready'}</span>
                    </li>
                  {:else}
                    <li class="px-3 py-2 text-dim">No project running</li>
                  {/each}
                </ul>
              </Panel>
              <Panel title="Fleet tech" sub="Tech points">
                <ul class="m-0 flex list-none flex-col gap-1.5 p-2.5">
                  {#each view.fleet_tech as group (group.id)}
                    <li class="ip-row grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-4 px-3 py-2">
                      <span>{group.name}</span>
                      <span class="text-sm text-soft">Lv.<span class="num text-lg text-white">{group.level}</span>/{group.max_level}</span>
                      <span class="num w-20 text-right text-lg text-link">{count(group.points)}</span>
                    </li>
                  {/each}
                </ul>
              </Panel>
              <Panel title="Research ships" sub="PR / DR">
                <ul class="m-0 flex list-none flex-col gap-1.5 p-2.5">
                  {#each view.blueprints.filter((b) => b.state !== 'locked') as ship (ship.id)}
                    <li class="ip-row flex items-center gap-3 px-3 py-2">
                      <Icon icon={ship.icon} rarity={ship.rarity} ship name={ship.name} size={44} />
                      <span>{ship.name}</span>
                      <span class="ip-tag bg-strip text-soft">PR{ship.version}</span>
                      <span class="ml-auto text-sm text-soft">{ship.dev ? `Dev Lv.${ship.dev.level}/${ship.dev.max_level}` : ship.state === 'running' ? 'Researching' : 'Stopped'}</span>
                    </li>
                  {:else}
                    <li class="px-3 py-2 text-dim">None started</li>
                  {/each}
                </ul>
              </Panel>
            </div>
          {:else if research.error}
            <p class="text-bad">{research.error.message}</p>
          {/if}
        {:else if tab === 'collection'}
          <Panel title="Collection">
            <div class="grid grid-cols-1 gap-2.5 p-2.5 sm:grid-cols-3">
              {#each [{ value: data.collection.ship_groups, label: 'Ships collected' }, { value: data.collection.trophies, label: 'Trophies claimed' }, { value: data.collection.stories, label: 'Stories read' }] as tile (tile.label)}
                <div class="ip-row flex flex-col gap-1 px-4 py-3">
                  <span class="num text-[30px]">{count(tile.value)}</span>
                  <span class="text-sm text-soft">{tile.label}</span>
                </div>
              {/each}
            </div>
          </Panel>
        {:else if tab === 'tutorial'}
          <Card title="Tutorial for {summary.name}" class="max-w-[520px]">
            <div class="ip-strip">Guide</div>
            <div class="grid grid-cols-2 gap-3">
              <label class="ip-lab">
                SG001 step
                <input class="ip-field num text-2xl" type="number" min="1" bind:value={guide} use:bounded={{ min: 1 }} />
              </label>
              <label class="ip-lab">
                SG002 step
                <input class="ip-field num text-2xl" type="number" min="1" bind:value={newGuide} use:bounded={{ min: 1 }} />
              </label>
            </div>
            <span class="text-[15px] text-ink-soft">Playing {newVersion ? 'SG002' : 'SG001'}</span>
            <button type="button" class="ip-btn" disabled={busy || guide < 1 || newGuide < 1 || (guide === data.guide && newGuide === data.new_guide)} onclick={() => post('/guide', { guide, new_guide: newGuide }, 'Tutorial saved')}>Save</button>
          </Card>
        {/if}
      </div>
    {/if}
  {/snippet}
</CommanderPage>
