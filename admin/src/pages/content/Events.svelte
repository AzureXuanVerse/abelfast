<script lang="ts">
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Seg from '../../components/Seg.svelte'
  import ContentTabs from '../../components/ContentTabs.svelte'
  import ContentVirtualList from '../../components/ContentVirtualList.svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import { get } from '../../lib/query'
  import { createNow } from '../../lib/clock.svelte'
  import { pad } from '../../lib/format'
  import { activityName, activityType, contentKeys, serverDate, serverDay, serverMonthDay, zoneName, type Activity, type Events } from '../../lib/content'

  type Filter = 'all' | Activity['status']

  let filter = $state<Filter>('all')

  const query = createQuery(() => ({ queryKey: contentKeys.events, queryFn: get<Events>('/events') }))
  const data = $derived(query.data ?? null)
  // the server's clock, ticking here
  const ticker = createNow(1000, () => data?.clock.now)

  const filters: { value: Filter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'live', label: 'Live' },
    { value: 'upcoming', label: 'Upcoming' },
    { value: 'ended', label: 'Ended' },
  ]
  const order: Record<Activity['status'], number> = { live: 0, upcoming: 1, ended: 2 }

  const offset = $derived(data?.clock.offset ?? 0)
  const now = $derived(data?.clock.now ?? 0)
  const activities = $derived(
    (data?.activities ?? [])
      .filter((a) => filter === 'all' || a.status === filter)
      .slice()
      .sort((a, b) => order[a.status] - order[b.status] || (a.status === 'ended' ? b.end - a.end : a.end - b.end) || a.id - b.id),
  )
  const counts = $derived.by(() => {
    const c = { live: 0, upcoming: 0, ended: 0 }
    for (const a of data?.activities ?? []) c[a.status]++
    return c
  })

  // the timeline: from the first of the month two months back to the
  // first of the month three months ahead, server clock
  const axis = $derived.by(() => {
    const d = new Date((now + offset) * 1000)
    const months: { label: string; at: number }[] = []
    for (let m = -2; m <= 3; m++) {
      const first = Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + m, 1) / 1000 - offset
      months.push({ label: new Date((first + offset) * 1000).toLocaleString('en-US', { month: 'short', timeZone: 'UTC' }).toUpperCase(), at: first })
    }
    return { months: months.slice(0, -1), from: months[0].at, to: months[months.length - 1].at }
  })
  const at = (unix: number) => Math.min(100, Math.max(0, ((unix - axis.from) / (axis.to - axis.from)) * 100))
  const days = (seconds: number) => Math.max(1, Math.round(seconds / 86400))

  function status(a: Activity): string {
    if (a.status === 'live') return `Ends in ${days(a.end - now)} days`
    if (a.status === 'upcoming') return `Starts in ${days(a.start - now)} days`
    return `Ended ${days(now - a.end)} days ago`
  }

  const rarities = [
    { rarity: 6, label: 'UR', color: 'text-tier-5' },
    { rarity: 5, label: 'SR', color: 'text-tier-4' },
    { rarity: 4, label: 'Elite', color: 'text-tier-3' },
    { rarity: 3, label: 'Rare', color: 'text-tier-2' },
    { rarity: 2, label: 'Common', color: 'text-white' },
  ]
  const shownRarities = $derived(rarities.filter((r) => data?.pools.some((p) => p.rates[r.rarity] !== undefined)))
  const clockNow = $derived(ticker.now)
  const capital = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)
  const percent = (n: number) => (Number.isInteger(n) ? n : n.toFixed(1)) + '%'
</script>

<Page title="Events" sub="SCHEDULE" back="/">
  {#snippet actions()}
    {#if data}<span class="text-soft">Game data <span class="text-white">{data.region}</span> <span class="num text-xl text-white">{data.data}</span></span>{/if}
    <Seg options={filters} bind:value={filter} label="Status" />
  {/snippet}
  <div class="relative flex items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <ContentTabs />
    <div class="flex min-w-0 flex-1 flex-wrap items-start gap-6">
      <Panel title="Activities" sub="TIMELINE" class="min-w-0 flex-[1_1_640px]">
        {#snippet actions()}
          <span class="flex gap-4 text-sm text-soft">
            <span>Live <span class="num text-lg text-white">{counts.live}</span></span>
            <span>Upcoming <span class="num text-lg text-white">{counts.upcoming}</span></span>
            <span>Ended <span class="num text-lg text-white">{counts.ended}</span></span>
          </span>
        {/snippet}
        {#if data}
          <div class="relative mx-[52px] mt-3 h-7">
            {#each axis.months as month (month.at)}
              <span class="ip-sub absolute top-1 border-l border-white/20 pl-1.5" style:left="{at(month.at)}%">{month.label}</span>
            {/each}
            <span class="num absolute -top-0.5 -translate-x-1/2 rounded-full bg-cyan px-2.5 text-lg whitespace-nowrap text-white" style:left="{at(now)}%">TODAY {serverMonthDay(now, offset)}</span>
          </div>
        {/if}
        {#if activities.length}
          <ContentVirtualList items={activities} estimate={104} gap={10} key={(a) => a.id} class="max-h-[calc(100vh-230px)] min-h-[400px] p-3" label="Activities">
            {#snippet row(a)}
              <article class="ip-row flex flex-col gap-2 px-10 py-3">
                <div class="flex items-start gap-4">
                  <div class="flex min-w-0 flex-col">
                    <span class="text-xs tracking-[0.16em] text-link uppercase">{activityType(a.type)} · <span class="num tracking-normal">{a.id}</span></span>
                    <h3 class="m-0 truncate text-xl">{activityName(a)}</h3>
                  </div>
                  <span class="num ml-auto text-2xl whitespace-nowrap text-white">{serverMonthDay(a.start, offset)} - {serverMonthDay(a.end, offset)}</span>
                </div>
                <div class="relative h-2.5 rounded-full bg-black/40">
                  <span
                    class={['absolute inset-y-0 rounded-full', a.status === 'live' ? 'bg-[#3d8af0]' : a.status === 'upcoming' ? 'bg-warn' : 'bg-white/25']}
                    style:left="{at(a.start)}%"
                    style:width="{Math.max(0.8, at(a.end) - at(a.start))}%"
                  ></span>
                  <span class="absolute -inset-y-1 w-0.5 bg-cyan" style:left="{at(now)}%"></span>
                </div>
                <div class="flex items-center gap-3 text-[15px] text-[#e5edff]">
                  {status(a)}
                  {#if !a.shown}<span class="ip-tag bg-white/10 text-soft">HIDDEN</span>{/if}
                </div>
              </article>
            {/snippet}
          </ContentVirtualList>
        {:else if data}
          <p class="m-0 py-6 text-center text-soft">No activity</p>
        {:else if query.error}
          <p class="m-0 py-6 text-center text-bad">{query.error.message}</p>
        {/if}
      </Panel>

      <div class="flex min-w-[320px] flex-[0_1_380px] flex-col gap-6">
        <Panel title="Shops" sub="ROTATION">
          <div class="flex flex-col px-5 py-2">
            {#each data?.shops ?? [] as shop (shop.name)}
              <div class="flex items-center gap-3 border-b border-white/10 py-2 last:border-0">
                <div class="flex min-w-0 flex-col">
                  <span class="text-[17px] leading-tight">{shop.name}</span>
                  <span class="text-[13px] text-soft">{shop.rule}</span>
                </div>
                <span class="ml-auto whitespace-nowrap text-soft">Next <span class="num text-xl text-white">{serverDate(shop.next, offset).slice(5)}</span></span>
              </div>
            {/each}
          </div>
        </Panel>

        <Panel title="Build pools" sub="RATES">
          <table class="w-full px-5 py-2 text-left">
            <thead>
              <tr class="text-[17px]">
                <th class="py-1.5 pr-3 pl-5">Pool</th>
                {#each shownRarities as r (r.rarity)}<th class={['py-1.5 pr-4 text-right', r.color]}>{r.label}</th>{/each}
              </tr>
            </thead>
            <tbody>
              {#each data?.pools ?? [] as pool (pool.id)}
                <tr class="border-t border-white/10 text-[17px]">
                  <td class="py-1.5 pr-3 pl-5">{capital(pool.name)}</td>
                  {#each shownRarities as r (r.rarity)}
                    <td class="num py-1.5 pr-4 text-right text-xl">{pool.rates[r.rarity] !== undefined ? percent(pool.rates[r.rarity]) : '-'}</td>
                  {/each}
                </tr>
              {/each}
            </tbody>
          </table>
        </Panel>

        <Panel title="Server time" sub="CLOCK">
          {#if data}
            <dl class="m-0 grid grid-cols-[auto_1fr] items-baseline gap-x-4 gap-y-3 px-5 py-4 text-[17px]">
              <dt>Server clock</dt>
              <dd class="num m-0 text-right text-2xl">{serverDate(clockNow, offset)}:{pad(new Date((clockNow + offset) * 1000).getUTCSeconds())}</dd>
              <dt>Daily reset</dt>
              <dd class="m-0 text-right"><span class="num text-2xl">00:00</span> server time</dd>
              <dt>Weekly reset</dt>
              <dd class="num m-0 text-right text-2xl">{serverDay(data.clock.next_week, offset)}</dd>
              <dt>Monthly reset</dt>
              <dd class="num m-0 text-right text-2xl">{serverDay(data.clock.next_month, offset)}</dd>
              <dt>Timezone</dt>
              <dd class="num m-0 text-right text-2xl">{zoneName(offset)}</dd>
            </dl>
          {/if}
        </Panel>
      </div>
    </div>
  </div>
</Page>
