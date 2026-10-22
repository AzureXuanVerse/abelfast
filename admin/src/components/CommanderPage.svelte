<script lang="ts">
  // A commander screen: the commander's resources in the header, the
  // commander sidebar, and the screen's body. The summary is one query
  // (keys.summary) shared by every commander screen: any write to the
  // commander invalidates it, so the header follows grants.
  import type { Snippet } from 'svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import { api, type CommanderSummary } from '../lib/api'
  import { createCommanderMutation, get, keys, type Kicked } from '../lib/query'
  import { link, route } from '../lib/router.svelte'
  import { count } from '../lib/format'
  import Page from './Page.svelte'

  let {
    id,
    title,
    actions,
    children,
  }: {
    id: number
    title: string
    actions?: Snippet
    children: Snippet<[CommanderSummary]>
  } = $props()

  const query = createQuery(() => ({
    queryKey: keys.summary(id),
    queryFn: get<CommanderSummary>(`/commanders/${id}/summary`),
  }))
  const summary = $derived(query.data)

  const base = $derived(`/commanders/${id}`)
  // sub-screens that light up their parent entry
  const parents: Record<string, string> = { gear: 'items', wardrobe: 'dock' }
  const current = $derived.by(() => {
    const rest = route.path.slice(base.length).split('/')[1] ?? ''
    return parents[rest] ?? rest
  })
  const entries = $derived([
    { key: '', label: 'Profile' },
    { key: 'items', label: 'Items', count: summary?.counts.items },
    { key: 'dock', label: 'Dock', count: summary?.counts.ships },
    { key: 'fleets', label: 'Fleets' },
    { key: 'meowfficers', label: 'Meowfficers', count: summary?.counts.meowfficers },
    { key: 'timers', label: 'Timers' },
    { key: 'progress', label: 'Progress' },
    { key: 'research', label: 'Research' },
    { key: 'combat', label: 'Combat' },
    { key: 'backyard', label: 'Backyard' },
    { key: 'friends', label: 'Friends', count: summary?.counts.friends },
    { key: 'shops', label: 'Shops' },
    { key: 'activities', label: 'Activities' },
    { key: 'archives', label: 'Archives' },
    { key: 'mail', label: 'Mail', alert: (summary?.counts.unread_mail ?? 0) > 0 },
  ])

  const kick = createCommanderMutation(
    () => id,
    () => api<Kicked>('POST', `/commanders/${id}/kick`),
    { success: ({ kicked }) => (kicked ? `${summary?.name} was kicked` : `${summary?.name} isn't playing`) },
  )
</script>

{#snippet resources()}
  {#if summary}
    {#each [{ name: 'Oil', icon: 'props/oil', value: summary.oil }, { name: 'Coins', icon: 'props/gold', value: summary.gold }, { name: 'Gems', icon: 'props/gem', value: summary.gems, gold: true }] as res (res.name)}
      <div class="flex items-center">
        <img src="/api/assets/{res.icon}.png" alt={res.name} class="relative z-1 size-9 object-contain" />
        <span class={['num -ml-2 flex h-[34px] min-w-[130px] items-center justify-end bg-linear-to-r from-[rgb(44_48_58/0.35)] to-[rgb(44_48_58/0.85)] to-25% px-4 text-[30px]', res.gold && 'text-gold']}>{count(res.value)}</span>
      </div>
    {/each}
  {/if}
  {@render actions?.()}
{/snippet}

<Page {title} sub={summary ? `${summary.name} · Commander #${id}` : ''} back="/commanders" backLabel="Back to commanders" actions={resources}>
  <div class="relative flex items-stretch gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <nav aria-label="Commander" class="ip-side h-[min(880px,calc(100vh-32px))] w-[188px] pt-2.5 pb-4">
      {#each entries as entry (entry.key)}
        <a href={entry.key ? `${base}/${entry.key}` : base} use:link class="ip-side-link" aria-current={current === entry.key ? 'page' : undefined}>
          {entry.label}
          {#if entry.count !== undefined}<span class={['num ml-auto text-xl', current !== entry.key && 'text-[#aab2c2]']}>{entry.count}</span>{/if}
          {#if entry.alert}<span aria-label="Unread mail" class="ml-auto size-3 rotate-45 bg-[#d0634a] shadow-[0_0_0_2px_#f2c3a6]"></span>{/if}
        </a>
      {/each}
      <div class="mt-auto mr-4 ml-7 flex flex-col gap-2.5 pt-3.5">
        {#if summary}
          <div class="flex items-baseline gap-2.5">
            <span class="num text-[40px] leading-none">{summary.level}</span>
            <span class="ip-sub">Level</span>
          </div>
        {/if}
        <button type="button" class="ip-danger h-12" onclick={() => kick.mutate()} disabled={!summary?.online || kick.isPending}>Kick</button>
      </div>
    </nav>
    <div class="min-w-0 flex-1">
      {#if summary}
        {@render children(summary)}
      {:else if query.error}
        <p class="text-bad">{query.error.message}</p>
      {/if}
    </div>
  </div>
</Page>
