<script lang="ts">
  // The commander's combat modes, each section with its own actions:
  // Exercises (rank, score, fights, defense fleet, rivals), Operation Siren
  // (the world's state and maps, its reset), META (the boss, sync points).
  import { createQuery } from '@tanstack/svelte-query'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import Meta from './combat/Meta.svelte'
  import Siren from './combat/Siren.svelte'
  import { clock, count, day } from '../../lib/format'
  import { link } from '../../lib/router.svelte'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import type { Combat, SocialShip } from '../../lib/social'
  import { bounded } from '../../lib/bounded'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'combat'),
    queryFn: get<Combat>(`/commanders/${id}/combat`),
  }))
  const combat = $derived(query.data)
  // the score editor starts at the commander's, again after each fetch
  let score = $derived(combat?.exercise.score ?? 0)
  // the rank tile turns into the score editor when clicked
  let editingScore = $state(false)

  const { mutation: edit, run } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)

  // the rank a score reaches, for the rank picker (orders aside)
  const scoreRank = $derived(combat ? [...combat.exercise.ranks].reverse().find((rank) => rank.point <= score)?.id ?? 1 : 1)

  function pickRank(rankID: number) {
    const rank = combat?.exercise.ranks.find((row) => row.id === rankID)
    if (rank) score = rank.point
  }

  function saveScore(event: SubmitEvent) {
    event.preventDefault()
    editingScore = false
    if (combat && score !== combat.exercise.score) run('/exercise/score', 'Score set', { score: Math.floor(score) })
  }

  const rankProgress = $derived.by(() => {
    if (!combat) return 0
    const { score, rank, next } = combat.exercise
    if (!next) return 100
    return Math.max(0, Math.min(100, ((score - rank.point) / Math.max(1, next.point - rank.point)) * 100))
  })
</script>

{#snippet fleet(ships: SocialShip[])}
  {#each ships as ship (ship.id)}
    <div class="flex w-[108px] flex-col items-center gap-1">
      <Icon icon={ship.icon} rarity={ship.rarity} ship name={ship.name} count={ship.level} size={96} />
      <span class="w-full truncate text-center text-sm">{ship.name}</span>
    </div>
  {/each}
{/snippet}

<CommanderPage {id} title="Combat">
  {#snippet children()}
    {#if combat}
      {@const ex = combat.exercise}
      {@const world = combat.world}
      {@const meta = combat.meta}
      <div class="flex flex-col gap-5">
          <section aria-label="Exercises" class="ip-panel">
            <div class="ip-head flex-wrap items-center gap-y-2">
              <h2>Exercises</h2>
              <span class="text-[15px] text-soft">Season <span class="num text-xl text-white">{day(ex.season)}</span> to <span class="num text-xl text-white">{day(ex.season_end)}</span></span>
              <span class="ml-auto flex gap-2">
                <button type="button" class="ip-btn2 ip-sm" disabled={busy || ex.fight_count >= ex.fight_max} onclick={() => run('/exercise/refill', 'Fights refilled')}>Refill fights</button>
                <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => run('/exercise/rivals', 'Rivals cleared')}>New rivals</button>
              </span>
            </div>
            <div class="flex flex-col gap-[18px] px-5 pt-[18px] pb-5">
              <div class="grid grid-cols-[minmax(0,2fr)_repeat(3,minmax(0,1fr))] gap-3">
                {#if editingScore}
                  <form class="ip-row flex flex-col gap-2 px-4 py-3" onsubmit={saveScore}>
                    <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] gap-2">
                      <!-- svelte-ignore a11y_autofocus -->
                      <input class="ip-field num h-10 text-xl" type="number" aria-label="Score" autofocus bind:value={score} use:bounded={{ min: 0 }} onkeydown={(e) => e.key === 'Escape' && ((editingScore = false), (score = ex.score))} />
                      <select class="ip-field h-10" aria-label="Rank" value={scoreRank} onchange={(event) => pickRank(Number(event.currentTarget.value))}>
                        {#each ex.ranks as rank (rank.id)}<option value={rank.id}>{rank.name}</option>{/each}
                      </select>
                    </div>
                    <div class="flex justify-end gap-2">
                      <button type="button" class="ip-btn2 ip-sm" onclick={() => ((editingScore = false), (score = ex.score))}>Cancel</button>
                      <button type="submit" class="ip-btn ip-sm" disabled={busy}>Save</button>
                    </div>
                  </form>
                {:else}
                  <button type="button" aria-label="Edit the score" class="ip-row flex cursor-pointer flex-col gap-1.5 border-0 px-4 py-3 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => (editingScore = true)}>
                    <span class="ip-sub">Rank</span>
                    <span class="flex flex-wrap items-baseline gap-x-2.5">
                      <span class="text-2xl text-gold">{ex.rank.name}</span>
                      <span class="num text-[22px] text-soft">{count(ex.score)}</span>
                      <span class="text-sm text-dim">pts</span>
                    </span>
                    <span aria-hidden="true" class="block h-1.5 w-full rounded-[3px] bg-black/40"><span class="block h-full rounded-[3px] bg-cyan" style:width="{rankProgress}%"></span></span>
                    <span class="text-[13px] whitespace-nowrap text-dim">
                      {#if ex.next}{ex.next.name} at <span class="num text-base text-soft">{count(ex.next.point)}</span>{:else}Highest rank{/if}
                      · #<span class="num text-base text-soft">{ex.standing}</span>
                    </span>
                  </button>
                {/if}
                <div class="ip-row flex flex-col gap-1 px-4 py-3">
                  <span class="ip-sub">Fights left</span>
                  <span><span class="num text-[38px]">{ex.fight_count}</span><span class="num text-xl text-dim"> /{ex.fight_max}</span></span>
                  <span class="text-[13px] text-dim">Refills <span class="num text-base text-soft">{clock(ex.next_refill)}</span></span>
                </div>
                <div class="ip-row flex flex-col gap-1 px-4 py-3">
                  <span class="ip-sub">New rivals</span>
                  <span><span class="num text-[38px]">{ex.flash_count}</span><span class="num text-xl text-dim"> /{ex.flash_max}</span></span>
                  <span class="text-[13px] text-dim">Today</span>
                </div>
                <div class="ip-row flex flex-col gap-1 px-4 py-3">
                  <span class="ip-sub">Shop resets</span>
                  <span class="num text-[38px]">{ex.shop_refreshes}</span>
                  <span class="text-[13px] text-dim">Today</span>
                </div>
              </div>
              <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-5">
                <div class="flex flex-col gap-2.5">
                  <h3 class="m-0 flex items-baseline gap-2.5 text-xl">Defense fleet <a href="/commanders/{id}/fleets?board=defense" use:link class="ml-auto text-base text-link">Edit in Fleets</a></h3>
                  <div class="flex flex-wrap gap-2">{@render fleet(ex.vanguard)}</div>
                  <div class="flex flex-wrap gap-2">{@render fleet(ex.main)}</div>
                </div>
                <div class="flex flex-col gap-2.5">
                  <h3 class="m-0 flex items-baseline gap-2.5 text-xl">Rivals <span class="ip-sub">Current</span></h3>
                  {#each ex.rivals as rival (rival.id)}
                    <div class="ip-row flex items-center gap-3 px-3 py-2">
                      <Icon icon={rival.icon} rarity={rival.rarity} ship name={rival.name} size={44} />
                      <a href="/commanders/{rival.id}" use:link class="truncate text-lg text-link">{rival.name}</a>
                      <span class="text-sm text-dim">Lv.<span class="num">{rival.level}</span></span>
                      <span class="ml-auto truncate text-[15px] text-soft">{rival.rank}</span>
                      <span class="num w-20 text-right text-[22px]">{count(rival.score)}</span>
                    </div>
                  {:else}
                    <div class="ip-row px-3 py-2.5 text-dim">None drawn</div>
                  {/each}
                </div>
              </div>
            </div>
          </section>

          <Siren {id} {world} />

          <Meta {id} {meta} />
      </div>
    {/if}
  {/snippet}
</CommanderPage>
