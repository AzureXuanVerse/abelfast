<script lang="ts">
  import { createQuery, useQueryClient } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import VirtualGrid from '../../components/VirtualGrid.svelte'
  import { ago, day, short } from '../../lib/format'
  import { fuzzy } from '../../lib/fuzzy'
  import { link } from '../../lib/router.svelte'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import type { FriendRow, Friends } from '../../lib/social'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'friends'),
    queryFn: get<Friends>(`/commanders/${id}/friends`),
  }))
  const data = $derived(query.data)
  let search = $state('')
  const [searched] = createDebouncedValue(() => search, { wait: 150 })

  // the other commander's lists change too (a friendship ends on both sides)
  const client = useQueryClient()
  const { mutation: edit, run: send } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)
  const run = (method: string, path: string, other: number, done: string) => send(path, done, undefined, method, () => client.invalidateQueries({ queryKey: keys.commander(other) }))

  function remove(friend: FriendRow) {
    if (confirm(`End the friendship with ${friend.name} on both sides?`)) run('DELETE', `/friends/${friend.id}`, friend.id, `${friend.name} removed`)
  }

  const shown = $derived(fuzzy(data?.friends ?? [], searched(), (f) => [f.name, f.id]))
</script>

<CommanderPage {id} title="Friends">
  {#snippet children()}
    {#if data}
      <div class="flex items-start gap-6">
        <section aria-label="Friends" class="ip-panel min-w-0 flex-1">
          <div class="ip-head items-center">
            <h2>Friends</h2>
            <span class="ip-sub">List</span>
            <span class="num text-2xl">{data.friends.length}</span><span class="num -ml-2 text-base text-dim">/{data.max}</span>
            <label class="relative ml-auto">
              <span class="sr-only">Name or UID</span>
              <svg aria-hidden="true" viewBox="0 0 24 24" class="absolute top-1/2 left-3 size-5 -translate-y-1/2" fill="none" stroke="#4a5163" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="6.5" /><path d="M16 16l4.5 4.5" /></svg>
              <input class="ip-field h-[42px] w-[190px] pl-10" placeholder="Name or UID" bind:value={search} />
            </label>
          </div>
          <div class="grid grid-cols-[minmax(0,1.6fr)_70px_minmax(0,1fr)_minmax(0,0.9fr)_44px] gap-3 px-6 pt-3 pb-1">
            <span class="ip-sub">Commander</span><span class="ip-sub">Level</span><span class="ip-sub">Last online</span><span class="ip-sub">Friends since</span><span></span>
          </div>
          <VirtualGrid items={shown} rowHeight={68} key={(f) => f.id} label="Friends" class="max-h-[calc(100vh-250px)] px-2.5 pb-2.5">
            {#snippet cell(friend)}
              <div class="ip-row grid grid-cols-[minmax(0,1.6fr)_70px_minmax(0,1fr)_minmax(0,0.9fr)_44px] items-center gap-3 px-3">
                <div class="flex min-w-0 items-center gap-3">
                  <Icon icon={friend.icon} rarity={friend.rarity} ship name={friend.name} size={52} />
                  <div class="flex min-w-0 flex-col">
                    <a href="/commanders/{friend.id}" use:link class="truncate text-lg text-gold no-underline hover:text-white">{friend.name}</a>
                    <span class="text-[13px] text-dim">UID:<span class="num">{friend.id}</span></span>
                  </div>
                </div>
                <span class="num text-2xl">{friend.level}</span>
                <span class={['text-[15px]', friend.online ? 'text-ok' : 'text-soft']}>{friend.online ? 'Online now' : ago(friend.last_online ?? 0)}</span>
                <span class="num text-xl text-soft">{day(friend.since ?? 0)}</span>
                <button
                  type="button"
                  aria-label="Remove {friend.name}"
                  class="grid size-[34px] cursor-pointer place-items-center rounded-full border-0 bg-red text-white hover:bg-red-hi disabled:opacity-50"
                  disabled={busy}
                  onclick={() => remove(friend)}
                >
                  <svg aria-hidden="true" viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18" /></svg>
                </button>
              </div>
            {/snippet}
            {#snippet empty()}
              <div class="ip-row px-3 py-2.5 text-dim">{search ? 'No match' : 'No friends'}</div>
            {/snippet}
          </VirtualGrid>
        </section>

        <div class="flex w-[432px] flex-none flex-col gap-5">
          <section aria-label="Requests" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Requests</h2>
              <span class="ip-sub">Waiting</span>
              <span class="num ml-auto text-2xl">{data.requests.length}</span>
            </div>
            <div class="flex max-h-[480px] flex-col gap-1.5 overflow-y-auto p-2.5">
              {#each data.requests as request (request.id)}
                <div class="ip-row flex flex-col gap-2 px-3 py-2.5">
                  <div class="flex items-center gap-3">
                    <Icon icon={request.icon} rarity={request.rarity} ship name={request.name} size={44} />
                    <div class="flex min-w-0 flex-col">
                      <a href="/commanders/{request.id}" use:link class="truncate text-[17px] text-gold no-underline hover:text-white">{request.name}</a>
                      <span class="text-[13px] whitespace-nowrap text-dim">UID:<span class="num">{request.id}</span> · Lv.<span class="num">{request.level}</span></span>
                    </div>
                    <span class="num ml-auto self-start text-base whitespace-nowrap text-soft">{short(request.sent_at ?? 0, '/')}</span>
                  </div>
                  {#if request.content}<div class="rounded-[3px] bg-black/30 px-3 py-1.5 text-[15px] break-words">{request.content}</div>{/if}
                  <div class="flex justify-end gap-2">
                    <button type="button" class="ip-btn2 ip-sm" disabled={busy} onclick={() => run('DELETE', `/requests/${request.id}`, request.id, `${request.name}'s request rejected`)}>Reject</button>
                    <button type="button" class="ip-btn ip-sm" disabled={busy} onclick={() => run('POST', `/requests/${request.id}/accept`, request.id, `${request.name} is now a friend`)}>Accept</button>
                  </div>
                </div>
              {:else}
                <div class="ip-row px-3 py-2.5 text-dim">None</div>
              {/each}
            </div>
          </section>

          <section aria-label="Blacklist" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Blacklist</h2>
              <span class="ip-sub">Blocked</span>
              <span class="num ml-auto text-2xl">{data.blacklist.length}</span>
            </div>
            <VirtualGrid items={data.blacklist} rowHeight={60} key={(b) => b.id} label="Blacklist" class="max-h-[420px] p-2.5">
              {#snippet cell(blocked)}
                <div class="ip-row flex items-center gap-3 px-3">
                  <Icon icon={blocked.icon} rarity={blocked.rarity} ship name={blocked.name} size={44} />
                  <div class="flex min-w-0 flex-col">
                    <a href="/commanders/{blocked.id}" use:link class="truncate text-[17px] text-gold no-underline hover:text-white">{blocked.name}</a>
                    <span class="truncate text-[13px] text-dim">UID:<span class="num">{blocked.id}</span> · blocked <span class="num text-sm">{short(blocked.blocked_at ?? 0, '/')}</span></span>
                  </div>
                  <button type="button" class="ip-btn2 ip-sm ml-auto" disabled={busy} onclick={() => run('DELETE', `/blacklist/${blocked.id}`, blocked.id, `${blocked.name} unblocked`)}>Unblock</button>
                </div>
              {/snippet}
              {#snippet empty()}
                <div class="ip-row px-3 py-2.5 text-dim">None</div>
              {/snippet}
            </VirtualGrid>
          </section>
        </div>
      </div>
    {/if}
  {/snippet}
</CommanderPage>
