<script lang="ts">
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Icon from '../../components/Icon.svelte'
  import Seg from '../../components/Seg.svelte'
  import ModerationTabs from '../../components/ModerationTabs.svelte'
  import ContentVirtualList from '../../components/ContentVirtualList.svelte'
  import { api } from '../../lib/api'
  import { fuzzy } from '../../lib/fuzzy'
  import { createWrite, get, invalidateCommander, type Kicked } from '../../lib/query'
  import { link } from '../../lib/router.svelte'
  import { count, recent, stamp } from '../../lib/format'
  import { moderationKeys, type Shared, type SharedCode, type SharedComment, type SharedTheme } from '../../lib/moderation'

  let query = $state('')
  let section = $state<'all' | 'comments' | 'codes' | 'themes'>('all')
  let order = $state<'reported' | 'newest'>('reported')
  let selected = $state<number | null>(null)

  const shared = createQuery(() => ({ queryKey: moderationKeys.shared, queryFn: get<Shared>('/moderation/shared') }))
  const [search] = createDebouncedValue(() => query, { wait: 150 })

  const sections = [
    { value: 'all' as const, label: 'All' },
    { value: 'comments' as const, label: 'Ship comments' },
    { value: 'codes' as const, label: 'Gear codes' },
    { value: 'themes' as const, label: 'Themes' },
  ]
  const orders = [
    { value: 'reported' as const, label: 'Most reported' },
    { value: 'newest' as const, label: 'Newest' },
  ]

  // a search ranks best match first, the order applies without one
  const comments = $derived(
    fuzzy(
      (shared.data?.comments ?? []).slice().sort((a, b) => (order === 'reported' ? b.reports.length - a.reports.length : 0) || b.posted_at - a.posted_at),
      search(),
      (c) => [c.ship.name, c.author, c.content],
    ),
  )
  const codes = $derived(fuzzy(shared.data?.codes ?? [], search(), (c) => [c.ship.name, c.author, c.code]))
  const themes = $derived(fuzzy(shared.data?.themes ?? [], search(), (t) => [t.name, t.author]))
  const current = $derived(comments.find((c) => c.id === selected) ?? comments[0] ?? null)
  const show = (name: typeof section) => section === 'all' || section === name

  const remove = createWrite(
    (what: { comment: SharedComment } | { code: SharedCode } | { dismiss: SharedComment }) =>
      'comment' in what
        ? api('DELETE', `/moderation/comments/${what.comment.id}`)
        : 'code' in what
          ? api('DELETE', `/moderation/gear-codes/${what.code.id}`)
          : api('POST', `/moderation/reports/comment/${what.dismiss.id}/dismiss`),
    [moderationKeys.all],
    { success: (_, what) => ('comment' in what ? 'Comment deleted' : 'code' in what ? 'Gear code deleted' : 'Reports dismissed') },
  )

  // unsharing kicks the author when playing: its screens refetch too
  const client = useQueryClient()
  const unsharing = createWrite((theme: SharedTheme) => api<Kicked>('POST', `/moderation/themes/${theme.author_id}/${theme.pos}/unshare`), [moderationKeys.all], {
    success: 'Theme unshared',
    onSuccess: (answer, theme) => invalidateCommander(client, theme.author_id, answer.kicked),
  })
  const busy = $derived(remove.isPending || unsharing.isPending)

  function deleteComment(comment: SharedComment) {
    if (!confirm(`Delete ${comment.author}'s comment on ${comment.ship.name}?`)) return
    remove.mutate({ comment })
  }
  function dismissComment(comment: SharedComment) {
    remove.mutate({ dismiss: comment })
  }
  function deleteCode(code: SharedCode) {
    if (!confirm(`Delete ${code.author}'s gear code for ${code.ship.name}?`)) return
    remove.mutate({ code })
  }
  function unshare(theme: SharedTheme) {
    if (!confirm(`Take "${theme.name}" out of the theme shop?`)) return
    unsharing.mutate(theme)
  }
</script>

<Page title="Shared" sub="PLAYER CONTENT" back="/">
  {#snippet actions()}
    <label class="relative flex items-center">
      <span class="sr-only">Search shared content</span>
      <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
      <input type="search" class="ip-field w-[200px] pl-10" placeholder="Ship or author" bind:value={query} />
    </label>
    <Seg options={sections} bind:value={section} label="Content" />
  {/snippet}
  <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <ModerationTabs />
    <div class="flex min-w-0 flex-[1_1_600px] flex-col gap-5">
      {#if show('comments')}
        <Panel title="Ship comments" sub="EVALUATION" class="flex flex-col">
          {#snippet actions()}
            <span class="num mr-auto text-2xl">{count(comments.length)}</span>
            <Seg options={orders} bind:value={order} label="Order" />
          {/snippet}
          {#if comments.length}
            <ContentVirtualList items={comments} estimate={76} gap={8} key={(c) => c.id} class="max-h-[560px] p-3" label="Ship comments">
              {#snippet row(comment)}
                <div class={['ip-row flex items-center gap-3 py-2 pr-3 pl-2', current?.id === comment.id && 'ip-sel']}>
                  <button type="button" class="flex min-w-0 flex-1 cursor-pointer items-center gap-3 border-0 bg-transparent p-0 text-left font-game font-bold text-white" onclick={() => (selected = comment.id)}>
                    <Icon icon={comment.ship.icon} rarity={comment.ship.rarity} ship size={52} name={comment.ship.name} />
                    <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span class="flex flex-wrap items-center gap-x-2">
                        <span class="text-lg">{comment.ship.name}</span>
                        <span class="flex items-center gap-1.5 text-sm text-soft">by <Icon icon={comment.author_icon} rarity={comment.author_rarity} ship size={22} name={comment.author} /><span class="text-gold">{comment.author}</span></span>
                        <span class="num text-base text-soft">{recent(comment.posted_at)}</span>
                      </span>
                      <span class="line-clamp-2 text-[15px] leading-snug break-words">{comment.content}</span>
                    </span>
                  </button>
                  <span class="flex flex-none items-center gap-3 text-sm">
                    <span class="text-ok" title="Good"><span aria-hidden="true">▲</span> <span class="num text-lg">{comment.good}</span></span>
                    <span class="text-bad" title="Bad"><span aria-hidden="true">▼</span> <span class="num text-lg">{comment.bad}</span></span>
                    {#if comment.reports.length}<span class="ip-tag bg-[#ef4b42] text-white">REPORTED ×{comment.reports.length}</span>{/if}
                  </span>
                  <button type="button" class="ip-danger ip-sm flex-none" disabled={busy} onclick={() => deleteComment(comment)}>Delete</button>
                </div>
              {/snippet}
            </ContentVirtualList>
          {:else if shared.isSuccess}
            <p class="m-0 px-2 py-6 text-center text-soft">No comment</p>
          {/if}
        </Panel>
      {/if}

      {#if show('codes')}
        <Panel title="Gear codes" sub="EQCODE" class="flex flex-col">
          {#snippet actions()}<span class="num text-2xl">{count(codes.length)}</span>{/snippet}
          {#if codes.length}
            <ContentVirtualList items={codes} estimate={64} gap={8} key={(c) => c.id} class="max-h-[480px] p-3" label="Gear codes">
              {#snippet row(code)}
                <div class="ip-row flex items-center gap-3 py-2 pr-3 pl-2">
                  <Icon icon={code.ship.icon} rarity={code.ship.rarity} ship size={44} name={code.ship.name} />
                  <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span class="flex flex-wrap items-center gap-x-1.5 text-[15px]">{code.ship.name} <span class="text-sm text-soft">by</span> <Icon icon={code.author_icon} rarity={code.author_rarity} ship size={22} name={code.author} /><a href="/commanders/{code.author_id}" use:link class="text-gold">{code.author}</a></span>
                    <span class="truncate font-mono text-[13px] font-normal text-[#dfe5f0]" title={code.code}>{code.code}</span>
                  </span>
                  <span class="flex flex-none flex-col items-end gap-1">
                    <span class="text-sm"><span class="num text-lg">{code.likes}</span> likes</span>
                    {#if code.nonsense}<span class="ip-tag bg-[#f0a93a] text-[#3a2a08]">NONSENSE ×{code.nonsense}</span>{/if}
                    {#if code.outdated}<span class="ip-tag bg-[#f0a93a] text-[#3a2a08]">OUTDATED ×{code.outdated}</span>{/if}
                  </span>
                  <button type="button" class="ip-danger ip-sm flex-none" disabled={busy} onclick={() => deleteCode(code)}>Delete</button>
                </div>
              {/snippet}
            </ContentVirtualList>
          {:else if shared.isSuccess}
            <p class="m-0 px-2 py-6 text-center text-soft">No gear code</p>
          {/if}
        </Panel>
      {/if}

      {#if show('themes')}
        <Panel title="Backyard themes" sub="SHARED" class="flex flex-col">
          {#snippet actions()}<span class="num text-2xl">{count(themes.length)}</span>{/snippet}
          {#if themes.length}
            <ContentVirtualList items={themes} estimate={60} gap={8} key={(t) => `${t.author_id}-${t.pos}`} class="max-h-[480px] p-3" label="Backyard themes">
              {#snippet row(theme)}
                <div class="ip-row flex items-center gap-3 py-2 pr-3 pl-2">
                  <span class="grid size-11 flex-none place-items-center rounded bg-navy">
                    <svg aria-hidden="true" viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><path d="M4 11l8-7 8 7v9h-5v-6H9v6H4z" /></svg>
                  </span>
                  <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span class="truncate text-[17px]">{theme.name || `Template ${theme.pos}`}</span>
                    <span class="flex items-center gap-1.5 text-sm text-soft">by <Icon icon={theme.author_icon} rarity={theme.author_rarity} ship size={22} name={theme.author} /><a href="/commanders/{theme.author_id}" use:link class="text-gold">{theme.author}</a> · <span class="num text-base">{recent(theme.shared_at)}</span></span>
                  </span>
                  <span class="flex flex-none flex-col items-end text-sm leading-tight">
                    <span><span class="num text-lg">{theme.likes}</span> likes</span>
                    <span><span class="num text-lg">{theme.favs}</span> saved</span>
                  </span>
                  <button type="button" class="ip-btn2 ip-sm flex-none" disabled={busy} onclick={() => unshare(theme)}>Unshare</button>
                </div>
              {/snippet}
            </ContentVirtualList>
          {:else if shared.isSuccess}
            <p class="m-0 px-2 py-6 text-center text-soft">No shared theme</p>
          {/if}
        </Panel>
      {/if}
    </div>

    {#if current && show('comments')}
      <aside aria-label="Comment on {current.ship.name}" class="ip-panel flex min-w-[360px] flex-[0_1_380px] flex-col overflow-hidden">
        <div class="ip-head items-center">
          <Icon icon={current.ship.icon} rarity={current.ship.rarity} ship size={40} name={current.ship.name} />
          <h2>{current.ship.name}</h2>
          <span class="ip-sub">COMMENT</span>
        </div>
        <div class="flex flex-col gap-3 p-4">
          <span class="flex flex-wrap items-center gap-1.5 text-sm text-soft">
            By <Icon icon={current.author_icon} rarity={current.author_rarity} ship size={26} name={current.author} /><a href="/commanders/{current.author_id}" use:link class="text-gold">{current.author}</a> · UID:<span class="num text-base">{current.author_id}</span> ·
            <span class="num text-base">{stamp(current.posted_at)}</span>
          </span>
          <p class="m-0 rounded-[3px] bg-black/35 px-3 py-2.5 text-base leading-snug break-words">{current.content}</p>
          <div class="flex gap-5 text-sm">
            <span><span class="num text-2xl text-ok">{current.good}</span> good</span>
            <span><span class="num text-2xl text-bad">{current.bad}</span> bad</span>
            <span><span class="num text-2xl">{current.reports.length}</span> reports</span>
          </div>
          {#if current.reports.length}
            <div class="ip-strip">REPORTS</div>
            <ul class="m-0 flex list-none flex-col gap-1 p-0">
              {#each current.reports as report (report.commander_id)}
                <li class="flex items-center gap-2.5 rounded-[3px] bg-white/5 px-2.5 py-1.5 text-sm">
                  <Icon icon={report.icon} rarity={report.rarity} ship size={24} name={report.name} />
                  <a href="/commanders/{report.commander_id}" use:link class="flex-none text-gold">{report.name}</a>
                  <span class="min-w-0 flex-1 break-words">{report.reason}</span>
                  <span class="num flex-none text-base text-soft">{recent(report.at)}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
        <div class="flex flex-col gap-2.5 border-t border-white/10 p-4">
          <button type="button" class="ip-danger" disabled={busy} onclick={() => current && deleteComment(current)}>Delete comment</button>
          <a href="/commanders/{current.author_id}" use:link class="ip-btn">Open commander</a>
          {#if current.reports.length}
            <button type="button" class="ip-btn2" disabled={busy} onclick={() => current && dismissComment(current)}>Dismiss reports</button>
          {/if}
        </div>
      </aside>
    {/if}
  </div>
</Page>
