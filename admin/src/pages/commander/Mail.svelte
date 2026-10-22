<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import PlayersDrops from '../../components/PlayersDrops.svelte'
  import PlayersQueue from '../../components/PlayersQueue.svelte'
  import Seg from '../../components/Seg.svelte'
  import { api, dropTypes } from '../../lib/api'
  import { createCommanderMutation, done, get, type Kicked } from '../../lib/query'
  import { drop, playerKeys, shipFramed, type PlayerMail, type PlayerMails, type Queued } from '../../lib/players'
  import { bounded } from '../../lib/bounded'
  import { stamp } from '../../lib/format'

  let { id }: { id: number } = $props()

  const mails = createQuery(() => ({
    queryKey: playerKeys.mails(id),
    queryFn: get<PlayerMails>(`/commanders/${id}/mails`),
  }))

  type Box = 'inbox' | 'storage' | 'compensations'
  let box = $state<Box>('inbox')
  let filter = $state<'all' | 'unread' | 'important'>('all')
  let selectedID = $state<number | null>(null)
  const selected = $derived(mails.data?.[box].find((m) => m.id === selectedID) ?? null)

  // the send form
  let kind = $state<'mail' | 'compensation'>('mail')
  let title = $state('')
  let sender = $state('')
  let content = $state('')
  let hours = $state(72)
  let attachments = $state<Queued[]>([])
  let picking = $state(false)

  const boxes: { key: Box; label: string }[] = [
    { key: 'inbox', label: 'Mail' },
    { key: 'storage', label: 'Storage' },
    { key: 'compensations', label: 'Compensations' },
  ]

  const shown = $derived((mails.data?.[box] ?? []).filter((m) => box !== 'inbox' || filter === 'all' || (filter === 'unread' ? !m.read : m.important)))
  const counter = $derived.by(() => {
    const data = mails.data
    if (!data) return null
    if (box === 'inbox') return { label: 'MAIL', value: `${data.inbox.length}/${data.inbox_max}` }
    if (box === 'storage') return { label: 'STORED', value: `${data.storage.length}/${data.storage_max}` }
    return { label: 'ACTIVE', value: String(data.compensations.length) }
  })

  function open(next: Box) {
    box = next
    selectedID = null
  }

  const remove = createCommanderMutation(
    () => id,
    ({ mail, compensation }: { mail: PlayerMail; compensation: boolean; commander: string }) => api<Kicked>('DELETE', `/commanders/${id}/${compensation ? 'compensations' : 'mails'}/${mail.id}`),
    { success: (answer, { commander }) => done('Deleted', commander)(answer), onSuccess: () => (selectedID = null) },
  )

  function confirmRemove(commander: string) {
    if (!selected) return
    const compensation = box === 'compensations'
    if (confirm(`Delete the ${compensation ? 'compensation' : 'mail'} "${selected.title}"?`)) remove.mutate({ mail: selected, compensation, commander })
  }

  function clear() {
    title = ''
    sender = ''
    content = ''
    attachments = []
    picking = false
  }

  const send = createCommanderMutation(
    () => id,
    (body: { title: string; sender: string; content: string; attachments: { type: number; id: number; count: number }[]; expire_hours: number }) =>
      api<{ id: number; kind: string }>('POST', `/commanders/${id}/mail`, body),
    { success: ({ kind }) => (kind === 'compensation' ? 'Compensation sent' : 'Mail sent'), onSuccess: clear },
  )

  function submit() {
    send.mutate({
      title: title.trim(),
      sender: sender.trim(),
      content,
      attachments: attachments.map(({ type, id, count }) => ({ type, id, count })),
      expire_hours: kind === 'compensation' ? Number(hours) : 0,
    })
  }

  const first = (m: PlayerMail) => m.attachments[0]
</script>

<CommanderPage {id} title="Mail">
  {#snippet children(summary)}
    <div class="flex flex-wrap items-start gap-6">
      <section aria-label="Mailbox" class="ip-panel flex w-[440px] flex-none flex-col overflow-hidden">
        <nav aria-label="Box" class="ip-tabs flex-nowrap px-2 [&>button]:px-4">
          {#each boxes as b (b.key)}
            <button type="button" aria-pressed={box === b.key} onclick={() => open(b.key)}>{b.label}</button>
          {/each}
        </nav>
        <div class="flex items-end justify-between gap-2 px-4 pt-3.5">
          {#if box === 'inbox'}
            <div class="shrink-0"><Seg
              label="Show"
              bind:value={filter}
              options={[
                { value: 'all', label: 'All' },
                { value: 'unread', label: 'Unread' },
                { value: 'important', label: 'Important' },
              ]}
            /></div>
          {:else}
            <span></span>
          {/if}
          {#if counter}
            <span class="flex items-baseline gap-2"><span class="ip-sub">{counter.label}</span><span class="num text-[26px] leading-none">{counter.value}</span></span>
          {/if}
        </div>
        <p class="m-0 px-4 pt-3 pb-1 text-[15px] text-soft">Newest first</p>
        <ul class="m-0 flex max-h-[calc(100vh-260px)] min-h-[300px] list-none flex-col gap-2.5 overflow-y-auto px-4 pt-1 pb-4">
          {#each shown as mail (mail.id)}
            {@const a = first(mail)}
            <li>
              <button
                type="button"
                aria-pressed={selectedID === mail.id}
                class={['ip-row flex w-full cursor-pointer items-center gap-4 border-0 px-4 py-3 text-left text-white', selectedID === mail.id && 'ip-sel', !mail.read && box !== 'compensations' && 'bg-white/12']}
                onclick={() => (selectedID = mail.id)}
              >
                <div class="w-16 flex-none">
                  {#if a}<Icon icon={a.icon} rarity={a.rarity} ship={shipFramed(a.type)} count={a.count} size={62} name={a.name} />{/if}
                </div>
                <span class="flex min-w-0 flex-col gap-1">
                  <span class="flex items-center gap-2">
                    {#if mail.important}<span aria-label="Important" class="text-gold">★</span>{/if}
                    <span class="truncate text-[22px] leading-tight">{mail.title}</span>
                  </span>
                  <span class="flex flex-wrap items-center gap-2">
                    <span class="num text-2xl leading-none text-soft">{stamp(mail.date).slice(0, 10)}</span>
                    {#if mail.claimed}<span class="ip-tag bg-strip text-soft">CLAIMED</span>{/if}
                    {#if !mail.read && box !== 'compensations'}<span class="ip-tag bg-cyan text-white">UNREAD</span>{/if}
                    {#if mail.expires}<span class="ip-tag bg-warn text-white">UNTIL {stamp(mail.expires).slice(5)}</span>{/if}
                  </span>
                </span>
              </button>
            </li>
          {:else}
            {#if mails.data}<li class="py-2 text-lg text-dim">Nothing here</li>{:else if mails.error}<li class="py-2 text-bad">{mails.error.message}</li>{/if}
          {/each}
        </ul>
      </section>

      <div class="flex min-w-[360px] flex-[1_1_480px] flex-col gap-5">
        {#if selected}
          <section aria-labelledby="mail-h" class="ip-panel overflow-hidden">
            <div class="ip-head items-center">
              <h2 id="mail-h" class="min-w-0 truncate">{selected.title}</h2>
              <span class="ip-sub">#{selected.id}</span>
              <button type="button" class="ip-danger ip-sm ml-auto" disabled={remove.isPending} onclick={() => confirmRemove(summary.name)}>Delete</button>
            </div>
            <div class="flex flex-col gap-3.5 px-5 py-4">
              <dl class="m-0 grid grid-cols-[auto_1fr] gap-x-[18px] gap-y-1.5 rounded-[3px] bg-white/6 px-4 py-3">
                <dt class="text-[15px] text-dim">From</dt>
                <dd class="m-0 text-[17px]">{selected.sender || 'Default sender'}</dd>
                <dt class="text-[15px] text-dim">Sent</dt>
                <dd class="num m-0 text-xl">{stamp(selected.date)}</dd>
                {#if selected.expires}
                  <dt class="text-[15px] text-dim">Expires</dt>
                  <dd class="num m-0 text-xl">{stamp(selected.expires)}</dd>
                {/if}
                <dt class="text-[15px] text-dim">State</dt>
                <dd class="m-0 text-[17px]">
                  {[box !== 'compensations' && (selected.read ? 'Read' : 'Unread'), selected.attachments.length && (selected.claimed ? 'Claimed' : 'Not claimed'), selected.important && 'Important'].filter(Boolean).join(' · ')}
                </dd>
              </dl>
              {#if selected.content}<p class="m-0 text-[17px] leading-[1.45] break-words whitespace-pre-line">{selected.content}</p>{/if}
              {#if selected.attachments.length}
                <ul class="m-0 flex list-none flex-wrap gap-4 p-0">
                  {#each selected.attachments as a, i (i)}
                    <li class="flex w-[84px] flex-col items-center gap-1.5">
                      <Icon icon={a.icon} rarity={a.rarity} ship={shipFramed(a.type)} count={a.type === drop.skinTimed ? undefined : a.count} size={72} name={a.name} />
                      <span class="line-clamp-2 text-center text-[13px] leading-tight">{a.name || `${a.type}:${a.id}`}</span>
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          </section>
        {/if}

        <section aria-labelledby="send-h" class="ip-card">
          <div class="ip-card-head"><h2 id="send-h" class="truncate">Send mail to {summary.name}</h2></div>
          <form class="ip-card-body" onsubmit={(e) => (e.preventDefault(), submit())}>
            <Seg
              boxes
              columns={2}
              label="Kind"
              bind:value={kind}
              options={[
                { value: 'mail', label: 'Mail' },
                { value: 'compensation', label: 'Compensation' },
              ]}
            />
            <div class="grid grid-cols-2 gap-4">
              <label class="ip-lab">Title <input class="ip-field" bind:value={title} required /></label>
              <label class="ip-lab">From <input class="ip-field" bind:value={sender} placeholder="Default sender" /></label>
            </div>
            <label class="ip-lab">Text <textarea class="ip-field min-h-24" rows="3" bind:value={content}></textarea></label>
            {#if kind === 'compensation'}
              <label class="ip-lab w-1/2 pr-2">Expires after, hours <input class="ip-field num text-[22px]" type="number" min="1" bind:value={hours} use:bounded={{ min: 1 }} required /></label>
            {/if}
            <div class="ip-strip">ATTACHMENTS</div>
            <PlayersQueue bind:drops={attachments} size={72}>
              {#snippet after()}
                <li>
                  <button
                    type="button"
                    aria-label="Add an attachment"
                    aria-pressed={picking}
                    class="grid size-[72px] cursor-pointer place-items-center rounded border-2 border-dashed border-[#aab2c2] bg-white text-[34px] leading-none text-[#6b7386]"
                    onclick={() => (picking = !picking)}>+</button
                  >
                </li>
              {/snippet}
            </PlayersQueue>
            {#if picking}<PlayersDrops kinds={dropTypes} bind:drops={attachments} />{/if}
            <div class="grid grid-cols-[1fr_1.8fr] gap-3">
              <button type="button" class="ip-btn2" onclick={clear}>Clear</button>
              <button type="submit" class="ip-btn" disabled={send.isPending || !title.trim()}>{kind === 'compensation' ? 'Send compensation' : 'Send mail'}</button>
            </div>
          </form>
        </section>
      </div>
    </div>
  {/snippet}
</CommanderPage>
