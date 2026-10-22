<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import { api } from '../lib/api'
  import { createWrite, get, keys } from '../lib/query'
  import { link } from '../lib/router.svelte'
  import { ago, clock, count, pad, size, stamp } from '../lib/format'
  import { createNow } from '../lib/clock.svelte'
  import { overviewKeys, type Overview } from '../lib/overview'
  import Page from '../components/Page.svelte'
  import Panel from '../components/Panel.svelte'

  const overview = createQuery(() => ({
    queryKey: overviewKeys.overview,
    queryFn: get<Overview>('/overview'),
    // every 30 s, and right after the daily reset
    refetchInterval: (query) => {
      const reset = query.state.data?.next_reset
      if (!reset) return 30_000
      return Math.min(30_000, Math.max(1_000, reset * 1000 - Date.now() + 1_000))
    },
  }))
  const session = createQuery(() => ({
    queryKey: overviewKeys.session,
    queryFn: get<{ password: boolean }>('/session'),
    staleTime: Infinity,
  }))
  const data = $derived(overview.data)
  const password = $derived(session.data?.password ?? false)

  // the countdown and uptime clock
  const ticker = createNow(1000)
  const now = $derived(ticker.now)

  const countdown = $derived.by(() => {
    if (!data) return ''
    const left = Math.floor(Math.max(0, data.next_reset - now))
    return `${pad(Math.floor(left / 3600))}:${pad(Math.floor((left % 3600) / 60))}:${pad(left % 60)}`
  })
  const uptime = $derived.by(() => {
    if (!data) return { d: 0, h: 0, m: 0 }
    const up = Math.max(0, now - data.server.started_at)
    return { d: Math.floor(up / 86400), h: Math.floor((up % 86400) / 3600), m: Math.floor((up % 3600) / 60) }
  })


  const tags = {
    new: { label: 'NEW', bg: 'bg-cyan' },
    login: { label: 'LOGIN', bg: 'bg-[#2f9e5c]' },
    logout: { label: 'LOGOUT', bg: 'bg-[#6b7386]' },
    mail: { label: 'MAIL', bg: 'bg-[#3d8bff]' },
  }

  const kickAll = createWrite(() => api<{ kicked: number }>('POST', '/overview/kick-all'), [overviewKeys.overview, keys.online, keys.commanders, ['commander']], {
    success: ({ kicked }) => (kicked === 1 ? '1 commander kicked' : `${kicked} commanders kicked`),
  })
  function kickEveryone() {
    if (!data || !confirm(data.counts.online === 1 ? 'Kick the commander playing?' : `Kick the ${data.counts.online} commanders playing?`)) return
    kickAll.mutate()
  }

  const signOut = createWrite(() => api('POST', '/logout'), [], { onSuccess: () => location.reload() })

</script>

{#snippet badge()}
  <span class="text-4xl text-sel">B</span>
{/snippet}

{#snippet actions()}
  {#if data}
    <div class="flex flex-col items-end leading-none">
      <span class="ip-sub text-soft">NEXT DAILY RESET 00:00</span>
      <span class="num text-[34px] text-gold">{countdown}</span>
    </div>
    <div class="flex h-[52px] items-center rounded-[26px] bg-[rgb(30_33_40/0.88)] px-1.5">
      <span class="px-3.5 text-base whitespace-nowrap">
        Up
        {#if uptime.d}<span class="num text-2xl">{uptime.d}</span> d{/if}
        <span class="num text-2xl">{uptime.h}</span> h
        {#if !uptime.d}<span class="num text-2xl">{uptime.m}</span> min{/if}
      </span>
      <a href="/logs" use:link aria-label="Logs" class="grid size-11 place-items-center rounded-full text-white hover:text-white">
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[26px]" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"><rect x="4" y="3" width="16" height="18" rx="2" /><path d="M8 8h8M8 12h8M8 16h5" /></svg>
      </a>
      <a href="/settings" use:link aria-label="Settings" class="grid size-11 place-items-center rounded-full text-white hover:text-white">
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[26px]" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><circle cx="12" cy="12" r="3.2" /><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1" /></svg>
      </a>
      {#if password}
        <button type="button" disabled={signOut.isPending} onclick={() => signOut.mutate()} class="ml-1 h-10 cursor-pointer rounded-[20px] border-0 bg-white px-[18px] text-base text-ink">Sign out</button>
      {/if}
    </div>
  {/if}
{/snippet}

{#snippet light(color: string, label: string)}
  <span role="img" aria-label={label} class={['h-[30px] w-2 flex-none rounded-sm', color]}></span>
{/snippet}

<Page title="Belfast" sub="BACK-OFFICE" back={null} {badge} {actions}>
  <div class="flex min-h-[calc(100vh-100px)] flex-col">
    {#if data}
      {@const s = data.server}
      <main class="flex flex-1 flex-wrap items-start gap-5 pt-[18px] pr-4 pb-6 pl-4 sm:pr-10 sm:pl-16">
        <Panel title="Server" sub="STATUS" class="flex w-full flex-none flex-col sm:w-[330px]">
          <dl class="m-0 flex flex-col px-5 pt-1.5 pb-4">
            <div class="flex items-center gap-3 border-b border-white/8 py-[11px]">
              {@render light('bg-ok', 'Listening')}
              <div class="flex min-w-0 flex-1 flex-col gap-[3px]">
                <dt class="text-sm text-soft">Game server</dt>
                <dd class="num m-0 truncate text-[25px] leading-none">{s.address}</dd>
                <dd class="m-0 text-[13px] text-dim">Region {s.region}</dd>
              </div>
            </div>
            <div class="flex items-center gap-3 border-b border-white/8 py-[11px]">
              {@render light('bg-ok', 'Running')}
              <div class="flex min-w-0 flex-1 flex-col gap-[3px]">
                <dt class="text-sm text-soft">Uptime</dt>
                <dd class="num m-0 text-[25px] leading-none">{uptime.d} d {uptime.h} h {uptime.m} min</dd>
                <dd class="m-0 text-[13px] text-dim">Started {stamp(s.started_at)}</dd>
              </div>
            </div>
            <div class="flex items-center gap-3 border-b border-white/8 py-[11px]">
              {@render light(s.client_version ? 'bg-ok' : 'bg-warn', s.client_version ? 'Known' : 'Unknown')}
              <div class="flex min-w-0 flex-1 flex-col gap-[3px]">
                <dt class="text-sm text-soft">Client version</dt>
                <dd class="num m-0 text-[25px] leading-none">{s.client_version || '-'}</dd>
              </div>
            </div>
            <div class="flex items-center gap-3 border-b border-white/8 py-[11px]">
              {@render light(s.assets_at ? 'bg-ok' : 'bg-bad', s.assets_at ? 'Relayed' : 'Failed')}
              <div class="flex min-w-0 flex-1 flex-col gap-[3px]">
                <dt class="text-sm text-soft">Asset hashes</dt>
                {#if s.assets_at}
                  <dd class="num m-0 text-[25px] leading-none">{ago(s.assets_at, now)}</dd>
                  <dd class="num m-0 truncate text-[15px] text-dim" title={s.asset_hash}>{s.asset_hash}</dd>
                {:else}
                  <dd class="m-0 text-[15px] text-bad">{s.assets_error}</dd>
                {/if}
              </div>
            </div>
            {#if s.database}
              <div class="flex items-center gap-3 py-[11px]">
                {@render light('bg-grey', 'Info')}
                <div class="flex min-w-0 flex-1 flex-col gap-[3px]">
                  <dt class="text-sm text-soft">Database</dt>
                  <dd class="num m-0 text-[25px] leading-none">{size(s.database_bytes)}</dd>
                  <dd class="m-0 truncate text-[13px] text-dim" title={s.database}>{s.database}</dd>
                </div>
              </div>
            {/if}
          </dl>
        </Panel>

        <div class="flex min-w-0 flex-[1_1_520px] flex-col gap-4">
          <section aria-label="Numbers" class="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
            {#each [{ tag: 'ONLINE', accent: 'shadow-[inset_0_3px_0_#2f9e5c,0_8px_24px_rgb(0_0_0/0.35)]', value: data.counts.online, label: 'Online now', href: '/online' }, { tag: 'FLEET', accent: 'shadow-[inset_0_3px_0_var(--color-cyan),0_8px_24px_rgb(0_0_0/0.35)]', value: data.counts.commanders, label: 'Commanders', href: '/commanders' }, { tag: 'NEW', accent: 'shadow-[inset_0_3px_0_#2f9e5c,0_8px_24px_rgb(0_0_0/0.35)]', value: data.counts.new_today, label: 'New today', href: '/commanders', ok: true }, { tag: 'MAIL', accent: 'shadow-[inset_0_3px_0_#3d8bff,0_8px_24px_rgb(0_0_0/0.35)]', value: data.counts.mails_today, label: 'Mails sent today', href: '/broadcast' }] as tile (tile.label)}
              <a href={tile.href} use:link class={['ip-panel flex flex-col gap-1 rounded-t-none px-4 py-3 text-white no-underline hover:text-white', tile.accent]}>
                <span class="ip-sub">{tile.tag}</span>
                <span class={['num text-[46px] leading-none', tile.ok && tile.value > 0 && 'text-ok']}>{count(tile.value)}</span>
                <span class="text-sm text-soft">{tile.label}</span>
              </a>
            {/each}
          </section>

          <Panel title="Recent activity" sub="TODAY {stamp(data.day_start).slice(0, 10)}">
            {#if data.activity.length}
              <ol class="m-0 flex list-none flex-col gap-1 p-2.5">
                {#each data.activity as event, i (i)}
                  <li class="ip-row grid min-h-10 grid-cols-[70px_52px_minmax(0,1fr)] items-center gap-x-3 gap-y-1 sm:grid-cols-[70px_52px_150px_minmax(0,1fr)] px-3.5 py-1">
                    <span class={['ip-tag justify-self-start text-white', tags[event.kind].bg]}>{tags[event.kind].label}</span>
                    <span class="num text-[22px]">{clock(event.time)}</span>
                    {#if event.actor_id}
                      <a href="/commanders/{event.actor_id}" use:link class="truncate text-[15px] text-gold no-underline hover:text-gold hover:underline">{event.actor}</a>
                    {:else}
                      <span class="truncate text-[15px] text-gold">{event.actor}</span>
                    {/if}
                    <span class="col-span-3 text-[15px] text-[#e5edff] sm:col-span-1">{event.what}</span>
                  </li>
                {/each}
              </ol>
            {:else}
              <p class="m-0 px-5 py-6 text-base text-dim">Nothing yet today</p>
            {/if}
          </Panel>
        </div>

        <div class="flex w-full flex-none flex-col gap-4 sm:w-[320px]">
          <Panel title="Quick actions" sub="ALL PLAYERS">
            <div class="flex gap-2.5 px-5 py-4">
              <a href="/notices" use:link class="ip-btn h-12 min-w-0 flex-1 px-2 text-[17px]">Post a notice</a>
              <a href="/broadcast" use:link class="ip-btn h-12 min-w-0 flex-1 px-2 text-[17px]">Mail everyone</a>
            </div>
          </Panel>
          <section aria-label="Kick everyone" class="ip-panel ip-danger-zone flex flex-col px-5 pt-4 pb-5">
            <button type="button" class="ip-danger" disabled={kickAll.isPending || data.counts.online === 0} onclick={kickEveryone}>Kick everyone</button>
          </section>
        </div>
      </main>
    {:else}
      <main class="flex-1"></main>
    {/if}

  </div>
</Page>
