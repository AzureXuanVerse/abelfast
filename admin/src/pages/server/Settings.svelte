<script lang="ts">
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import { api } from '../../lib/api'
  import { createWrite, get } from '../../lib/query'
  import { count, size, stamp } from '../../lib/format'
  import { overviewKeys } from '../../lib/overview'
  import { serverKeys, type ServerSettings, type UpstreamVersion } from '../../lib/server'
  import Page from '../../components/Page.svelte'

  const settings = createQuery(() => ({
    queryKey: serverKeys.settings,
    queryFn: get<ServerSettings>('/server/settings'),
  }))
  const data = $derived(settings.data)

  // the version check updates the last check shown, failed or not
  const client = useQueryClient()
  const check = createWrite(
    () =>
      api<UpstreamVersion>('POST', '/server/version-check').catch((err) => {
        client.invalidateQueries({ queryKey: serverKeys.settings })
        throw err
      }),
    [serverKeys.settings, overviewKeys.overview],
    { success: (version) => `Client ${version.client_version || '?'}` },
  )

  const logoutAll = createWrite(() => api('POST', '/server/logout-all'), [], { onSuccess: () => location.reload() })
  function logOutEveryone() {
    if (confirm('End every back-office session, this one included?')) logoutAll.mutate()
  }

  // the $<name>hash$ version strings but $azhash$, and the plain ones
  const otherVersions = $derived(
    (data?.version?.versions ?? [])
      .filter((v) => !v.startsWith('$azhash$'))
      .map((v) => {
        const fields = v.split('$')
        return fields.length === 4 ? { name: fields[1].replace(/hash$/, ''), value: fields[2], hash: fields[3] } : { name: v, value: '', hash: '' }
      }),
  )

</script>

{#snippet light(color: string, label: string, tall = 'h-7')}
  <span role="img" aria-label={label} class={['w-2 flex-none rounded-sm', tall, color]}></span>
{/snippet}

{#snippet row(label: string, value: string, num = true, title = '')}
  <div class="grid grid-cols-[140px_minmax(0,1fr)] items-baseline gap-3">
    <dt class="text-[17px] whitespace-nowrap">{label}</dt>
    <dd class={['m-0 min-w-0 truncate', num ? 'num text-[22px]' : 'text-[17px] text-soft']} title={title || value}>{value}</dd>
  </div>
{/snippet}

<Page title="Settings" sub="SERVER" back="/">
  <div class="relative flex items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    {#if data}
      <main class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] items-start gap-5">
        <div class="flex min-w-0 flex-col gap-5">
          <section id="server" aria-labelledby="h-server" class="ip-panel scroll-mt-4">
            <div class="ip-head"><h2 id="h-server">Server</h2><span class="ip-sub">GAME SERVER</span></div>
            <dl class="m-0 flex flex-col gap-3.5 px-5 pt-4 pb-5">
              {@render row('Region', data.region, false)}
              {@render row('Listen address', data.listen_addr)}
              {#each data.servers as server (server.id)}
                {@render row('Server name', server.name, false)}
                {@render row('Address', `${server.host}:${server.port}`)}
              {/each}
            </dl>
          </section>

          <section id="access" aria-labelledby="h-access" class="ip-panel scroll-mt-4">
            <div class="ip-head"><h2 id="h-access">Access</h2><span class="ip-sub">BACK-OFFICE</span></div>
            <div class="flex flex-col gap-3.5 px-5 pt-4 pb-5">
              {#if data.password_set}
                <div class="ip-row flex items-center gap-3 px-3.5 py-2.5 shadow-[inset_0_3px_0_#2f9e5c]">
                  {@render light('bg-ok', 'Protected')}
                  <span class="text-[15px] text-ok">Password set</span>
                </div>
              {:else}
                <div role="alert" class="ip-row flex items-center gap-3 px-3.5 py-2.5 shadow-[inset_0_3px_0_var(--color-warn)]">
                  {@render light('bg-warn', 'Open')}
                  <span class="text-[15px] text-[#ffd8a6]">No password set.</span>
                </div>
              {/if}
              <dl class="m-0 flex flex-col gap-3.5">
                {@render row('Listen address', data.admin_addr)}
                {#if data.password_set}{@render row('Sessions', count(data.sessions))}{/if}
              </dl>
              {#if data.password_set}
                <div role="group" aria-label="Log everyone out" class="ip-row flex flex-col items-end px-4 pt-3.5 pb-4 shadow-[inset_0_3px_0_var(--color-red)]">
                  <button type="button" class="ip-danger" disabled={logoutAll.isPending} onclick={logOutEveryone}>Log everyone out</button>
                </div>
              {/if}
            </div>
          </section>
        </div>

        <div class="flex min-w-0 flex-col gap-5">
          <section id="gateway" aria-labelledby="h-gateway" class="ip-panel scroll-mt-4">
            <div class="ip-head"><h2 id="h-gateway">Gateway</h2><span class="ip-sub">SERVER LIST</span></div>
            <div class="flex flex-col gap-2 px-4 pt-3 pb-[18px]">
              <div aria-hidden="true" class="ip-sub grid grid-cols-[8px_26px_minmax(0,1fr)_minmax(0,140px)_60px_90px] gap-3 px-3.5">
                <span></span><span>ID</span><span>NAME</span><span>HOST</span><span>PORT</span><span>STATE</span>
              </div>
              <ul aria-label="Server list" class="m-0 flex list-none flex-col gap-1.5 p-0">
                {#each data.servers as server (server.id)}
                  <li class="ip-row grid min-h-[50px] grid-cols-[8px_26px_minmax(0,1fr)_minmax(0,140px)_60px_90px] items-center gap-3 px-3.5 py-1.5">
                    {@render light('bg-ok', 'Online')}
                    <span class="num text-[26px]">{server.id}</span>
                    <span class="truncate text-lg">{server.name}</span>
                    <span class="num truncate text-[21px]" title={server.host}>{server.host}</span>
                    <span class="num text-[21px]">{server.port}</span>
                    <span class="text-base text-ok">Online</span>
                  </li>
                {/each}
              </ul>

              <div class="ip-row mt-2 flex flex-col gap-2.5 px-4 py-3.5">
                <div class="flex items-baseline gap-2.5">
                  <h3 class="m-0 text-xl">Official gateway</h3>
                  <span class="ip-sub">VERSION CHECK</span>
                </div>
                <div class="grid grid-cols-[96px_minmax(0,1fr)_auto] items-center gap-3">
                  <span class="text-base">Upstream</span>
                  <span class="num truncate text-[22px]" title={data.upstream}>{data.upstream}</span>
                  <button type="button" class="ip-btn h-[46px] text-[19px]" disabled={check.isPending} onclick={() => check.mutate()}>Check now</button>
                </div>
                {#if data.official && data.official !== data.upstream}
                  <div class="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-3">
                    <span class="text-base">Official</span>
                    <span class="num truncate text-[20px] text-soft">{data.official}</span>
                  </div>
                {/if}
                <div class="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-3">
                  <span class="text-base">Last check</span>
                  {#if data.version}
                    <span class="flex min-w-0 items-center gap-2.5 text-sm text-soft">
                      {@render light(data.version_error ? 'bg-warn' : 'bg-ok', data.version_error ? 'Stale' : 'Fresh', 'h-[26px]')}
                      <span class="num text-[22px] whitespace-nowrap text-white">{stamp(data.version.at)}</span>
                    </span>
                  {:else}
                    <span class="flex items-center gap-2.5 text-sm text-soft">
                      {@render light('bg-grey', 'None', 'h-[26px]')}
                      Never
                    </span>
                  {/if}
                </div>
                {#if data.version_error}
                  <p class="m-0 text-sm break-words text-bad">{data.version_error}</p>
                {/if}
                {#if data.version}
                  <div class="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-3">
                    <span class="text-base">Client</span>
                    <span class="num text-[22px]">{data.version.client_version || '-'}</span>
                  </div>
                  <div class="grid grid-cols-[96px_minmax(0,1fr)] items-center gap-3">
                    <span class="text-base">Assets</span>
                    <span class="num truncate text-[20px]" title={data.version.asset_hash}>{data.version.asset_hash || '-'}</span>
                  </div>
                  {#if otherVersions.length}
                    <ul aria-label="Other hashes" class="m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-1.5 p-0">
                      {#each otherVersions as v (v.name)}
                        <li class="flex items-baseline gap-2 rounded-[3px] bg-black/20 px-2.5 py-1" title={v.hash}>
                          <span class="text-[13px] text-dim">{v.name}</span>
                          <span class="num text-lg">{v.value}</span>
                        </li>
                      {/each}
                    </ul>
                  {/if}
                {/if}
              </div>
            </div>
          </section>

          <section id="data" aria-labelledby="h-data" class="ip-panel scroll-mt-4">
            <div class="ip-head"><h2 id="h-data">Data</h2><span class="ip-sub">STORAGE</span></div>
            <div class="flex flex-col gap-3 px-5 pt-4 pb-5">
              <dl class="m-0 flex flex-col gap-3">
                <div class="grid grid-cols-[110px_minmax(0,1fr)_auto] items-baseline gap-3">
                  <dt class="text-[17px]">Database</dt>
                  <dd dir="rtl" class="num m-0 min-w-0 truncate text-left text-[20px]" title={data.database}><bdi>{data.database || '-'}</bdi></dd>
                  <dd class="num m-0 text-[20px] text-soft">{data.database ? size(data.database_bytes) : ''}</dd>
                </div>
              </dl>
              <div class="ip-row flex flex-col gap-2.5 px-4 py-3.5">
                <div class="flex items-baseline gap-2.5">
                  <h3 class="m-0 text-xl">Game data</h3>
                  <span class="ip-sub">SHARECFG</span>
                </div>
                <dl class="m-0 flex flex-col gap-2.5">
                  <div class="grid grid-cols-[96px_minmax(0,1fr)] items-baseline gap-3">
                    <dt class="text-base">Directory</dt>
                    <dd dir="rtl" class="num m-0 min-w-0 truncate text-left text-[20px]" title={data.region_dir}><bdi>{data.region_dir}</bdi></dd>
                  </div>
                  <div class="grid grid-cols-[96px_minmax(0,1fr)] items-baseline gap-3">
                    <dt class="text-base">Tables</dt>
                    <dd class="m-0 text-sm text-soft"><span class="num text-[22px] text-white">{count(data.tables)}</span> files</dd>
                  </div>
                  <div class="grid grid-cols-[96px_minmax(0,1fr)] items-baseline gap-3">
                    <dt class="text-base">Server</dt>
                    <dd class="m-0 text-sm text-soft"><span class="num text-[22px] text-white">{count(data.server_files)}</span> files</dd>
                  </div>
                </dl>
              </div>
            </div>
          </section>
        </div>
      </main>
    {/if}
  </div>
</Page>
