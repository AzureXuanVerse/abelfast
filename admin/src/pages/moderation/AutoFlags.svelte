<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Icon from '../../components/Icon.svelte'
  import Seg from '../../components/Seg.svelte'
  import ModerationTabs from '../../components/ModerationTabs.svelte'
  import ContentVirtualList from '../../components/ContentVirtualList.svelte'
  import { api } from '../../lib/api'
  import { fuzzy } from '../../lib/fuzzy'
  import { createWrite, get } from '../../lib/query'
  import { link } from '../../lib/router.svelte'
  import { moderationKeys, type FlaggedCommander, type Flags } from '../../lib/moderation'
  import { recent, stamp } from '../../lib/format'

  let query = $state('')
  let kind = $state<'all' | 'battle' | 'client'>('all')
  let selected = $state<number | null>(null)
  let showAll = $state(false)

  const flags = createQuery(() => ({ queryKey: moderationKeys.flags, queryFn: get<Flags>('/moderation/flags') }))
  const [search] = createDebouncedValue(() => query, { wait: 150 })

  const kinds = [
    { value: 'all' as const, label: 'All' },
    { value: 'battle' as const, label: 'Battle checks' },
    { value: 'client' as const, label: 'Client switches' },
  ]
  const shortList = 7

  const shown = $derived(
    fuzzy(
      (flags.data?.commanders ?? []).filter((c) => kind === 'all' || c.tallies.some((t) => t.kind === kind)),
      search(),
      (c) => [c.name, c.id, c.account_id],
    ),
  )
  const current = $derived(shown.find((c) => c.id === selected) ?? shown[0] ?? null)
  const marks = $derived(current ? (showAll ? current.marks : current.marks.slice(0, shortList)) : [])

  function pick(commander: FlaggedCommander) {
    selected = commander.id
    showAll = false
  }

  const dismissal = createWrite((commander: FlaggedCommander) => api('DELETE', `/moderation/flags/${commander.id}`), [moderationKeys.all], {
    success: 'Marks dismissed',
    onSuccess: () => (selected = null),
  })

  function dismiss(commander: FlaggedCommander) {
    if (!confirm(`Dismiss the ${commander.marks.length} marks of ${commander.name}?`)) return
    dismissal.mutate(commander)
  }

  const tagColor = (kind: string) => (kind === 'client' ? 'bg-[#8a93a6] text-white' : 'bg-[#f0a93a] text-[#3a2a08]')
  const reasonColor = (reason: number) => (reason >= 900 ? 'bg-[#ef4b42] text-white' : reason === 99 || reason === 100 ? 'bg-[#8a93a6] text-white' : 'bg-[#f0a93a] text-[#3a2a08]')
  const stats = $derived([
    { label: 'MARKS TODAY', value: flags.data?.today ?? 0, color: 'border-[#ef4b42]' },
    { label: 'THIS WEEK', value: flags.data?.week ?? 0, color: 'border-[#f0a93a]' },
    { label: 'COMMANDERS FLAGGED', value: flags.data?.commanders.length ?? 0, color: 'border-cyan' },
  ])
</script>

<Page title="Auto-flags" sub="CHEATER MARKS" back="/">
  {#snippet actions()}
    <label class="relative flex items-center">
      <span class="sr-only">Find a flagged commander</span>
      <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
      <input type="search" class="ip-field w-60 pl-10" placeholder="Name or UID" bind:value={query} />
    </label>
    <Seg options={kinds} bind:value={kind} label="Reason" />
  {/snippet}
  <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <ModerationTabs />
    <div class="flex min-w-0 flex-[1_1_560px] flex-col gap-5">
      <div class="grid grid-cols-3 gap-3.5">
        {#each stats as stat (stat.label)}
          <div class={['ip-panel flex flex-col gap-1 border-t-[3px] px-[18px] py-3', stat.color]}>
            <span class="ip-sub">{stat.label}</span>
            <span class="num text-[40px] leading-none">{stat.value}</span>
          </div>
        {/each}
      </div>
      <Panel title="Flagged commanders" sub="CS_10994" class="flex flex-col">
        {#snippet actions()}<span class="text-sm text-soft">Most marks first</span>{/snippet}
        <div class="flex flex-col gap-2 p-3">
          {#each shown as commander (commander.id)}
            <button type="button" class={['ip-row flex cursor-pointer items-center gap-4 border-0 py-2.5 pr-[18px] pl-4 text-left font-game font-bold text-white', current?.id === commander.id && 'ip-sel']} onclick={() => pick(commander)}>
              <Icon icon={commander.icon} rarity={commander.rarity} ship size={56} name={commander.name} />
              <span class="flex min-w-0 flex-1 flex-col gap-1.5">
                <span class="flex flex-wrap items-baseline gap-x-2.5">
                  <span class="text-[21px] text-gold">{commander.name}</span>
                  <span class="text-sm text-soft">UID:<span class="num text-lg">{commander.id}</span></span>
                  <span class="text-[13px] text-dim">Account <span class="num text-[17px] text-[#dfe5f0]">{commander.account_id}</span></span>
                  {#if commander.online}<span class="text-[13px] text-ok">Online</span>{/if}
                </span>
                <span class="flex flex-wrap gap-1.5">
                  {#each commander.tallies as tally (tally.label)}
                    <span class={['ip-tag', tagColor(tally.kind)]}>{tally.label.toUpperCase()} ×{tally.count}</span>
                  {/each}
                </span>
              </span>
              <span class="flex w-[150px] flex-none flex-col items-end gap-1 leading-none">
                <span class={['num text-[38px]', commander.marks.length >= 10 ? 'text-bad' : 'text-white']}>{commander.marks.length}</span>
                <span class="text-xs text-soft">last <span class="num text-base text-white">{recent(commander.last)}</span></span>
              </span>
            </button>
          {:else}
            {#if flags.isSuccess}<p class="m-0 px-2 py-6 text-center text-soft">No flagged commander</p>{/if}
          {/each}
          {#if flags.error}<p class="m-0 px-2 py-6 text-center text-bad">{flags.error.message}</p>{/if}
        </div>
      </Panel>
    </div>
    {#if current}
      <aside aria-label="Marks of {current.name}" class="ip-panel flex min-w-[400px] flex-[0_1_430px] flex-col overflow-hidden">
        <div class="ip-head items-center">
          <Icon icon={current.icon} rarity={current.rarity} ship size={40} name={current.name} />
          <h2 class="text-gold">{current.name}</h2>
          <span class="text-sm text-soft">UID:<span class="num text-lg">{current.id}</span></span>
          <span class="ml-auto text-sm"><span class="num text-3xl">{current.marks.length}</span> marks</span>
        </div>
        <dl class="m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 px-5 pt-4 pb-2">
          <dt class="text-soft">Account</dt>
          <dd class="num m-0 text-lg">{current.account_id}</dd>
          <dt class="text-soft">First mark</dt>
          <dd class="num m-0 text-lg">{stamp(current.first)}</dd>
          <dt class="text-soft">Last mark</dt>
          <dd class="num m-0 text-lg">{stamp(current.last)}</dd>
        </dl>
        <ContentVirtualList items={marks} estimate={40} gap={6} key={(mark) => mark.id} class="max-h-[420px] px-3 pb-2" label="Marks">
          {#snippet row(mark)}
            <div class="ip-row flex items-center gap-3 px-3 py-2 text-[15px]">
              <span class="num w-[92px] flex-none text-lg text-soft">{recent(mark.at)}</span>
              <span class={['num w-12 flex-none rounded-[3px] text-center text-sm', reasonColor(mark.reason)]}>{mark.reason}</span>
              <span class="min-w-0 flex-1">{mark.detail}</span>
            </div>
          {/snippet}
        </ContentVirtualList>
        {#if current.marks.length > shortList && !showAll}
          <button type="button" class="mx-4 mb-2 cursor-pointer self-start border-0 bg-transparent p-0 font-game text-[15px] font-bold text-link underline" onclick={() => (showAll = true)}>Show the {current.marks.length - shortList} older marks</button>
        {/if}
        <div class="grid grid-cols-2 gap-2.5 border-t border-white/10 p-4">
          <a href="/commanders/{current.id}" use:link class="ip-btn px-3 text-xl">Open commander</a>
          <button type="button" class="ip-btn2 px-3 text-xl" disabled={dismissal.isPending} onclick={() => current && dismiss(current)}>Dismiss</button>
        </div>
      </aside>
    {/if}
  </div>
</Page>
