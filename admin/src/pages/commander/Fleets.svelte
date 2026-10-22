<script lang="ts">
  // The commander's fleets as the game's formation screen shows them: one
  // fleet on a board, its flagship's painting behind it, its main and
  // vanguard rows (or its submarines) as dock cards, its meowfficers, and
  // its levels, hulls and oil over it (hidden with Overlays). Below, the
  // regular fleets, the hard mode chapters' and the events' as tabs. A slot
  // opens the ship picker, cards are dragged to swap them or onto a tab to
  // move them; every change saves at once.
  import { createQuery } from '@tanstack/svelte-query'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import DockShipCard from '../../components/DockShipCard.svelte'
  import Icon from '../../components/Icon.svelte'
  import Modal from '../../components/Modal.svelte'
  import Picker, { type PickerItem } from '../../components/Picker.svelte'
  import Seg from '../../components/Seg.svelte'
  import { toast } from '../../components/Toast.svelte'
  import { fleetName, type Dock, type DockShip, type FleetShipRef, type Fleets } from '../../lib/dock'
  import { tier } from '../../lib/format'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import { link, param, route } from '../../lib/router.svelte'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'fleets'),
    queryFn: get<Fleets>(`/commanders/${id}/fleets`),
  }))
  const dock = createQuery(() => ({
    queryKey: keys.screen(id, 'dock'),
    queryFn: get<Dock>(`/commanders/${id}/dock`),
  }))
  const data = $derived(query.data)
  const docked = $derived(new Map((dock.data?.ships ?? []).map((s) => [s.id, s])))

  type Row = 'main' | 'vanguard' | 'submarine' | 'support'
  type Kind = 'surface' | 'submarine' | 'support'

  // a fleet on the board: a regular one, a chapter formation's team or an
  // event's, saved by its own route
  interface Board {
    key: string
    tab: string
    title: string
    sub: string
    kind: Kind
    ships: FleetShipRef[]
    cats: number[]
    oil?: number
    locked: boolean
    unlockChapter?: string
    unlockMap?: number
    duplicates: number[]
    problem?: string
    limits: string[]
    // the route its ships and cats are posted to, the order the ships go in
    path: string
    mainFirst: boolean
    regular?: number
    // no meowfficer slots, no clearing (the exercise defense fleet)
    defense?: boolean
  }
  // the boards shown together, as tabs
  interface BoardSet {
    key: string
    label: string
    boards: Board[]
    clear?: { path: string; what: string; label: string }
  }

  type Group = 'regular' | 'hard' | 'events'
  let group = $state<Group>('regular')
  let setKey = $state('')
  let boardKey = $state('')

  const sets = $derived.by((): Record<Group, BoardSet[]> => {
    if (!data) return { regular: [], hard: [], events: [] }
    const regular: BoardSet = {
      key: 'regular',
      label: 'Regular',
      boards: data.fleets.map((f) => ({
        key: `fleet:${f.id}`,
        tab: f.id === 101 ? 'Exercise' : f.id >= 11 ? `Sub ${f.id - 10}` : `Fleet ${f.id}`,
        title: fleetName(f.id, f.name),
        sub: '',
        kind: f.id >= 11 && f.id < 100 ? 'submarine' : 'surface',
        ships: f.ships,
        cats: f.meowfficers,
        oil: f.oil,
        locked: f.locked,
        unlockChapter: f.unlock_chapter,
        unlockMap: f.unlock_map,
        duplicates: f.duplicates ?? [],
        limits: [],
        path: `/fleets/${f.id}`,
        mainFirst: false,
        regular: f.id,
      })),
    }
    if (data.defense) {
      regular.boards.push({
        key: 'defense',
        tab: 'Defense',
        title: 'Defense fleet',
        sub: data.defense.set ? '' : "Fleet 1's ships until set",
        kind: 'surface',
        ships: data.defense.ships,
        cats: [],
        locked: false,
        duplicates: [],
        limits: [],
        path: '/exercise/defense',
        mainFirst: false,
        defense: true,
      })
    }
    const kinds: { kind: 'main' | 'submarine' | 'support'; board: Kind; tab: string }[] = [
      { kind: 'main', board: 'surface', tab: 'Fleet' },
      { kind: 'submarine', board: 'submarine', tab: 'Sub' },
      { kind: 'support', board: 'support', tab: 'Support' },
    ]
    const hard = data.elite.map((saved): BoardSet => {
      const chapters = saved.chapters?.length ? (saved.chapters.length > 1 ? `${saved.chapters[0]} to ${saved.chapters[saved.chapters.length - 1]}` : saved.chapters[0]) : `Formation ${saved.formation}`
      const boards = kinds.flatMap(({ kind, board, tab }) => {
        const teams = saved.teams.filter((t) => t.kind === kind)
        const count = Math.max(teams.length, saved.max?.[kind] ?? 0)
        return Array.from({ length: count }, (_, i): Board => {
          const team = teams[i]
          return {
            key: `elite:${saved.formation}:${kind}:${i + 1}`,
            tab: `${tab} ${i + 1}`,
            title: `${tab} ${i + 1}`,
            sub: chapters,
            kind: board,
            ships: team?.ships ?? [],
            cats: team?.meowfficers ?? [0, 0],
            oil: team?.oil,
            locked: false,
            duplicates: team?.duplicates ?? [],
            problem: team?.problem,
            limits: team?.limits ?? [],
            path: `/elite-fleets/${saved.formation}/${kind}/${i + 1}`,
            mainFirst: true,
          }
        })
      })
      return { key: `elite:${saved.formation}`, label: chapters, boards, clear: { path: `/elite-fleets/${saved.formation}`, what: `${chapters} fleets cleared`, label: 'Clear formation' } }
    })
    const events = data.activity.map(
      (activity): BoardSet => ({
        key: `activity:${activity.id}`,
        label: `Event ${activity.id}`,
        boards: activity.fleets.map((f) => ({
          key: `activity:${activity.id}:${f.id}`,
          tab: fleetName(f.id),
          title: fleetName(f.id, f.name),
          sub: `Event ${activity.id}`,
          kind: f.id >= 11 && f.id < 100 ? 'submarine' : 'surface',
          ships: f.ships,
          cats: f.meowfficers,
          oil: f.oil,
          locked: false,
          duplicates: f.duplicates ?? [],
          limits: [],
          path: `/activity-fleets/${activity.id}/${f.id}`,
          mainFirst: false,
        })),
        clear: { path: `/activity-fleets/${activity.id}`, what: `Event ${activity.id} fleets cleared`, label: 'Clear event fleets' },
      }),
    )
    return { regular: [regular], hard, events }
  })

  // ?fleet= (from the dock) picks a regular fleet
  $effect(() => {
    void route.search
    const linked = param('fleet')
    if (linked) {
      group = 'regular'
      boardKey = `fleet:${linked}`
    }
    // ?board=defense (from Combat) picks the exercise defense fleet
    if (new URLSearchParams(route.search).get('board') === 'defense') {
      group = 'regular'
      boardKey = 'defense'
    }
  })
  const set = $derived(sets[group].find((s) => s.key === setKey) ?? sets[group][0])
  const board = $derived(set?.boards.find((b) => b.key === boardKey) ?? set?.boards[0])

  const rows = (kind: Kind): Row[] => (kind === 'surface' ? ['main', 'vanguard'] : kind === 'submarine' ? ['submarine'] : ['support'])
  const rowOf = (b: Board, ship: FleetShipRef): Row => (b.kind === 'support' ? 'support' : b.kind === 'submarine' ? 'submarine' : ship.team === 'vanguard' ? 'vanguard' : 'main')
  const inRow = (b: Board, row: Row) => b.ships.filter((s) => rowOf(b, s) === row)
  const rowNames: Record<Row, string> = { main: 'Main', vanguard: 'Vanguard', submarine: 'Submarine', support: 'Support' }

  // the ship ids a board saves, its rows in the order its route takes
  function order(b: Board, byRow: Partial<Record<Row, number[]>>): number[] {
    const list = (row: Row) => byRow[row] ?? inRow(b, row).map((s) => s.id)
    if (b.kind !== 'surface') return list(rows(b.kind)[0])
    return b.mainFirst ? [...list('main'), ...list('vanguard')] : [...list('vanguard'), ...list('main')]
  }

  const { mutation: write } = createCommanderWrite(() => id)
  const busy = $derived(write.isPending)
  const saveBoard = (b: Board, ships: number[], cats: number[], what: string) => write.mutateAsync({ path: b.path, body: { ships, meowfficers: cats }, done: what })

  // picking a ship for a slot
  let picker = $state<Picker>()
  let picking = $state<{ row: Row; index: number } | null>(null)
  const shipName = (shipID: number) => docked.get(shipID)?.name ?? `#${shipID}`
  const candidates = $derived.by((): PickerItem[] => {
    if (!board || !picking) return []
    const row = picking.row
    const fits = (s: DockShip) => (row === 'support' ? s.team !== 'submarine' : s.team === row)
    const here = new Set(board.ships.map((s) => s.id))
    return (dock.data?.ships ?? [])
      .filter((s) => fits(s) && !here.has(s.id))
      .sort((a, b) => b.level - a.level || b.rarity - a.rarity)
      .map((s) => ({ id: s.id, name: s.name, icon: s.square_icon, rarity: s.rarity, ship: true, sub: s.fleet ? `Lv.${s.level} · ${fleetName(s.fleet)}` : `Lv.${s.level}`, search: [s.type] }))
  })
  function pickSlot(row: Row, index: number) {
    picking = { row, index }
    queueMicrotask(() => picker?.show())
  }
  function picked(item: PickerItem) {
    if (!board || !picking) return
    const ids = inRow(board, picking.row).map((s) => s.id)
    if (picking.index < ids.length) ids[picking.index] = item.id
    else ids.push(item.id)
    saveBoard(board, order(board, { [picking.row]: ids }), board.cats, `${shipName(item.id)} joins ${board.title}`)
    picking = null
  }
  function removeShip(b: Board, ship: FleetShipRef) {
    const row = rowOf(b, ship)
    saveBoard(b, order(b, { [row]: inRow(b, row).filter((s) => s.id !== ship.id).map((s) => s.id) }), b.cats, `${ship.name} leaves ${b.title}`)
  }

  // dragging a card: onto a slot of its row swaps, onto a tab moves it
  let dragged = $state<{ board: string; ship: FleetShipRef } | null>(null)
  function dropOnSlot(row: Row, index: number) {
    const from = dragged
    dragged = null
    if (!board || !from || from.board !== board.key) return
    if (rowOf(board, from.ship) !== row) return toast(`${from.ship.name} can't go in the ${rowNames[row].toLowerCase()} row`, true)
    const ids = inRow(board, row).map((s) => s.id)
    const at = ids.indexOf(from.ship.id)
    const to = Math.min(index, ids.length - 1)
    if (at === to) return
    ;[ids[at], ids[to]] = [ids[to], ids[at]]
    saveBoard(board, order(board, { [row]: ids }), board.cats, `${board.title} reordered`)
  }
  async function dropOnTab(target: Board) {
    const from = dragged
    dragged = null
    const source = set?.boards.find((b) => b.key === from?.board)
    if (!from || !source || target.key === source.key) return
    if (target.locked) return toast(`${target.title} is locked`, true)
    const row: Row = target.kind === 'support' ? 'support' : target.kind === 'submarine' ? 'submarine' : from.ship.team === 'vanguard' ? 'vanguard' : 'main'
    if (target.kind !== source.kind && !(target.kind === 'support' && from.ship.team !== 'submarine')) return toast(`${from.ship.name} can't join ${target.title}`, true)
    const into = inRow(target, row).map((s) => s.id)
    if (into.length >= 3) return toast(`${target.title}'s ${rowNames[row].toLowerCase()} row is full`, true)
    // the defense fleet shares its ships with the others: a copy, not a move
    if (target.defense) {
      try {
        await saveBoard(target, order(target, { [row]: [...into, from.ship.id] }), target.cats, `${from.ship.name} defends too`)
        boardKey = target.key
      } catch {
        // the toast has the server's answer
      }
      return
    }
    const sourceRow = rowOf(source, from.ship)
    const before = order(source, {})
    try {
      await saveBoard(source, order(source, { [sourceRow]: inRow(source, sourceRow).filter((s) => s.id !== from.ship.id).map((s) => s.id) }), source.cats, `${from.ship.name} leaves ${source.title}`)
    } catch {
      return // the toast has the server's answer
    }
    try {
      await saveBoard(target, order(target, { [row]: [...into, from.ship.id] }), target.cats, `${from.ship.name} moved to ${target.title}`)
      boardKey = target.key
    } catch {
      // the target refused it: the ship goes back where it was
      await saveBoard(source, before, source.cats, `${from.ship.name} back in ${source.title}`).catch(() => {})
    }
  }

  // meowfficers
  let catPicker = $state<Picker>()
  let catSlot = $state(0)
  const catItems = $derived<PickerItem[]>([
    { id: 0, name: 'None', rarity: 1 },
    ...(data?.meowfficers ?? []).filter((cat) => cat.id !== board?.cats[1 - catSlot]).map((cat) => ({ id: cat.id, name: cat.name, icon: cat.icon, rarity: tier(cat.rarity ?? 2, true) })),
  ])
  function pickCat(item: PickerItem) {
    if (!board) return
    const cats = [...board.cats]
    while (cats.length < 2) cats.push(0)
    cats[catSlot] = item.id
    saveBoard(board, order(board, {}), cats, item.id ? `${item.name} joins ${board.title}` : `${board.title}'s meowfficer removed`)
  }

  // renaming a regular fleet
  let renameModal = $state<Modal>()
  let name = $state('')
  function rename(event: SubmitEvent) {
    event.preventDefault()
    if (!board) return
    write.mutate({ path: board.path, body: { name }, done: name ? `Renamed to ${name}` : 'Name reset' }, { onSuccess: () => renameModal?.close() })
  }

  function clear() {
    if (!board || !set) return
    if (board.regular) {
      if (confirm(`Clear ${board.title}?`)) write.mutate({ method: 'DELETE', path: board.path, done: `${board.title} cleared` })
    } else if (set.clear && confirm(`${set.clear.label}?`)) {
      write.mutate({ method: 'DELETE', path: set.clear.path, done: set.clear.what })
    }
  }

  function unlock(b: Board) {
    if (!b.unlockMap || !confirm(`Clear chapters 1 to ${b.unlockMap}? ${b.title} unlocks with ${b.unlockChapter}.`)) return
    write.mutate({ path: '/progress/unlock', body: { chapter: b.unlockMap, hard: false }, done: `${b.title} unlocked` })
  }

  // the board's overlays: its levels, hulls and oil
  let overlays = $state(true)
  const levels = $derived(board?.ships.map((s) => s.level) ?? [])
  const hullShort: Record<string, string> = {
    Destroyer: 'DD',
    'Light Cruiser': 'CL',
    'Heavy Cruiser': 'CA',
    'Large Cruiser': 'CB',
    Battlecruiser: 'BC',
    Battleship: 'BB',
    'Light Aircraft Carrier': 'CVL',
    'Aircraft Carrier': 'CV',
    Submarine: 'SS',
    'Aviation Battleship': 'BBV',
    'Repair Ship': 'AR',
    Monitor: 'BM',
    'Submarine Carrier': 'SSV',
    'Munition Ship': 'AE',
  }
  const hulls = $derived.by(() => {
    const counts = new Map<string, number>()
    for (const s of board?.ships ?? []) {
      const type = (s.type ?? docked.get(s.id)?.type ?? '').trim()
      const short = hullShort[type] ?? type
      if (short) counts.set(short, (counts.get(short) ?? 0) + 1)
    }
    return [...counts].map(([hull, n]) => `${n} ${hull}`)
  })

  // the flagship's painting behind the board
  const flagship = $derived(board ? (inRow(board, rows(board.kind)[0])[0] ?? board.ships[0]) : undefined)
  const painting = $derived((flagship && (docked.get(flagship.id)?.square_icon ?? flagship.square_icon))?.split('/').pop() ?? '')
  let paintingFailed = $state(false)
  $effect(() => {
    void painting
    paintingFailed = false
  })

  const problems: Record<string, string> = {
    teamCount: 'Needs 1 to 3 main and 1 to 3 vanguard ships',
    typeLimitation: "The hulls don't meet the chapter's limits",
  }
  const glass = 'rounded bg-[rgb(20_24_34/0.82)] shadow-[0_8px_24px_rgb(0_0_0/0.35)]'
  const shade = '[text-shadow:0_1px_2px_rgb(0_0_0/0.9),0_2px_12px_rgb(0_0_0/0.75)]'
  const roundButton = 'grid cursor-pointer place-items-center rounded-full border-0 text-white'
</script>

{#snippet slot(b: Board, row: Row, index: number)}
  {@const ship = inRow(b, row)[index]}
  {@const docked_ = ship && docked.get(ship.id)}
  <div class="relative w-[150px]" role="listitem" ondragover={(e) => dragged && e.preventDefault()} ondrop={() => dropOnSlot(row, index)}>
    {#if ship}
      <button
        type="button"
        draggable="true"
        aria-label="{rowNames[row]} {index + 1}: {ship.name}, level {ship.level}"
        class={['block w-full cursor-grab rounded-[3px] border-0 bg-transparent p-0 transition-transform hover:-translate-y-0.5', b.duplicates.includes(ship.id) && 'shadow-[0_0_0_3px_var(--color-red)]']}
        ondragstart={() => (dragged = { board: b.key, ship })}
        ondragend={() => (dragged = null)}
        onclick={() => pickSlot(row, index)}
      >
        <DockShipCard icon={docked_?.icon ?? ship.icon} name={ship.name} rarity={ship.rarity} level={ship.level} star={docked_?.star ?? 0} maxStar={docked_?.max_star ?? 0} locked={docked_?.locked} married={docked_?.married} />
      </button>
      {#if b.duplicates.includes(ship.id)}<span class="ip-tag absolute -top-2.5 left-1/2 -translate-x-1/2 bg-red whitespace-nowrap text-white">IN 2 FLEETS</span>{/if}
      <button type="button" aria-label="Remove {ship.name}" class="{roundButton} absolute -top-2 -right-2 size-7 bg-[#6b7386] text-[15px] shadow-[0_2px_6px_rgb(0_0_0/0.4)] hover:bg-red" disabled={busy} onclick={() => removeShip(b, ship)}>×</button>
      <a href="/commanders/{id}/dock?ship={ship.id}" use:link aria-label="Open {ship.name} in the dock" title="Open in the dock" class="{roundButton} absolute -right-2 -bottom-2 size-7 bg-cyan no-underline shadow-[0_2px_6px_rgb(0_0_0/0.4)]">
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M7 17L17 7M9 7h8v8" /></svg>
      </a>
    {:else}
      <button
        type="button"
        aria-label="Add a {rowNames[row].toLowerCase()} ship"
        disabled={b.locked}
        class="grid aspect-[216/300] w-full cursor-pointer place-items-center rounded-[3px] border-2 border-dashed border-white/35 bg-white/6 text-white/70 hover:border-cyan hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
        onclick={() => pickSlot(row, index)}
      >
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-10" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
      </button>
    {/if}
  </div>
{/snippet}

<CommanderPage {id} title="Fleets">
  {#snippet children()}
    {#if query.error}
      <p class="text-bad">{query.error.message}</p>
    {:else if !data}
      <div class="ip-panel grid h-[640px] place-items-center text-lg text-dim">Loading…</div>
    {:else}
      <section aria-label="Fleets" class="flex flex-col gap-3.5">
        {#if board}
          <div class="relative h-[640px] overflow-hidden rounded bg-[radial-gradient(ellipse_at_42%_38%,#3a4670_0%,#222a44_45%,#121624_100%)] shadow-[0_8px_24px_rgb(0_0_0/0.35)]">
            <div aria-hidden="true" class="absolute inset-0 bg-[repeating-linear-gradient(62deg,transparent_0_220px,rgb(140_180_255/0.05)_220px_222px)]"></div>
            {#if painting && !paintingFailed}
              {#key painting}
                <img src="/api/painting/{painting}.png" alt="" aria-hidden="true" class="pointer-events-none absolute -right-10 bottom-0 h-[115%] object-contain opacity-45" onerror={() => (paintingFailed = true)} />
              {/key}
            {/if}
            <div aria-hidden="true" class="pointer-events-none absolute inset-0 bg-[linear-gradient(90deg,rgb(12_15_24/0.75)_0%,rgb(12_15_24/0.35)_55%,transparent_100%)]"></div>

            <div class="absolute top-5 left-6 flex max-w-[60%] flex-col gap-2">
              <div class="flex items-center gap-3">
                {#if board.regular}
                  <button type="button" aria-label="Rename {board.title}" class="cursor-pointer rounded border-0 bg-transparent px-1 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => ((name = data!.fleets.find((f) => f.id === board!.regular)?.name ?? ''), renameModal?.show())}>
                    <h2 class="m-0 text-[40px] leading-tight {shade}">{board.title}</h2>
                  </button>
                {:else}
                  <h2 class="m-0 px-1 text-[40px] leading-tight {shade}">{board.title}</h2>
                {/if}
                <div class={['flex gap-2', board.defense && 'hidden']}>
                  {#each ['Command', 'Support'] as role, i (role)}
                    {@const cat = data.meowfficers.find((c) => c.id === board!.cats[i])}
                    <span class="relative">
                    <button
                      type="button"
                      aria-label="{role} meowfficer: {cat?.name ?? 'none'}"
                      title="{role}: {cat?.name ?? 'none'}"
                      disabled={board.locked}
                      class="cursor-pointer rounded-full border-0 bg-[rgb(30_33_40/0.88)] p-1 hover:shadow-[0_0_0_2px_var(--color-cyan)] disabled:opacity-40"
                      onclick={() => ((catSlot = i), queueMicrotask(() => catPicker?.show()))}
                    >
                      {#if cat}
                        <Icon icon={cat.icon} rarity={tier(cat.rarity ?? 2, true)} size={44} name={cat.name} label={false} />
                      {:else}
                        <span class="grid size-11 place-items-center text-white/60">
                          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-6 fill-current"><circle cx="6" cy="10" r="2.2" /><circle cx="10" cy="6" r="2.2" /><circle cx="14" cy="6" r="2.2" /><circle cx="18" cy="10" r="2.2" /><path d="M12 11c-3 0-5.5 3.2-5.5 5.6 0 1.8 1.6 2.4 3 2.4 1 0 1.6-.5 2.5-.5s1.5.5 2.5.5c1.4 0 3-.6 3-2.4C17.5 14.2 15 11 12 11z" /></svg>
                        </span>
                      {/if}
                    </button>
                    {#if cat}
                      <a href="/commanders/{id}/meowfficers?cat={cat.id}" use:link aria-label="Open {cat.name} in Meowfficers" title="Open in Meowfficers" class="{roundButton} absolute -right-1.5 -bottom-1.5 size-6 bg-cyan no-underline shadow-[0_2px_6px_rgb(0_0_0/0.4)]">
                        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-3.5" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round"><path d="M7 17L17 7M9 7h8v8" /></svg>
                      </a>
                    {/if}
                    </span>
                  {/each}
                </div>
              </div>
              {#if board.sub}<span class="px-1 text-lg text-soft {shade}">{board.sub}</span>{/if}
              <div class="flex flex-wrap gap-2 px-1">
                {#if board.problem && problems[board.problem]}<span class="ip-tag bg-red text-white">{problems[board.problem]}</span>{/if}
                {#if board.duplicates.length}<span class="ip-tag bg-red text-white">{board.duplicates.length === 1 ? 'A ship is' : `${board.duplicates.length} ships are`} in another fleet</span>{/if}
                {#each board.limits as limit, i (i)}<span class="ip-tag bg-charcoal/85 text-white">{limit}</span>{/each}
              </div>
            </div>

            <div class="absolute top-5 right-5 flex gap-2.5">
              <button
                type="button"
                aria-pressed={overlays}
                class="flex h-12 cursor-pointer items-center gap-2.5 rounded-3xl border-0 bg-[rgb(30_33_40/0.88)] px-4 text-base text-white hover:bg-[rgb(50_54_64/0.95)]"
                onclick={() => (overlays = !overlays)}
              >
                <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                  {#if overlays}<path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" />{:else}<path d="M3 3l18 18M10.6 5.1A10 10 0 0 1 12 5c6.4 0 10 7 10 7a17 17 0 0 1-3.2 4M6.6 6.6A17 17 0 0 0 2 12s3.6 7 10 7a9.6 9.6 0 0 0 4.4-1" />{/if}
                </svg>
                Overlays
              </button>
              {#if !board.defense && (board.regular ? board.regular !== 1 && board.ships.length > 0 : set?.clear)}
                <button type="button" class="ip-danger h-12 rounded-3xl px-5 text-base" disabled={busy} onclick={clear}>{board.regular ? 'Clear fleet' : set?.clear?.label}</button>
              {/if}
            </div>

            {#if board.locked}
              <div class="absolute inset-0 grid place-items-center">
                <div class={[glass, 'flex flex-col items-center gap-3 px-8 py-6']}>
                  <svg aria-hidden="true" viewBox="0 0 24 24" class="size-10 fill-white"><path d="M7 10V8a5 5 0 0 1 10 0v2h1v11H6V10h1Zm2 0h6V8a3 3 0 0 0-6 0v2Z" /></svg>
                  <span class="text-xl">{board.unlockChapter ? `Locked until ${board.unlockChapter} is cleared` : 'Locked'}</span>
                  {#if board.unlockMap}<button type="button" class="ip-btn h-12 px-6" disabled={busy} onclick={() => unlock(board!)}>Clear chapters 1 to {board.unlockMap}</button>{/if}
                </div>
              </div>
            {:else}
              <div class="absolute inset-x-6 top-[200px] flex flex-wrap justify-center gap-12">
                {#each rows(board.kind) as row (row)}
                  <div class="flex flex-col gap-2.5">
                    <span class="ip-sub px-1 {shade}">{rowNames[row]}</span>
                    <div role="list" aria-label="{rowNames[row]} row" class="flex gap-4">
                      {#each [0, 1, 2] as index (index)}{@render slot(board, row, index)}{/each}
                    </div>
                  </div>
                {/each}
              </div>
            {/if}

            {#if overlays && !board.locked && board.ships.length}
              <dl aria-label="Fleet data" class={[glass, 'absolute bottom-5 left-6 m-0 flex items-center gap-6 px-5 py-3']}>
                <div class="flex flex-col"><dt class="ip-sub">Total Lv.</dt><dd class="num m-0 text-[30px] leading-tight">{levels.reduce((a, b) => a + b, 0)}</dd></div>
                <div class="flex flex-col"><dt class="ip-sub">Average Lv.</dt><dd class="num m-0 text-[30px] leading-tight">{Math.round(levels.reduce((a, b) => a + b, 0) / Math.max(1, levels.length))}</dd></div>
                {#if board.oil !== undefined}<div class="flex flex-col"><dt class="ip-sub">Oil a sortie</dt><dd class="num m-0 text-[30px] leading-tight">{board.oil}</dd></div>{/if}
                <div class="flex flex-col"><dt class="ip-sub">Hulls</dt><dd class="m-0 text-lg leading-[1.65]">{hulls.join(' · ')}</dd></div>
              </dl>
            {/if}
          </div>
        {:else}
          <div class="ip-panel grid h-[640px] place-items-center text-lg text-dim">{group === 'hard' ? 'No hard mode fleets saved' : group === 'events' ? 'No event fleets saved' : 'No fleet'}</div>
        {/if}

        <div class="flex flex-wrap items-center gap-3">
          <Seg
            label="Fleets"
            bind:value={() => group, (value) => ((group = value), (setKey = ''), (boardKey = ''))}
            options={[
              { value: 'regular', label: 'Regular' },
              { value: 'hard', label: 'Hard mode' },
              { value: 'events', label: 'Events' },
            ]}
          />
          {#if group !== 'regular' && sets[group].length}
            <select class="ip-field w-auto max-w-[320px]" aria-label={group === 'hard' ? 'Chapter' : 'Event'} bind:value={() => set?.key ?? '', (value) => ((setKey = value), (boardKey = ''))}>
              {#each sets[group] as option (option.key)}<option value={option.key}>{option.label}</option>{/each}
            </select>
          {/if}
        </div>

        {#if set}
          <nav aria-label="Fleet" class="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-2.5">
            {#each set.boards as b (b.key)}
              <button
                type="button"
                aria-pressed={board?.key === b.key}
                class={[
                  'flex h-[62px] cursor-pointer items-center gap-2 rounded border-0 px-4 text-left text-[19px] text-white shadow-[0_4px_12px_rgb(0_0_0/0.3)]',
                  board?.key === b.key ? 'bg-cyan' : 'bg-charcoal/90 hover:bg-[#3a3e48]',
                  dragged && dragged.board !== b.key && !b.locked && 'outline-2 outline-cyan outline-dashed',
                ]}
                ondragover={(e) => dragged && dragged.board !== b.key && !b.locked && e.preventDefault()}
                ondrop={() => dropOnTab(b)}
                onclick={() => (boardKey = b.key)}
              >
                {b.tab}
                {#if b.locked}
                  <svg role="img" aria-label="Locked" viewBox="0 0 24 24" class="ml-auto size-5 fill-[#aab2c2]"><path d="M7 10V8a5 5 0 0 1 10 0v2h1v11H6V10h1Zm2 0h6V8a3 3 0 0 0-6 0v2Z" /></svg>
                {:else if b.problem && problems[b.problem] || b.duplicates.length}
                  <span aria-label="Has a problem" class="ml-auto size-3 rotate-45 bg-[#d0634a] shadow-[0_0_0_2px_#f2c3a6]"></span>
                {:else}
                  <span class="num ml-auto text-2xl text-[#aab2c2]">{b.ships.length}</span>
                {/if}
              </button>
            {/each}
          </nav>
        {/if}
      </section>

      <Picker bind:this={picker} trigger={false} label={picking ? `${rowNames[picking.row]} ship` : 'Ship'} items={candidates} placeholder="Name, id or hull" value={null} onselect={picked} />
      <Picker bind:this={catPicker} trigger={false} label="{catSlot ? 'Support' : 'Command'} meowfficer" items={catItems} placeholder="Meowfficer name" value={board?.cats[catSlot] ?? 0} onselect={pickCat} />
      {#if board?.regular}
        <Modal bind:this={renameModal} title="Rename {board.title}">
          <form class="flex items-end gap-2.5" onsubmit={rename}>
            <label class="flex min-w-0 flex-1 flex-col gap-1.5 text-[15px] text-soft">Name <input class="ip-field w-full" placeholder={fleetName(board.regular)} bind:value={name} /></label>
            <button type="submit" class="ip-btn h-[46px] text-[19px]" disabled={busy}>Rename</button>
          </form>
        </Modal>
      {/if}
    {/if}
  {/snippet}
</CommanderPage>
