<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Icon from '../../components/Icon.svelte'
  import Seg from '../../components/Seg.svelte'
  import ModerationTabs from '../../components/ModerationTabs.svelte'
  import { api } from '../../lib/api'
  import { createWrite, get } from '../../lib/query'
  import { link } from '../../lib/router.svelte'
  import { moderationKeys, type ReportedContent } from '../../lib/moderation'
  import { recent } from '../../lib/format'

  let kind = $state<'all' | 'comment' | 'gear'>('all')

  const reports = createQuery(() => ({ queryKey: moderationKeys.reports, queryFn: get<ReportedContent[]>('/moderation/reports') }))

  const kinds = [
    { value: 'all' as const, label: 'All' },
    { value: 'comment' as const, label: 'Ship comments' },
    { value: 'gear' as const, label: 'Gear codes' },
  ]
  const shown = $derived((reports.data ?? []).filter((item) => kind === 'all' || item.kind === kind))

  const key = (item: ReportedContent) => `${item.kind}${item.id}`

  const action = createWrite(
    ({ item, what }: { item: ReportedContent; what: 'dismiss' | 'delete' }) =>
      what === 'dismiss'
        ? api('POST', `/moderation/reports/${item.kind}/${item.id}/dismiss`)
        : api('DELETE', item.kind === 'comment' ? `/moderation/comments/${item.id}` : `/moderation/gear-codes/${item.id}`),
    [moderationKeys.all],
    { success: (_, { item, what }) => (what === 'dismiss' ? 'Reports dismissed' : item.kind === 'comment' ? 'Comment deleted' : 'Gear code deleted') },
  )
  const busy = (item: ReportedContent) => action.isPending && action.variables && key(action.variables.item) === key(item)

  function act(item: ReportedContent, what: 'dismiss' | 'delete') {
    const noun = item.kind === 'comment' ? 'comment' : 'gear code'
    if (what === 'delete' && !confirm(`Delete ${item.author}'s ${noun} on ${item.ship.name}?`)) return
    action.mutate({ item, what })
  }
</script>

<Page title="Reports" sub="MODERATION" back="/">
  {#snippet actions()}
    <Seg options={kinds} bind:value={kind} label="Content" />
  {/snippet}
  <div class="relative flex items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <ModerationTabs />
    <Panel title="Report queue" sub="CONTENT" class="flex min-w-0 flex-1 flex-col">
      {#snippet actions()}
        <span class="num text-2xl">{shown.length}</span><span class="text-sm text-soft">open · most reported first</span>
      {/snippet}
      <div class="grid grid-cols-[repeat(auto-fill,minmax(440px,1fr))] gap-3 p-3">
        {#each shown as item (key(item))}
          <article class="ip-row flex flex-col gap-3 p-4">
            <div class="flex items-center gap-3">
              <Icon icon={item.ship.icon} rarity={item.ship.rarity} ship size={56} name={item.ship.name} />
              <div class="flex min-w-0 flex-1 flex-col gap-1">
                <div class="flex items-center gap-2.5">
                  <span class={['ip-tag', item.kind === 'comment' ? 'bg-[#3d7ff0] text-white' : 'bg-[#f0a93a] text-[#3a2a08]']}>{item.kind === 'comment' ? 'SHIP COMMENT' : 'GEAR CODE'}</span>
                  <span class="truncate text-lg">{item.ship.name}</span>
                </div>
                <span class="flex items-center gap-2 text-sm text-soft">
                  <Icon icon={item.author_icon} rarity={item.author_rarity} ship size={26} name={item.author} />
                  <span>
                    By <a href="/commanders/{item.author_id}" use:link class="text-gold">{item.author}</a>
                    UID:<span class="num text-base">{item.author_id}</span>
                    <span class="num ml-1 text-base">{recent(item.posted_at)}</span>
                  </span>
                </span>
              </div>
              <span class="flex-none text-right text-sm text-bad"><span class="num text-3xl leading-none">{item.reports.length}</span> {item.reports.length === 1 ? 'reporter' : 'reporters'}</span>
            </div>
            <p class={['m-0 rounded-[3px] bg-black/35 px-3 py-2.5 text-[15px] leading-snug break-words', item.kind === 'gear' && 'font-mono font-normal']}>{item.content}</p>
            <ul class="m-0 flex list-none flex-col gap-1 p-0">
              {#each item.reports as report (report.commander_id)}
                <li class="flex items-center gap-2.5 rounded-[3px] bg-white/5 px-2.5 py-1.5 text-sm">
                  <Icon icon={report.icon} rarity={report.rarity} ship size={24} name={report.name} />
                  <a href="/commanders/{report.commander_id}" use:link class="flex-none text-gold">{report.name}</a>
                  <span class="min-w-0 flex-1 break-words">{report.reason}</span>
                  <span class="num flex-none text-base text-soft">{recent(report.at)}</span>
                </li>
              {/each}
            </ul>
            <div class="grid grid-cols-3 gap-2.5">
              <button type="button" class="ip-btn2 ip-sm" disabled={busy(item)} onclick={() => act(item, 'dismiss')}>Dismiss</button>
              <button type="button" class="ip-danger ip-sm" disabled={busy(item)} onclick={() => act(item, 'delete')}>{item.kind === 'comment' ? 'Delete comment' : 'Delete code'}</button>
              <a href="/commanders/{item.author_id}" use:link class="ip-btn ip-sm">Open commander</a>
            </div>
          </article>
        {:else}
          {#if reports.isSuccess}<p class="col-span-full m-0 px-2 py-6 text-center text-soft">No report</p>{/if}
        {/each}
        {#if reports.error}<p class="col-span-full m-0 px-2 py-6 text-center text-bad">{reports.error.message}</p>{/if}
      </div>
    </Panel>
  </div>
</Page>
