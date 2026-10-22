<script lang="ts">
  // The server's log as it happens (refetched on each live "logs" event):
  // the stream, narrowed by level, search and the sidebar's filters, and the
  // line picked.
  import { tick, untrack } from 'svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import { api } from '../../lib/api'
  import { rosterQuery } from '../../lib/query'
  import { fuzzy } from '../../lib/fuzzy'
  import { link } from '../../lib/router.svelte'
  import { clock, count, pad } from '../../lib/format'
  import { serverKeys, type LogEvent, type Logs } from '../../lib/server'
  import Page from '../../components/Page.svelte'
  import VirtualGrid from '../../components/VirtualGrid.svelte'

  // how many events the screen keeps, the logger keeps as many
  const keep = 2000

  let events = $state.raw<LogEvent[]>([])
  // the seq to ask after next time
  let after = $state(0)
  let query = $state('')
  let levels = $state<Record<string, boolean>>({ DEBUG: true, INFO: true, WARN: true, ERROR: true })
  let selected = $state<number | null>(null)
  let stream = $state<VirtualGrid<LogEvent>>()

  // each fetch asks for the events after the last one and appends them
  const logs = createQuery(() => ({
    queryKey: serverKeys.logs,
    queryFn: async () => {
      const loaded = await api<Logs>('GET', `/server/logs?after=${untrack(() => after)}`)
      const follow = stream?.atEnd() ?? true
      // a restarted server counts from 0 again
      const base = loaded.next < untrack(() => after) ? [] : untrack(() => events)
      after = loaded.next
      if (loaded.events.length || base.length === 0) events = [...base, ...loaded.events].slice(-keep)
      if (follow && loaded.events.length) tick().then(() => stream?.scrollToEnd())
      return loaded
    },
    staleTime: 0,
    // the events live in this screen: a new visit starts over
    gcTime: 0,
  }))

  // the commanders' names, for the events that only say commander=<id>
  const roster = createQuery(rosterQuery)
  const names = $derived.by(() => {
    const names: Record<number, string> = {}
    for (const row of roster.data ?? []) names[row.id] = row.name
    return names
  })

  function field(event: LogEvent, key: string): string {
    return event.fields.find(([k]) => k === key)?.[1] ?? ''
  }
  function commanderOf(event: LogEvent): number {
    return Number(field(event, 'commander')) || 0
  }
  function details(event: LogEvent): string {
    const shown = event.fields.filter(([k]) => k !== 'commander').map(([k, v]) => `${k}=${v}`)
    return [...shown, event.message].join(' | ')
  }
  function raw(event: LogEvent): string {
    const at = new Date(event.time)
    const date = `${pad(at.getDate())}/${pad(at.getMonth() + 1)}/${at.getFullYear()} ${clock(event.time / 1000, true)}`
    const fields = event.fields.map(([k, v]) => `${k}=${v || '-'}`).join(' ')
    return [date, event.level, event.scope, fields, event.message].filter(Boolean).join(' | ')
  }
  function who(id: number): string {
    return `${names[id] || 'Commander'} #${id}`
  }

  // the sidebar's filters: what the scopes tell (Handler/Missing, a Client
  // reply with a result, the connections and logins)
  type Filter = 'all' | 'missing' | 'refused' | 'connections'
  let filter = $state<Filter>('all')
  const connectionScopes = new Set(['Server/Hello', 'Server/Goodbye', 'Server/Disconnect', 'Client/Close', 'Gateway/Login', 'Gateway/ServerLogin'])
  const filters: Record<Filter, (event: LogEvent) => boolean> = {
    all: () => true,
    missing: (event) => event.scope === 'Handler/Missing',
    refused: (event) => event.scope.startsWith('Client') && field(event, 'result') !== '' && field(event, 'result') !== '0' && field(event, 'reply') !== '10998',
    connections: (event) => connectionScopes.has(event.scope),
  }

  const [debounced] = createDebouncedValue(() => query, { wait: 150 })
  // in time order: the stream reads as a log, the search only narrows it
  const shown = $derived.by(() => {
    const kept = events.filter((event) => levels[event.level] && filters[filter](event))
    const q = debounced().trim()
    if (!q) return kept
    const found = new Set(
      fuzzy(kept, q, (event) => {
        const commander = commanderOf(event)
        return [event.scope, event.message, ...event.fields.map(([k, v]) => `${k}=${v}`), commander ? names[commander] : undefined]
      }),
    )
    return kept.filter((event) => found.has(event))
  })

  const current = $derived(events.find((event) => event.seq === selected) ?? null)
  const hasDebug = $derived(events.some((event) => event.level === 'DEBUG'))

  const tags: Record<string, string> = { DEBUG: 'bg-[#6b7386]', INFO: 'bg-[#3d8bff]', WARN: 'bg-warn', ERROR: 'bg-[#ef4b42]' }
  const levelChoices = $derived([...(hasDebug ? [{ level: 'DEBUG', label: 'Debug' }] : []), { level: 'INFO', label: 'Info' }, { level: 'WARN', label: 'Warn' }, { level: 'ERROR', label: 'Error' }])

  const sections = $derived(
    (
      [
        { id: 'all', label: 'All', icon: 'M4 4h16v16H4z M8 9l3 3-3 3 M13 15h4' },
        { id: 'missing', label: 'Missing handlers', icon: 'M9 3h6v4h4v6h-4v4H9v4H3v-6h4v-4H3V5h6z' },
        { id: 'refused', label: 'Refused requests', icon: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z M5.6 5.6l12.8 12.8' },
        { id: 'connections', label: 'Connections', icon: 'M7 3v5 M17 3v5 M5 8h14v3a7 7 0 0 1-14 0z M12 18v3' },
      ] as { id: Filter; label: string; icon: string }[]
    ).map((section) => ({ ...section, count: events.filter(filters[section.id]).length })),
  )
  const active = $derived(sections.find((section) => section.id === filter))
</script>

{#snippet actions()}
  <div role="group" aria-label="Levels shown" class="ip-seg">
    {#each levelChoices as choice (choice.level)}
      <button type="button" aria-pressed={levels[choice.level]} onclick={() => (levels[choice.level] = !levels[choice.level])}>
        <span aria-hidden="true" class={['size-2.5 rounded-[2px]', tags[choice.level]]}></span>{choice.label}
      </button>
    {/each}
  </div>
  <label class="relative flex items-center">
    <span class="sr-only">Search the log</span>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
    <input type="search" class="ip-field w-[310px] pl-10" placeholder="Scope, packet or commander" bind:value={query} />
  </label>
{/snippet}

<Page title="Logs" sub="RECORDS" back="/" {actions}>
  <div class="relative flex items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <nav aria-label="Log filters" class="ip-side max-h-[calc(100vh-32px)] w-[150px] flex-none py-2">
      {#each sections as section, i (section.id)}
        <button
          type="button"
          class={['ip-tab w-full cursor-pointer border-x-0 border-t-0 bg-transparent font-[inherit]', i === sections.length - 1 && 'border-b-0']}
          aria-current={filter === section.id ? 'page' : undefined}
          onclick={() => (filter = section.id)}
        >
          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[34px]" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d={section.icon} /></svg>
          {section.label}
          <span class="num text-lg text-white/70">{count(section.count)}</span>
        </button>
      {/each}
    </nav>

    <section aria-labelledby="h-log" class="ip-panel min-w-0 flex-1">
      <div class="ip-head items-center">
        <h2 id="h-log">{active?.label === 'All' ? 'Server log' : active?.label}</h2>
        <span class="ml-auto text-sm text-soft"><span class="num text-[22px] text-white">{count(shown.length)}</span> shown</span>
      </div>
      <VirtualGrid bind:this={stream} items={shown} key={(event) => event.seq} rowHeight={40} gap={0} overscan={10} label="Log events" class="h-[calc(100vh-260px)] min-h-[420px] p-2.5">
        {#snippet cell(event)}
          {@const commander = commanderOf(event)}
          {@const line = details(event)}
          <button
            type="button"
            aria-pressed={selected === event.seq}
            onclick={() => (selected = event.seq)}
            class={['ip-row grid h-9 w-full cursor-pointer grid-cols-[76px_58px_minmax(0,150px)_minmax(0,1fr)_auto] items-center gap-2.5 border-0 px-3 py-1 text-left font-[inherit] text-white', selected === event.seq && 'ip-sel']}
          >
            <span class="num text-[19px]">{clock(event.time / 1000, true)}</span>
            <span class={['ip-tag justify-self-start text-center text-white', tags[event.level]]}>{event.level}</span>
            <span class="truncate text-[15px] text-link" title={event.scope}>{event.scope}</span>
            <span class="truncate text-sm" title={line}>{line}</span>
            {#if commander}<span class="truncate text-sm text-gold">{who(commander)}</span>{:else}<span></span>{/if}
          </button>
        {/snippet}
        {#snippet empty()}
          <p class="m-0 px-3 py-4 text-base text-dim">Nothing logged</p>
        {/snippet}
      </VirtualGrid>
    </section>

    {#if current}
      {@const commander = commanderOf(current)}
      <section aria-label="Line" class="ip-panel sticky top-4 w-[440px] flex-none">
        <div class="flex flex-col gap-3 px-5 pt-4 pb-5">
          {#if current.fields.length}
            <dl class="m-0 grid grid-cols-3 gap-x-4 gap-y-2.5">
              {#each current.fields as [key, value], i (i)}
                <div class="flex min-w-0 flex-col">
                  <dt class="ip-sub">{key}</dt>
                  <dd class="num m-0 truncate text-[22px]" title={value}>{key === 'commander' && Number(value) ? who(Number(value)) : value || '-'}</dd>
                </div>
              {/each}
            </dl>
          {/if}
          <p class="m-0 text-[17px] break-words">{current.message}</p>
          <code class="block overflow-x-auto rounded-[3px] bg-black/30 px-3 py-2 text-[13px] font-normal whitespace-pre text-soft">{raw(current)}</code>
          <div class="flex flex-wrap gap-3">
            {#if commander}
              <a href="/commanders/{commander}" use:link class="ip-btn">Open {names[commander] || `#${commander}`}</a>
            {/if}
            <button type="button" class="ip-btn2" onclick={() => (query = current.scope)}>Show only {current.scope}</button>
          </div>
        </div>
      </section>
    {/if}
  </div>
</Page>
