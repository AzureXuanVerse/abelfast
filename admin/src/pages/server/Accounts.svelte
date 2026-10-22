<script lang="ts">
  import { untrack } from 'svelte'
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import { createWindowVirtualizer } from '@tanstack/svelte-virtual'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import { api } from '../../lib/api'
  import { createWrite, get, invalidateCommander } from '../../lib/query'
  import { fuzzy } from '../../lib/fuzzy'
  import { link } from '../../lib/router.svelte'
  import { serverKeys, secondarySystems, type Account, type AccountDetail } from '../../lib/server'
  import { day, stamp } from '../../lib/format'
  import Page from '../../components/Page.svelte'
  import Icon from '../../components/Icon.svelte'
  import Seg from '../../components/Seg.svelte'

  let selected = $state<number | null>(null)
  let query = $state('')
  let show = $state<'all' | 'linked' | 'unlinked'>('all')
  let mailBox = $state('')
  let password = $state('')

  const list = createQuery(() => ({
    queryKey: serverKeys.accounts,
    queryFn: get<Account[]>('/server/accounts'),
  }))
  const accounts = $derived(list.data)
  const account = createQuery(() => ({
    queryKey: serverKeys.account(selected ?? 0),
    queryFn: get<AccountDetail>(`/server/accounts/${selected}`),
    enabled: selected !== null,
  }))
  const detail = $derived(selected !== null ? account.data : undefined)

  // the first account until one is picked, or when the picked one is gone
  $effect(() => {
    const rows = accounts
    if (!rows) return
    untrack(() => {
      if (selected === null || !rows.some((a) => a.account_id === selected)) selected = rows[0]?.account_id ?? null
    })
  })
  // the forms start from the account's values
  $effect(() => {
    const loaded = detail
    untrack(() => {
      mailBox = loaded?.mail_box ?? ''
      password = ''
    })
  })

  const [debounced] = createDebouncedValue(() => query, { wait: 150 })
  const shown = $derived(
    fuzzy(
      (accounts ?? []).filter((a) => (show === 'linked' ? a.commander : show === 'unlinked' ? !a.commander : true)),
      debounced(),
      (a) => [a.username, a.mail_box, a.account_id, a.commander?.name, a.commander?.id],
    ),
  )
  const linked = $derived(accounts?.filter((a) => a.commander).length ?? 0)

  // the rows, rendered as the page scrolls to them
  const rowHeight = 64
  const gap = 6
  let rowsElement = $state<HTMLElement>()
  let top = $state(0)
  $effect(() => {
    if (!rowsElement) return
    const element = rowsElement
    const measure = () => (top = element.getBoundingClientRect().top + scrollY)
    const observer = new ResizeObserver(measure)
    observer.observe(document.body)
    measure()
    return () => observer.disconnect()
  })
  const virtualizer = createWindowVirtualizer<HTMLDivElement>({
    count: 0,
    estimateSize: () => rowHeight + gap,
    overscan: 6,
  })
  $effect(() => {
    const count = shown.length
    const margin = top
    untrack(() => {
      $virtualizer.setOptions({ count, scrollMargin: margin })
      $virtualizer.measure()
    })
  })

  // a write to the selected account: it and the list refetch, and its
  // commander's screens when the write changes the commander
  const client = useQueryClient()
  interface Action {
    method: string
    path: string
    body?: unknown
    done: string
    commander?: number
  }
  const act = createWrite(
    (action: Action) => api(action.method, `/server/accounts/${selected}${action.path}`, action.body),
    [serverKeys.accounts],
    {
      success: (_, action) => action.done,
      onSuccess: (_, action) => {
        if (action.commander) invalidateCommander(client, action.commander, true)
      },
    },
  )

  function unlink() {
    if (!detail?.commander || !confirm(`Unlink ${detail.commander.name} from ${detail.username}? The account gets a new commander at its next login.`)) return
    act.mutate({ method: 'POST', path: '/unlink', done: 'Commander unlinked', commander: detail.commander.id })
  }
  function clearSecondary() {
    if (!detail?.commander || !confirm(`Clear ${detail.commander.name}'s secondary password?`)) return
    act.mutate({ method: 'POST', path: '/clear-secondary', done: 'Secondary password cleared', commander: detail.commander.id })
  }
  function forget(code: string) {
    act.mutate({ method: 'DELETE', path: `/codes/${encodeURIComponent(code)}`, done: `${code} forgotten`, commander: detail?.commander?.id })
  }
  function remove() {
    if (!detail || !confirm(`Delete the account ${detail.username}? Its commander stays in the database.`)) return
    const gone = detail.account_id
    act.mutate(
      { method: 'DELETE', path: '', done: `${detail.username} deleted`, commander: detail.commander?.id },
      {
        onSuccess: () => {
          client.removeQueries({ queryKey: serverKeys.account(gone) })
          selected = null
        },
      },
    )
  }
  const busy = $derived(act.isPending)

  const tags = { set: { label: '2ND PW', bg: 'bg-[#3d8bff]' }, locked: { label: 'LOCKED OUT', bg: 'bg-red' } }
</script>

{#snippet actions()}
  <label class="relative flex items-center">
    <span class="sr-only">Find an account</span>
    <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute left-3 size-5" fill="none" stroke="#6b7386" stroke-width="2.4"><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" stroke-linecap="round" /></svg>
    <input type="search" class="ip-field w-[300px] pl-10" placeholder="Username, mail or ID" bind:value={query} />
  </label>
  <Seg
    label="Show"
    bind:value={show}
    options={[
      { value: 'all', label: 'All' },
      { value: 'linked', label: 'With commander' },
      { value: 'unlinked', label: 'No commander' },
    ]}
  />
{/snippet}

{#snippet strip(label: string)}
  <div class="ip-strip">{label}</div>
{/snippet}

<Page title="Accounts" sub="LOGIN" back="/" {actions}>
  <div class="relative flex items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <section aria-labelledby="acc-h" class="ip-panel flex min-w-0 flex-[1_1_760px] flex-col">
      <div class="ip-head items-center">
        <h2 id="acc-h">Inner login</h2>
        <span class="ip-sub">ACCOUNTS</span>
        {#if accounts}
          <span class="ml-auto text-sm text-soft">
            <span class="num text-[22px] text-white">{accounts.length}</span> accounts ·
            <span class="num text-[22px] text-white">{linked}</span> with a commander
          </span>
        {/if}
      </div>
      <div aria-hidden="true" class="ip-sub grid grid-cols-[minmax(0,1.1fr)_150px_minmax(0,1.3fr)_130px] gap-3 px-[30px] pt-3">
        <span>USERNAME</span><span>ACCOUNT ID</span><span>COMMANDER</span><span>REGISTERED</span>
      </div>
      <div class="px-3 pt-2 pb-3.5">
        {#if accounts && shown.length === 0}
          <p class="m-0 px-[18px] py-4 text-base text-dim">No account</p>
        {/if}
        <ul bind:this={rowsElement} class="relative m-0 list-none p-0" style:height="{$virtualizer.getTotalSize()}px">
          {#each $virtualizer.getVirtualItems() as line (line.key)}
            {@const account = shown[line.index]}
            {#if account}
              <li class="absolute top-0 left-0 w-full" style:transform="translateY({line.start - $virtualizer.options.scrollMargin}px)">
                <button
                  type="button"
                  aria-pressed={selected === account.account_id}
                  onclick={() => (selected = account.account_id)}
                  class={['ip-row grid h-16 w-full cursor-pointer grid-cols-[minmax(0,1.1fr)_150px_minmax(0,1.3fr)_130px] items-center gap-3 border-0 px-[18px] py-2 text-left font-[inherit] text-white', selected === account.account_id && 'ip-sel']}
                >
                  <span class="flex min-w-0 flex-col gap-0.5">
                    <span class="truncate text-[19px]">{account.username}</span>
                    <span class="truncate text-[13px] text-dim">{account.mail_box || 'No mail box'}</span>
                  </span>
                  <span class="num text-[20px] text-soft">{account.account_id}</span>
                  <span class="flex min-w-0 items-center gap-2.5">
                    {#if account.commander}
                      <Icon icon={account.commander.icon} rarity={account.commander.rarity} ship size={40} name={account.commander.name} />
                      <span class="truncate text-[17px] text-gold">{account.commander.name}</span>
                      <span class="text-[13px] whitespace-nowrap text-soft">UID:<span class="num text-base">{account.commander.id}</span></span>
                      {#if account.secondary}
                        <span class={['ip-tag text-white', tags[account.secondary].bg]}>{tags[account.secondary].label}</span>
                      {/if}
                      {#if account.commander.online}<span class="ip-tag bg-[#2f9e5c] text-white">ONLINE</span>{/if}
                    {:else}
                      <span class="text-[15px] text-dim">No commander yet</span>
                    {/if}
                  </span>
                  <span class="num text-[19px] text-soft">{day(account.registered_at)}</span>
                </button>
              </li>
            {/if}
          {/each}
        </ul>
      </div>
    </section>

    {#if detail}
      <div class="flex min-w-[360px] flex-[0_1_420px] flex-col gap-[18px]">
        <aside aria-labelledby="sel-h" class="ip-card flex flex-col">
          <div class="ip-card-head"><h2 id="sel-h">{detail.username}</h2></div>
          <div class="ip-card-body">
            <dl class="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[15px]">
              <dt class="text-[#6b7386]">Account ID</dt>
              <dd class="num m-0 text-[20px]">{detail.account_id}</dd>
              <dt class="text-[#6b7386]">Registered</dt>
              <dd class="num m-0 text-[20px]">{stamp(detail.registered_at)}</dd>
            </dl>

            {@render strip('Commander')}
            {#if detail.commander}
              <div class="flex items-center gap-3 rounded bg-white p-2.5 shadow-[inset_0_0_0_1px_#dfe3ea]">
                <Icon icon={detail.commander.icon} rarity={detail.commander.rarity} ship size={52} name={detail.commander.name} />
                <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span class="truncate text-[19px]">{detail.commander.name}</span>
                  <span class="text-[13px] text-[#5a6276]">
                    UID:<span class="num text-base">{detail.commander.id}</span> · Lv.<span class="num text-base">{detail.commander.level}</span>
                    {#if detail.commander.online}· <span class="text-[#2f9e5c]">Online</span>{/if}
                  </span>
                </span>
                <a href="/commanders/{detail.commander.id}" use:link class="ip-btn ip-sm h-10 px-4 text-base">Open</a>
              </div>
              <button type="button" class="ip-btn2 h-11 text-lg" disabled={busy} onclick={unlink}>Unlink commander</button>
            {:else}
              <p class="m-0 text-[15px] text-[#6b7386]">No commander yet</p>
            {/if}

            {@render strip('Mail box')}
            <form class="flex gap-2.5" onsubmit={(event) => { event.preventDefault(); act.mutate({ method: 'POST', path: '/mail-box', body: { mail_box: mailBox.trim() }, done: 'Mail box changed' }) }}>
              <label for="mail" class="sr-only">Mail box</label>
              <input id="mail" class="ip-field min-w-0 flex-1" bind:value={mailBox} maxlength="254" />
              <button type="submit" class="ip-btn h-[46px] text-lg" disabled={busy || mailBox.trim() === detail.mail_box}>Change</button>
            </form>

            {@render strip('Password')}
            <form class="flex gap-2.5" onsubmit={(event) => { event.preventDefault(); act.mutate({ method: 'POST', path: '/password', body: { password }, done: 'Password reset' }, { onSuccess: () => (password = '') }) }}>
              <label for="pw" class="sr-only">New password</label>
              <input id="pw" type="password" class="ip-field min-w-0 flex-1" placeholder="New password" autocomplete="new-password" maxlength="128" bind:value={password} />
              <button type="submit" class="ip-btn h-[46px] text-lg" disabled={busy || !password}>Reset</button>
            </form>

            {#if detail.commander}
              {@render strip('Secondary password')}
              {@const secondary = detail.secondary_password}
              <dl class="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[15px]">
                <dt class="text-[#6b7386]">State</dt>
                <dd class={['m-0', secondary?.state === 'locked' && 'text-red']}>
                  {#if !secondary}None{:else if secondary.state === 'locked'}Locked out until <span class="num text-lg">{stamp(secondary.fail_cd)}</span>{:else}Set{/if}
                </dd>
                {#if secondary}
                  <dt class="text-[#6b7386]">Wrong tries</dt>
                  <dd class="num m-0 text-[20px]">{secondary.fail_count}</dd>
                  <dt class="text-[#6b7386]">Protects</dt>
                  <dd class="m-0">{secondary.systems.map((id) => secondarySystems[id] ?? `#${id}`).join(', ') || 'Nothing'}</dd>
                {/if}
              </dl>
              {#if secondary}
                <button type="button" class="ip-btn2 h-11 text-lg" disabled={busy} onclick={clearSecondary}>Clear secondary password</button>
              {/if}

              {@render strip('Redeem codes used')}
              {#if detail.codes.length}
                <ul class="m-0 flex list-none flex-col gap-1.5 p-0">
                  {#each detail.codes as code (code.code)}
                    <li class="flex items-center gap-2.5 rounded bg-white px-3 py-2 shadow-[inset_0_0_0_1px_#dfe3ea]">
                      <span class="min-w-0 flex-1 truncate text-[15px] tracking-[0.14em]">{code.code}</span>
                      <span class="num text-lg text-[#5a6276]">{day(code.at)}</span>
                      <button type="button" aria-label="Forget {code.code} for this commander" disabled={busy} onclick={() => forget(code.code)} class="grid size-7 cursor-pointer place-items-center rounded-full border-0 bg-[#6b7386] p-0 text-[15px] leading-none text-white hover:bg-[#5a6276] disabled:opacity-50">×</button>
                    </li>
                  {/each}
                </ul>
              {:else}
                <p class="m-0 text-[15px] text-[#6b7386]">None</p>
              {/if}
            {/if}
          </div>
        </aside>

        <div role="group" aria-label="Delete {detail.username}" class="ip-panel ip-danger-zone flex flex-col px-5 pt-4 pb-5">
          <button type="button" class="ip-danger" disabled={busy} onclick={remove}>Delete account</button>
        </div>
      </div>
    {/if}
  </div>
</Page>
