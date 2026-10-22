<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Card from '../../components/Card.svelte'
  import Seg from '../../components/Seg.svelte'
  import Icon from '../../components/Icon.svelte'
  import ContentRewards from '../../components/ContentRewards.svelte'
  import ContentVirtualList from '../../components/ContentVirtualList.svelte'
  import { toast } from '../../components/Toast.svelte'
  import { api, type Drop } from '../../lib/api'
  import { fuzzy } from '../../lib/fuzzy'
  import { createWrite, get } from '../../lib/query'
  import { shipFramed } from '../../lib/players'
  import { link } from '../../lib/router.svelte'
  import { contentKeys, plainDrops, serverDate, serverShort, type Code, type Codes, type Reward } from '../../lib/content'

  type Filter = 'all' | Code['status']

  let filter = $state<Filter>('all')
  let search = $state('')

  let code = $state('')
  let title = $state('')
  let content = $state('')
  let rewards = $state<Reward[]>([])
  let start = $state('')
  let end = $state('')

  const filters: { value: Filter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'active', label: 'Active' },
    { value: 'scheduled', label: 'Scheduled' },
    { value: 'ended', label: 'Ended' },
  ]
  const statusLabel: Record<Code['status'], string> = { active: 'Active', scheduled: 'Scheduled', ended: 'Ended' }
  const statusColor: Record<Code['status'], string> = { active: 'text-ok', scheduled: 'text-gold', ended: 'text-dim' }

  const query = createQuery(() => ({ queryKey: contentKeys.codes, queryFn: get<Codes>('/codes') }))
  const data = $derived(query.data ?? null)
  const [debounced] = createDebouncedValue(() => search, { wait: 150 })

  const codes = $derived(
    fuzzy(
      (data?.codes ?? []).filter((c) => filter === 'all' || c.status === filter),
      debounced(),
      (c) => [c.code, c.title],
    ),
  )
  const redemptions = $derived(fuzzy(data?.redemptions ?? [], debounced(), (r) => [r.code, r.name, r.commander_id]))
  const active = $derived(data?.codes.filter((c) => c.status === 'active').length ?? 0)
  const scheduled = $derived(data?.codes.filter((c) => c.status === 'scheduled').length ?? 0)
  const offset = $derived(data?.offset ?? 0)

  function generate() {
    const letters = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
    const random = crypto.getRandomValues(new Uint8Array(10))
    code = Array.from(random, (n) => letters[n % letters.length]).join('')
  }

  function clear() {
    code = title = content = start = end = ''
    rewards = []
  }

  const creating = createWrite(
    (body: { code: string; title: string; content: string; rewards: Drop[]; start: string; end: string }) => api('POST', '/codes', body),
    [contentKeys.codes],
    { success: (_, body) => `${body.code.toUpperCase()} works now`, onSuccess: () => clear() },
  )

  function create() {
    creating.mutate({ code, title, content, rewards: plainDrops(rewards), start, end })
  }

  const acting = createWrite(({ method, path }: { method: string; path: string; done: string }) => api(method, path), [contentKeys.codes], {
    success: (_, { done }) => done,
  })

  function act(method: string, path: string, question: string, done: string) {
    if (!confirm(question)) return
    acting.mutate({ method, path, done })
  }

  async function copy(c: Code) {
    try {
      await navigator.clipboard.writeText(c.code)
      toast(`${c.code} copied`)
    } catch {
      toast('The clipboard is unavailable', true)
    }
  }
</script>

<Page title="Redeem codes" sub="EXCHANGE" back="/">
  {#snippet actions()}
    <label class="relative">
      <svg aria-hidden="true" viewBox="0 0 24 24" class="pointer-events-none absolute top-[13px] left-3.5 size-5 text-ink-soft" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><circle cx="11" cy="11" r="6" /><path d="M20 20l-4.5-4.5" /></svg>
      <input class="ip-field w-[270px] pl-10" placeholder="Code or commander" aria-label="Search codes" bind:value={search} />
    </label>
    <Seg options={filters} bind:value={filter} label="Status" />
  {/snippet}
  <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <div class="flex min-w-0 flex-[1_1_760px] flex-col gap-6">
      <Panel title="Codes" sub="CODE LIST">
        {#snippet actions()}
          <span class="text-soft"><span class="num text-xl text-white">{active}</span> active, <span class="num text-xl text-white">{scheduled}</span> scheduled</span>
        {/snippet}
        <div class="overflow-x-auto p-3">
          <table class="w-full border-separate border-spacing-y-2 text-left">
            <thead>
              <tr class="ip-sub">
                <th class="px-4 font-bold">CODE</th>
                <th class="px-2 font-bold">REWARDS</th>
                <th class="px-2 font-bold">USES</th>
                <th class="px-2 font-bold">WINDOW</th>
                <th><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {#each codes as c (c.code)}
                <tr class="bg-white/6">
                  <td class="rounded-l-[3px] px-4 py-3 align-middle">
                    <div class={['text-xl tracking-[0.06em]', c.status === 'ended' && 'text-soft']}>{c.code}</div>
                    <div class={['text-[15px]', statusColor[c.status]]}>{statusLabel[c.status]}</div>
                  </td>
                  <td class="px-2 py-3">
                    <div class="flex flex-wrap gap-1.5">
                      {#each c.rewards as r (r.type + ':' + r.id)}
                        <Icon icon={r.icon} rarity={r.rarity} ship={shipFramed(r.type)} count={r.count} size={46} name={r.name} />
                      {/each}
                    </div>
                  </td>
                  <td class="px-2 py-3 whitespace-nowrap">
                    <div class="num text-2xl">{c.uses}</div>
                    <div class="text-[13px] text-soft">Once per commander</div>
                  </td>
                  <td class="px-2 py-3 whitespace-nowrap">
                    {#if c.start}
                      <div class="flex items-baseline gap-2"><span class="ip-sub w-12">FROM</span><span class="num text-xl">{serverDate(c.start, offset)}</span></div>
                      <div class="flex items-baseline gap-2"><span class="ip-sub w-12">UNTIL</span><span class="num text-xl">{serverDate(c.end, offset)}</span></div>
                    {:else}
                      <span class="text-soft">Always</span>
                    {/if}
                  </td>
                  <td class="rounded-r-[3px] px-3 py-3">
                    <div class="flex w-[92px] flex-col gap-1.5">
                      <button type="button" class="ip-btn2 ip-sm" aria-label="Copy {c.code}" onclick={() => copy(c)}>Copy</button>
                      {#if c.status !== 'ended'}
                        <button type="button" class="ip-danger ip-sm" aria-label="End {c.code}" disabled={acting.isPending} onclick={() => act('POST', `/codes/${encodeURIComponent(c.code)}/end`, `End ${c.code} now?`, `${c.code} ended`)}>End</button>
                      {:else}
                        <button type="button" class="ip-danger ip-sm" aria-label="Delete {c.code}" disabled={acting.isPending} onclick={() => act('DELETE', `/codes/${encodeURIComponent(c.code)}`, `Delete ${c.code}? Its redemptions stay.`, `${c.code} deleted`)}>Delete</button>
                      {/if}
                    </div>
                  </td>
                </tr>
              {:else}
                {#if data}<tr><td colspan="5" class="py-6 text-center text-soft">No code</td></tr>{:else if query.error}<tr><td colspan="5" class="py-6 text-center text-bad">{query.error.message}</td></tr>{/if}
              {/each}
            </tbody>
          </table>
        </div>
      </Panel>

      <Panel title="Redemptions" sub="LOG">
        {#snippet actions()}<span class="num text-2xl">{redemptions.length}</span>{/snippet}
        <div class="p-3 pb-0">
          <div class="ip-sub grid grid-cols-[150px_minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1.6fr)] gap-3 px-3 font-bold">
            <span>TIME</span><span>COMMANDER</span><span>CODE</span><span>DELIVERED</span>
          </div>
        </div>
        {#if redemptions.length}
          <ContentVirtualList items={redemptions} estimate={58} gap={6} key={(r) => r.commander_id + ':' + r.code} class="max-h-[560px] p-3" label="Redemptions">
            {#snippet row(r)}
              <div class="grid grid-cols-[150px_minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1.6fr)] items-center gap-3 rounded-[3px] bg-white/6 px-3 py-2">
                <span class="num text-xl whitespace-nowrap">{serverShort(r.at, offset)}</span>
                <span class="flex min-w-0 items-center gap-2">
                  <Icon icon={r.icon} rarity={r.rarity} ship size={36} name={r.name} />
                  <span class="flex min-w-0 flex-col leading-tight">
                    <a href="/commanders/{r.commander_id}" use:link class="truncate text-lg text-gold no-underline hover:text-white">{r.name}</a>
                    <span class="num text-sm text-soft">UID:{r.commander_id}</span>
                  </span>
                </span>
                <span class="truncate tracking-[0.06em]">{r.code}</span>
                <div class="flex flex-wrap gap-1.5">
                  {#each r.rewards as d (d.type + ':' + d.id)}
                    <Icon icon={d.icon} rarity={d.rarity} ship={shipFramed(d.type)} count={d.count} size={38} name={d.name} />
                  {:else}
                    <span class="text-sm text-dim">Code deleted</span>
                  {/each}
                </div>
              </div>
            {/snippet}
          </ContentVirtualList>
        {:else if data}
          <p class="m-0 py-6 text-center text-soft">No redemption</p>
        {/if}
      </Panel>
    </div>

    <Card title="New code" class="min-w-[340px] flex-[0_1_420px]">
      <div class="ip-strip">CODE</div>
      <div class="flex gap-2.5">
        <input class="ip-field min-w-0 flex-1 text-xl tracking-[0.06em] uppercase" aria-label="Code" bind:value={code} />
        <button type="button" class="ip-btn2 h-[46px] px-4 text-lg" onclick={generate}>Generate</button>
      </div>
      <div class="ip-strip">REWARDS</div>
      <ContentRewards bind:rewards />
      <div class="ip-strip">MAIL</div>
      <label class="ip-lab">Title
        <input class="ip-field" placeholder={code.toUpperCase() || 'The code'} bind:value={title} />
      </label>
      <label class="ip-lab">Text
        <textarea class="ip-field" rows="2" bind:value={content}></textarea>
      </label>
      <div class="ip-strip">DATES</div>
      <div class="grid grid-cols-2 gap-3.5">
        <label class="ip-lab min-w-0">Starts
          <input class="ip-field num w-full min-w-0 px-2 text-lg" type="datetime-local" bind:value={start} />
        </label>
        <label class="ip-lab min-w-0">Expires
          <input class="ip-field num w-full min-w-0 px-2 text-lg" type="datetime-local" bind:value={end} />
        </label>
      </div>
      <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1.7fr)] gap-3">
        <button type="button" class="ip-btn2" onclick={clear}>Clear</button>
        <button type="button" class="ip-btn" disabled={creating.isPending || !code.trim() || !rewards.length} onclick={create}>Create code</button>
      </div>
    </Card>
  </div>
</Page>
