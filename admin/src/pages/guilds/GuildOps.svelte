<script lang="ts">
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Seg from '../../components/Seg.svelte'
  import Icon from '../../components/Icon.svelte'
  import { api } from '../../lib/api'
  import { createWrite, get, keys } from '../../lib/query'
  import { link } from '../../lib/router.svelte'
  import { count, recent, stamp } from '../../lib/format'
  import { guildKeys, type GuildOps } from '../../lib/moderation'

  let { id }: { id: number } = $props()

  let rank = $state<'donations' | 'missions' | 'operations'>('donations')

  const ranks = [
    { value: 'donations' as const, label: 'Donations' },
    { value: 'missions' as const, label: 'Missions' },
    { value: 'operations' as const, label: 'Operations' },
  ]

  const query = createQuery(() => ({ queryKey: guildKeys.ops(id), queryFn: get<GuildOps>(`/guilds/${id}/operations`) }))
  const ops = $derived(query.data ?? null)

  // an application answered changes that commander: its screens refetch too
  const client = useQueryClient()
  const write = createWrite((change: { path: string; member?: number; done: string }) => api('POST', change.path), [guildKeys.all], {
    success: (_, change) => change.done,
    onSuccess: (_, change) => {
      if (change.member) client.invalidateQueries({ queryKey: keys.commander(change.member) })
    },
  })
  const busy = $derived(write.isPending)

  function request(member: { id: number; name: string }, action: 'accept' | 'reject') {
    write.mutate({ path: `/guilds/${id}/requests/${member.id}/${action}`, member: member.id, done: action === 'accept' ? `${member.name} joined` : `${member.name} rejected` })
  }

  function operation(action: 'end' | 'reset') {
    const what = action === 'end' ? 'End the operation? Nothing progresses any more.' : 'Start the operation over? Its missions, fleets and boss are drawn again.'
    if (!confirm(what)) return
    write.mutate({ path: `/guilds/${id}/operation/${action}`, done: action === 'end' ? 'Operation ended' : 'Operation started over' })
  }

  const op = $derived(ops?.operation ?? null)
  const missionsDone = $derived(op ? op.missions.filter((m) => m.finished).length : 0)
  const nodeColor = (state: number) => (state === 1 ? 'bg-[#7fe0a4]' : state === 2 ? 'bg-[#ef4b42]' : 'bg-[#1e222b]')
</script>

<Page title="Guilds" sub={ops?.name.toUpperCase() ?? 'GUILD'} back="/guilds/{id}" backLabel="Back to the guild">
  {#snippet actions()}
    <nav aria-label="Guild screen" class="ip-seg">
      <a href="/guilds/{id}" use:link>Overview</a>
      <a href="/guilds/{id}/operations" use:link aria-current="page">Operations</a>
    </nav>
  {/snippet}
  {#if query.error}
    <p class="px-16 pt-6 text-bad">{query.error.message}</p>
  {:else if ops}
    <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
      <div class="flex min-w-0 flex-[1_1_640px] flex-col gap-6">
        {#if op}
          <section aria-label={op.name} class="ip-panel flex flex-col">
            <div class="ip-head items-center">
              <h2>{op.name}</h2>
              <span class={['ip-sub', op.open ? 'text-cyan' : 'text-soft']}>{op.open ? 'IN PROGRESS' : op.ended ? 'ENDED' : 'OVER'}</span>
              <span class="ml-auto text-sm text-soft">Ends <span class="num text-lg text-white">{stamp(op.end_time)}</span></span>
            </div>
            <div class="flex flex-col gap-3 p-4">
              <div class="grid grid-cols-4 gap-2.5">
                {#each [{ label: 'MEMBERS JOINED', value: `${op.joined} / ${ops.members}` }, { label: 'MISSIONS DONE', value: `${missionsDone} / ${op.missions.length}` }, { label: 'SHIPS SENT', value: op.ships }, { label: 'REPORTS', value: op.reports }] as tile (tile.label)}
                  <div class="ip-row flex flex-col gap-1 px-3 py-2">
                    <span class="ip-sub">{tile.label}</span>
                    <span class="num text-2xl leading-none">{tile.value}</span>
                  </div>
                {/each}
              </div>
              <div class="ip-strip">MISSIONS</div>
              {#each op.missions as mission (mission.name + mission.position)}
                <div class="ip-row flex items-center gap-4 px-3 py-2">
                  <span class="flex min-w-0 flex-1 flex-col">
                    <span class="text-[17px]">{mission.name}{#if mission.elite}<span class="ip-sub ml-2 text-warn">ELITE</span>{/if}{#if mission.side}<span class="ip-sub ml-2">SIDE</span>{/if}</span>
                    <span class="text-xs text-soft"><span class="num text-sm">{mission.ships}</span> ships · {mission.finished ? 'done' : mission.started ? 'running' : 'not started'}</span>
                  </span>
                  <div class="relative h-2 w-[260px] flex-none rounded bg-black/50">
                    <div class={['h-full rounded', mission.finished ? 'bg-[#7fe0a4]' : 'bg-cyan']} style:width="{mission.progress * 100}%"></div>
                    {#each mission.nodes as [at, state] (at)}
                      <span class={['absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white', nodeColor(state)]} style:left="{at}%"></span>
                    {/each}
                  </div>
                  <span class="num w-16 flex-none text-right text-2xl">{Math.floor(mission.progress * 100)}%</span>
                </div>
              {/each}
              <div class="ip-row flex items-center gap-4 border-l-4 border-red px-3 py-2">
                <span class="flex min-w-0 flex-1 flex-col gap-1.5">
                  <span class="text-[19px]">{op.boss.name}<span class="ip-sub ml-2 text-bad">{op.boss.active ? 'BOSS' : 'BOSS · LOCKED'}</span></span>
                  <div class="h-2 rounded bg-black/50"><div class="h-full rounded bg-red" style:width="{op.boss.hp ? Math.max(0, 100 - (op.boss.damage / op.boss.hp) * 100) : 0}%"></div></div>
                </span>
                <span class="flex flex-none flex-col items-end leading-tight">
                  <span class="num text-2xl">{count(Math.max(0, op.boss.hp - op.boss.damage))}</span>
                  <span class="text-xs text-soft">of <span class="num text-sm">{count(op.boss.hp)}</span> HP</span>
                </span>
              </div>
              {#if op.open}
                <div class="flex justify-end gap-2.5">
                  <button type="button" class="ip-btn2" disabled={busy} onclick={() => operation('reset')}>Reset operation</button>
                  <button type="button" class="ip-danger" disabled={busy} onclick={() => operation('end')}>End operation</button>
                </div>
              {/if}
            </div>
          </section>
        {:else}
          <Panel title="Operation" class="flex flex-col"><p class="m-0 px-5 py-6 text-soft">No operation started</p></Panel>
        {/if}

        <div class="flex flex-wrap items-start gap-6">
          <Panel title="Technologies" sub="RESEARCH" class="flex min-w-[340px] flex-[1_1_380px] flex-col">
            <div class="flex flex-col gap-2 p-3">
              {#each ops.techs as tech (tech.group)}
                <div class={['ip-row flex flex-col gap-1 px-3 py-2', tech.active && 'ip-sel']}>
                  <div class="flex items-baseline gap-2">
                    <span class="min-w-0 flex-1 text-[15px]">{tech.name}</span>
                    <span class="text-xs text-soft">Lv.<span class="num text-lg text-white">{tech.level}</span>/<span class="num text-sm">{tech.level_max}</span></span>
                    <span class="num w-[90px] text-right text-base text-soft">{count(tech.progress)} / {count(tech.exp)}</span>
                  </div>
                  <div class="flex items-center gap-2">
                    {#if tech.active}<span class="ip-tag bg-cyan py-0 text-white">ACTIVE</span>{/if}
                    <div class="h-1.5 flex-1 rounded bg-black/50"><div class="h-full rounded bg-cyan" style:width="{tech.exp ? Math.min(100, (tech.progress / tech.exp) * 100) : 0}%"></div></div>
                  </div>
                </div>
              {/each}
            </div>
          </Panel>
          <Panel title="Ranks" class="flex min-w-[340px] flex-[1_1_380px] flex-col">
            <div class="flex flex-col gap-1.5 p-3">
              <div class="mb-1.5 self-start"><Seg options={ranks} bind:value={rank} label="Rank" /></div>
              <div class="grid grid-cols-[1fr_60px_60px_70px] gap-2 px-2 text-[11px] tracking-[0.2em] text-dim">
                <span>MEMBER</span><span class="text-right">WEEK</span><span class="text-right">MONTH</span><span class="text-right">TOTAL</span>
              </div>
              {#each ops.ranks[rank] as row, i (row.id)}
                <div class="ip-row grid grid-cols-[1fr_60px_60px_70px] items-baseline gap-2 px-2 py-1.5">
                  <span class="truncate"><span class="num mr-1 text-soft">{i + 1}</span><a href="/commanders/{row.id}" use:link class="text-gold">{row.name}</a></span>
                  <span class="num text-right text-lg">{count(row.week)}</span>
                  <span class="num text-right text-lg">{count(row.month)}</span>
                  <span class="num text-right text-lg">{count(row.total)}</span>
                </div>
              {:else}
                <p class="m-0 px-2 py-4 text-center text-soft">Nobody yet</p>
              {/each}
            </div>
          </Panel>
        </div>
      </div>

      <div class="flex min-w-[360px] flex-[0_1_400px] flex-col gap-6">
        <Panel title="Applications" class="flex flex-col">
          {#snippet actions()}<span class="num text-2xl">{ops?.requests.length}</span>{/snippet}
          <div class="flex max-h-[420px] flex-col gap-2 overflow-y-auto p-3">
            {#each ops.requests as req (req.id)}
              <div class="ip-row flex flex-col gap-2 p-3">
                <div class="flex items-center gap-2">
                  <Icon icon={req.icon} rarity={req.rarity} ship size={36} name={req.name} />
                  <a href="/commanders/{req.id}" use:link class="text-lg text-gold">{req.name}</a>
                  <span class="text-sm">Lv.<span class="num text-base">{req.level}</span></span>
                  <span class="num ml-auto text-base text-soft">{recent(req.time)}</span>
                </div>
                {#if req.content}<p class="m-0 text-[15px] break-words">{req.content}</p>{/if}
                <div class="grid grid-cols-2 gap-2">
                  <button type="button" class="ip-btn ip-sm" disabled={busy} onclick={() => request(req, 'accept')}>Accept</button>
                  <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => request(req, 'reject')}>Reject</button>
                </div>
              </div>
            {:else}
              <p class="m-0 px-2 py-4 text-center text-soft">No application</p>
            {/each}
          </div>
        </Panel>

        <Panel title="Logistics" sub="THIS WEEK" class="flex flex-col">
          <div class="flex flex-col gap-3 p-3">
            {#if ops.logistics.task}
              <div class="ip-row flex flex-col gap-2 px-3 py-2.5">
                <span class="text-sm">{ops.logistics.task}</span>
                <div class="flex items-center gap-3">
                  <div class="h-2 flex-1 rounded bg-black/50"><div class="h-full rounded bg-cyan" style:width="{ops.logistics.task_max ? Math.min(100, (ops.logistics.task_progress / ops.logistics.task_max) * 100) : 0}%"></div></div>
                  <span class="num text-lg">{ops.logistics.task_progress}/{ops.logistics.task_max}</span>
                </div>
              </div>
            {:else}
              <p class="m-0 px-1 text-soft">No weekly mission picked</p>
            {/if}
            <dl class="m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 px-1 text-[15px]">
              <dt>Supplies</dt>
              <dd class="m-0 text-sm text-soft">{#if ops.logistics.supply_until * 1000 > Date.now()}until <span class="num text-base text-white">{recent(ops.logistics.supply_until)}</span>{:else}none{/if}</dd>
              <dt>Donated today</dt>
              <dd class="m-0 text-sm text-soft"><span class="num text-base text-white">{ops.logistics.donated_capital}</span> funds · <span class="num text-base text-white">{ops.logistics.donated_tech}</span> tech</dd>
            </dl>
          </div>
        </Panel>

        <Panel title="Capital log" class="flex flex-col">
          {#snippet actions()}<span class="num text-2xl">{count(ops?.capital ?? 0)}</span>{/snippet}
          <ul class="m-0 flex max-h-[420px] list-none flex-col gap-1.5 overflow-y-auto p-3">
            {#each ops.capital_log as line, i (i)}
              <li class="ip-row flex items-center gap-3 px-3 py-1.5 text-sm">
                <span class="num w-[88px] flex-none text-base text-soft">{recent(line.time)}</span>
                <span class="min-w-0 flex-1">{#if line.name}<span class="text-gold">{line.name}</span>{' '}{/if}{line.text}</span>
                {#if line.amount}<span class={['num flex-none text-lg', line.amount > 0 ? 'text-ok' : 'text-bad']}>{line.amount > 0 ? '+' : ''}{count(line.amount)}</span>{/if}
              </li>
            {:else}
              <li class="px-2 py-4 text-center text-soft">No funds movement</li>
            {/each}
          </ul>
        </Panel>
      </div>
    </div>
  {/if}
</Page>
