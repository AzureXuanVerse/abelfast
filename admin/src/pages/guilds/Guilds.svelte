<script lang="ts">
  import { untrack } from 'svelte'
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Card from '../../components/Card.svelte'
  import Seg from '../../components/Seg.svelte'
  import Icon from '../../components/Icon.svelte'
  import ContentVirtualList from '../../components/ContentVirtualList.svelte'
  import { api } from '../../lib/api'
  import { fuzzy } from '../../lib/fuzzy'
  import { createWrite, get, keys } from '../../lib/query'
  import { link, navigate } from '../../lib/router.svelte'
  import { ago, count, day, stamp } from '../../lib/format'
  import { duties, factions, guildKeys, type GuildDetail, type GuildRow } from '../../lib/moderation'
  import { bounded, uint32 } from '../../lib/bounded'

  let { id }: { id?: number } = $props()

  let query = $state('')
  let faction = $state(0)
  let showAll = $state(false)

  // the edit card
  let name = $state('')
  let announce = $state('')
  let manifesto = $state('')
  let funds = $state(0)
  let leader = $state(0)

  const factionOptions = [
    { value: 0, label: 'All factions' },
    { value: 1, label: 'Azur Lane' },
    { value: 2, label: 'Crimson Axis' },
  ]
  const shortRoster = 8

  const list = createQuery(() => ({ queryKey: guildKeys.all, queryFn: get<GuildRow[]>('/guilds') }))
  const guilds = $derived(list.data ?? [])
  const selectedID = $derived(id ?? guilds[0]?.id)
  const detail = createQuery(() => ({
    queryKey: guildKeys.guild(selectedID ?? 0),
    queryFn: get<GuildDetail>(`/guilds/${selectedID}`),
    enabled: !!selectedID,
  }))
  const guild = $derived(detail.data ?? null)

  const [search] = createDebouncedValue(() => query, { wait: 150 })
  const shown = $derived(
    fuzzy(
      guilds.filter((g) => !faction || g.faction === faction),
      search(),
      (g) => [g.name, g.id, g.leader],
    ),
  )
  const roster = $derived(guild ? (showAll ? guild.roster : guild.roster.slice(0, shortRoster)) : [])
  const leaderID = $derived(guild?.roster.find((m) => m.duty === 1)?.id ?? 0)

  // the card follows the guild shown, and its changes once saved
  $effect(() => {
    if (guild) untrack(() => reset())
  })

  function reset() {
    if (!guild) return
    name = guild.name
    announce = guild.announce
    manifesto = guild.manifesto
    funds = 0
    leader = guild.roster.find((m) => m.duty === 1)?.id ?? 0
  }

  // a member's change may kick that member: its screens refetch too
  const client = useQueryClient()
  const write = createWrite(
    (change: { path: string; method: string; body?: unknown; member?: number; done: string }) => api(change.method, change.path, change.body),
    [guildKeys.all],
    {
      success: (_, change) => change.done,
      onSuccess: (_, change) => {
        if (change.member) client.invalidateQueries({ queryKey: keys.commander(change.member) })
      },
    },
  )

  function save() {
    if (!guild) return
    const body: Record<string, unknown> = {}
    if (name !== guild.name) body.name = name
    if (announce !== guild.announce) body.announce = announce
    if (manifesto !== guild.manifesto) body.manifesto = manifesto
    if (Number(funds) > 0) body.give_funds = Number(funds)
    if (leader && leader !== leaderID) body.leader = leader
    if (!Object.keys(body).length) return
    write.mutate({ method: 'POST', path: `/guilds/${guild.id}`, body, done: 'Guild saved' })
  }

  function setDuty(member: number, duty: number) {
    if (!guild) return
    write.mutate({ method: 'POST', path: `/guilds/${guild.id}/members/${member}`, body: { duty }, member, done: 'Duty changed' })
  }

  function remove(member: { id: number; name: string }) {
    if (!guild || !confirm(`Remove ${member.name} from ${guild.name}?`)) return
    write.mutate({ method: 'DELETE', path: `/guilds/${guild.id}/members/${member.id}`, member: member.id, done: `${member.name} removed` })
  }

  // the disbanded guild leaves the cache before the list refetches, so
  // nothing asks for it again
  const disbanding = createWrite(
    async (gone: GuildDetail) => {
      await api('DELETE', `/guilds/${gone.id}`)
      client.setQueryData<GuildRow[]>(guildKeys.all, (rows) => rows?.filter((row) => row.id !== gone.id))
      client.removeQueries({ queryKey: guildKeys.guild(gone.id) })
      navigate('/guilds')
    },
    [guildKeys.all, ['commander']],
    { success: (_, gone) => `${gone.name} disbanded` },
  )

  const busy = $derived(write.isPending || disbanding.isPending)

  function disband() {
    if (!guild || !confirm(`Disband ${guild.name}? Its members, log, research and operations are deleted.`)) return
    disbanding.mutate(guild)
  }

  const logWords: Record<string, string> = { joined: 'Joined', duty: 'Duty', left: 'Left', removed: 'Removed' }
  const logColors: Record<string, string> = { joined: 'bg-[#3d9ff0] text-white', duty: 'bg-[#8a93a6] text-white', left: 'bg-[#f0d36a] text-[#3a2a08]', removed: 'bg-[#ef4b42] text-white' }
  const logText = (line: GuildDetail['logs'][number]) =>
    line.kind === 'joined' ? `${line.name} joined the guild` : line.kind === 'left' ? `${line.name} left the guild` : line.kind === 'removed' ? `${line.name} was removed from the guild` : `${line.name} became ${duties[line.duty ?? 0] ?? 'duty ' + line.duty}`
  const factionDot = (f: number) => (f === 2 ? 'text-[#ff5a5a]' : 'text-[#4fa7ff]')
</script>

<Page title="Guilds" sub="GUILD" back="/">
  {#snippet actions()}
    <label class="relative flex items-center">
      <span class="sr-only">Find a guild</span>
      <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
      <input type="search" class="ip-field w-[170px] pl-10" placeholder="Guild or ID" bind:value={query} />
    </label>
    <nav aria-label="Guild screen" class="ip-seg">
      <a href={selectedID ? `/guilds/${selectedID}` : '/guilds'} use:link aria-current="page">Overview</a>
      <a href={selectedID ? `/guilds/${selectedID}/operations` : '/guilds'} use:link>Operations</a>
    </nav>
    <Seg options={factionOptions} bind:value={faction} label="Faction" />
  {/snippet}
  <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <Panel title="All guilds" class="flex w-[280px] flex-none flex-col">
      {#snippet actions()}<span class="num text-2xl">{guilds.length}</span>{/snippet}
      {#if shown.length}
        <ContentVirtualList items={shown} estimate={86} gap={8} key={(row) => row.id} class="max-h-[calc(100vh-160px)] p-2.5" label="Guilds">
          {#snippet row(row)}
          <a href="/guilds/{row.id}" use:link class={['ip-row flex items-end gap-2 border-l-4 px-3 py-2 text-white no-underline hover:text-white', row.faction === 2 ? 'border-[#ef4b42]' : 'border-cyan', row.id === selectedID && 'ip-sel']}>
            <span class="flex min-w-0 flex-1 flex-col">
              <span class="text-xl leading-tight break-words">{row.name}</span>
              <span class={['text-[13px]', factionDot(row.faction)]}>◆ {factions[row.faction] ?? 'Faction ' + row.faction}</span>
              <span class="text-[13px] text-soft">Leader {row.leader || '-'}</span>
            </span>
            <span class="flex flex-none flex-col items-end">
              <span class="text-xs text-soft">Lv. <span class="num text-2xl text-white">{row.level}</span></span>
              <span class="num text-xl text-soft">{row.members}/{row.max_members}</span>
            </span>
          </a>
          {/snippet}
        </ContentVirtualList>
      {:else if list.isSuccess}
        <p class="m-0 px-2 py-6 text-center text-soft">No guild</p>
      {:else if list.error}
        <p class="m-0 px-2 py-6 text-center text-bad">{list.error.message}</p>
      {/if}
    </Panel>

    {#if detail.error}
      <p class="m-0 flex-1 px-2 py-6 text-bad">{detail.error.message}</p>
    {:else if guild}
      <section aria-label={guild.name} class="ip-panel flex min-w-0 flex-[1_1_560px] flex-col">
        <div class="ip-head items-center">
          <span aria-hidden="true" class={['text-xl', factionDot(guild.faction)]}>◆</span>
          <h2>{guild.name}</h2>
          <span class="text-sm text-soft">ID <span class="num text-lg">{guild.id}</span></span>
          <span class={['ip-sub', factionDot(guild.faction)]}>{(factions[guild.faction] ?? '').toUpperCase()}</span>
          <span class="ml-auto text-sm text-soft">Founded <span class="num text-lg text-white">{day(guild.created_at)}</span></span>
        </div>
        <div class="flex flex-col gap-4 p-4">
          <div class="grid grid-cols-[1.4fr_1fr_1fr] gap-3">
            <div class="ip-row flex flex-col justify-center gap-2 px-3 py-2.5">
              <div class="flex items-baseline gap-2">
                <span class="text-sm text-soft">Lv.</span><span class="num text-3xl">{guild.level}</span>
                <span class="num ml-auto text-base text-soft">{count(guild.exp)} / {count(guild.exp_next)} EXP</span>
              </div>
              <div class="h-2 overflow-hidden rounded bg-black/40"><div class="h-full bg-cyan" style:width="{guild.exp_next ? Math.min(100, (guild.exp / guild.exp_next) * 100) : 100}%"></div></div>
            </div>
            <div class="ip-row flex items-center gap-2.5 px-3 py-2.5">
              <span class="flex flex-col">
                <span class="text-sm text-soft">Capital</span>
                <span class="num text-3xl leading-none">{count(guild.capital)}</span>
              </span>
            </div>
            <div class="ip-row flex flex-col px-3 py-2.5">
              <span class="text-sm text-soft">Tech researched</span>
              <span class="num text-3xl leading-none">{guild.tech_level}<span class="text-lg text-soft"> / {guild.tech_max}</span></span>
            </div>
          </div>

          {#if guild.operation}
            <a href="/guilds/{guild.id}/operations" use:link class="ip-row flex flex-wrap items-baseline gap-x-3 gap-y-1 border-l-4 border-cyan px-3 py-2.5 text-white no-underline hover:text-white">
              <span class="text-lg">Current operation</span>
              <span class={['ip-sub', guild.operation.open ? 'text-cyan' : 'text-soft']}>{guild.operation.open ? 'IN PROGRESS' : 'OVER'}</span>
              <span class="text-lg">
                {guild.operation.name},
                {#if guild.operation.main_done < guild.operation.main_count}stage <span class="num text-xl">{guild.operation.main_done + 1}</span> of <span class="num text-xl">{guild.operation.main_count}</span>{:else}boss{/if}
              </span>
              <span class="ml-auto text-sm text-soft"><span class="num text-base text-white">{guild.operation.joined}</span> of <span class="num text-base text-white">{guild.members}</span> joined · ends <span class="num text-base text-white">{day(guild.operation.end_time)}</span></span>
            </a>
          {/if}

          <div class="flex items-baseline gap-3">
            <h3 class="m-0 text-2xl">Members</h3>
            <span class="num text-2xl">{guild.members}/{guild.max_members}</span>
            <span class="ip-sub">CREW</span>
            {#if guild.roster.length > shortRoster && !showAll}
              <button type="button" class="ml-auto cursor-pointer border-0 bg-transparent p-0 font-game text-[15px] font-bold text-link underline" onclick={() => (showAll = true)}>Show the {guild.roster.length - shortRoster} other members</button>
            {/if}
          </div>
          <div class="flex flex-col gap-1.5">
            <div class="grid grid-cols-[1fr_130px_110px_96px] gap-3 px-2 text-[11px] tracking-[0.2em] text-dim">
              <span>COMMANDER</span><span>DUTY</span><span class="text-right">LIVENESS</span><span></span>
            </div>
            <ContentVirtualList items={roster} estimate={56} gap={6} key={(member) => member.id} class="max-h-[620px]" label="Members">
              {#snippet row(member)}
              <div class="ip-row grid grid-cols-[1fr_130px_110px_96px] items-center gap-3 px-2 py-1.5">
                <span class="flex min-w-0 items-center gap-2.5">
                  <Icon icon={member.icon} rarity={member.rarity} ship size={42} name={member.name} />
                  <span class="flex min-w-0 flex-col">
                  <a href="/commanders/{member.id}" use:link class="truncate text-[17px] text-gold">{member.name}</a>
                  <span class="text-xs text-soft">UID:<span class="num text-sm">{member.id}</span> · Lv.<span class="num text-sm">{member.level}</span> · {#if member.online}<span class="text-ok">Online</span>{:else}{ago(member.last_online)}{/if}</span>
                  </span>
                </span>
                {#if member.duty === 1}
                  <span class="justify-self-start rounded-full bg-white px-4 py-1 text-ink">{duties[1]}</span>
                {:else}
                  <select class="ip-field h-10" aria-label="Duty of {member.name}" value={member.duty} disabled={busy} onchange={(e) => setDuty(member.id, Number(e.currentTarget.value))}>
                    {#if member.duty === 5}<option value={5} disabled>{duties[5]}</option>{/if}
                    {#each [2, 3, 4] as duty (duty)}<option value={duty}>{duties[duty]}</option>{/each}
                  </select>
                {/if}
                <span class="num text-right text-2xl">{count(member.liveness)}</span>
                {#if member.duty !== 1}
                  <button type="button" class="ip-danger ip-sm" disabled={busy} onclick={() => remove(member)}>Remove</button>
                {/if}
              </div>
              {/snippet}
            </ContentVirtualList>
          </div>

          <div class="rounded bg-black/30 px-3 py-2.5">
            <div class="flex items-baseline gap-2.5">
              <h3 class="m-0 text-xl">Guild log</h3>
              <span class="ip-sub">RECENT</span>
            </div>
            <ul class="m-0 mt-2 flex max-h-[240px] list-none flex-col gap-1 overflow-y-auto p-0">
              {#each guild.logs as line, i (i)}
                <li class="flex items-center gap-3 text-[15px]">
                  <span class="num w-[130px] flex-none text-base text-soft">{stamp(line.time)}</span>
                  <span class={['w-[70px] flex-none rounded-[3px] text-center text-xs', logColors[line.kind] ?? 'bg-strip']}>{logWords[line.kind] ?? line.kind}</span>
                  <span class="min-w-0 flex-1">{logText(line)}</span>
                </li>
              {:else}
                <li class="text-soft">No log line</li>
              {/each}
            </ul>
          </div>
        </div>
      </section>

      <div class="flex min-w-[340px] flex-[0_1_360px] flex-col gap-5">
        <Card title="Edit {guild.name}">
          <div class="ip-strip">NAME</div>
          <input class="ip-field" aria-label="Name" maxlength="20" bind:value={name} />
          <div class="ip-strip">TEXTS</div>
          <label class="ip-lab">Announcement
            <textarea class="ip-field" rows="3" maxlength="200" bind:value={announce}></textarea>
          </label>
          <label class="ip-lab">Manifesto
            <textarea class="ip-field" rows="2" maxlength="200" bind:value={manifesto}></textarea>
          </label>
          <div class="ip-strip">FUNDS</div>
          <label class="flex items-center gap-3 text-[15px] text-ink-soft">Give
            <input class="ip-field num min-w-0 flex-1 text-2xl" type="number" min="0" bind:value={funds} use:bounded={{ min: 0, max: uint32 - (guild?.capital ?? 0) }} />
          </label>
          <div class="ip-strip">LEADER</div>
          <label class="flex items-center gap-3 text-[15px] text-ink-soft">New leader
            <select class="ip-field min-w-0 flex-1" bind:value={leader}>
              {#each guild.roster as member (member.id)}
                <option value={member.id}>{member.name}{member.id === leaderID ? ', current' : ''}</option>
              {/each}
            </select>
          </label>
          <div class="grid grid-cols-[1fr_1.6fr] gap-3">
            <button type="button" class="ip-btn2" disabled={busy} onclick={reset}>Reset</button>
            <button type="button" class="ip-btn" disabled={busy || !name.trim()} onclick={save}>Save changes</button>
          </div>
        </Card>
        <div class="ip-panel ip-danger-zone p-5">
          <button type="button" class="ip-danger w-full" disabled={busy} onclick={disband}>Disband guild</button>
        </div>
      </div>
    {/if}
  </div>
</Page>
