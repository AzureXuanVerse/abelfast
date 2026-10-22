<script lang="ts">
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import { api } from '../lib/api'
  import { createWrite, get, keys, type Kicked } from '../lib/query'
  import { link } from '../lib/router.svelte'
  import { overviewKeys, type RosterRow } from '../lib/overview'
  import Page from '../components/Page.svelte'

  const online = createQuery(() => ({
    queryKey: keys.online,
    queryFn: get<RosterRow[]>('/overview/online'),
  }))
  const players = $derived(online.data)

  const client = useQueryClient()
  const kick = createWrite((player: RosterRow) => api<Kicked>('POST', `/commanders/${player.id}/kick`), [keys.online, keys.commanders, overviewKeys.overview], {
    success: ({ kicked }, player) => (kicked ? `${player.name} was kicked` : `${player.name} isn't playing`),
    onSuccess: (_, player) => client.invalidateQueries({ queryKey: keys.commander(player.id) }),
  })

  function progress(player: RosterRow) {
    if (!player.exp_next) return 100
    return Math.min(100, ((player.exp ?? 0) / player.exp_next) * 100)
  }
</script>

{#snippet actions()}
  <a href="/commanders" use:link class="ip-btn2 h-[46px] text-[19px]">All commanders</a>
{/snippet}

<Page title="Online now" sub="PLAYERS" {actions}>
  <main aria-label="Players online" class="flex flex-col gap-3.5 pt-[18px] pr-10 pb-7 pl-16">
    {#if players && players.length === 0}
      <p class="ip-panel m-0 px-6 py-5 text-lg text-dim">Nobody is playing</p>
    {/if}
    {#each players ?? [] as player (player.id)}
      <article class="ip-panel flex items-center gap-6 py-[18px] pr-6 pl-[18px]">
        <div class="size-[104px] flex-none overflow-hidden rounded bg-[#151a26] shadow-[0_0_0_3px_#fff,0_4px_10px_rgb(0_0_0/0.35)]">
          {#if player.icon}<img src="/api/assets/{player.icon}.png" alt="" class="size-full object-cover" />{/if}
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-2">
          <div class="flex items-center gap-3">
            <span class="truncate text-[30px] text-gold">{player.name}</span>
            <span class="text-[15px] text-soft">UID:<span class="num text-lg">{player.id}</span></span>
            <span class="ip-tag bg-[#2f9e5c] text-white">ONLINE</span>
          </div>
          <span class="text-base text-soft">Connected from <span class="num text-xl text-white">{player.ip}</span></span>
          <div class="flex items-center gap-3">
            <span class="ip-sub text-soft">LV.</span><span class="num -ml-2 text-2xl">{player.level}</span>
            <span aria-hidden="true" class="h-2 w-[340px] max-w-full rounded bg-black/45"><span class="block h-full rounded bg-cyan" style:width="{progress(player)}%"></span></span>
          </div>
        </div>
        <a href="/commanders/{player.id}" use:link class="ip-btn2" aria-label="Open {player.name}">Open</a>
        <button type="button" class="ip-danger w-[120px]" aria-label="Kick {player.name}" disabled={kick.isPending && kick.variables?.id === player.id} onclick={() => kick.mutate(player)}>Kick</button>
      </article>
    {/each}
  </main>
</Page>
