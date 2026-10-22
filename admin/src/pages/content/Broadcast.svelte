<script lang="ts">
  import { createQueries, createQuery, keepPreviousData } from '@tanstack/svelte-query'
  import { createDebouncedValue } from '@tanstack/svelte-pacer'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Seg from '../../components/Seg.svelte'
  import Icon from '../../components/Icon.svelte'
  import Stepper from '../../components/Stepper.svelte'
  import Picker, { type PickerItem } from '../../components/Picker.svelte'
  import ContentRewards from '../../components/ContentRewards.svelte'
  import { api } from '../../lib/api'
  import { createWrite, rosterQuery } from '../../lib/query'
  import { day, stamp } from '../../lib/format'
  import { shipFramed } from '../../lib/players'
  import { contentKeys, plainDrops, type Audience, type Reward } from '../../lib/content'
  import { bounded } from '../../lib/bounded'

  type Kind = Audience['kind']

  let kind = $state<Kind>('everyone')
  let levelMin = $state(1)
  let levelMax = $state(125)
  let registeredBefore = $state(new Date().toISOString().slice(0, 10))
  let picked = $state<number[]>([])
  let adding = $state<number | null>(null)

  let compensation = $state(false)
  let title = $state('')
  let sender = $state('')
  let content = $state('')
  let rewards = $state<Reward[]>([])
  let hours = $state(72)

  const kinds: { kind: Kind; label: string }[] = [
    { kind: 'everyone', label: 'Everyone' },
    { kind: 'online', label: 'Online now' },
    { kind: 'level', label: 'Level range' },
    { kind: 'registered', label: 'Registered before a date' },
    { kind: 'picked', label: 'Pick commanders' },
  ]

  const roster = createQuery(rosterQuery)
  const commanders = $derived(roster.data ?? [])

  const audience = (k: Kind): Audience => ({
    kind: k,
    level_min: Number(levelMin) || 0,
    level_max: Number(levelMax) || 0,
    registered_before: registeredBefore,
    ids: $state.snapshot(picked),
  })

  // how many each audience reaches, asked once the fields settle
  const [audiences] = createDebouncedValue(() => kinds.map((k) => audience(k.kind)), { wait: 250 })
  const reaches = createQueries(() => ({
    queries: audiences().map((a) => ({
      queryKey: contentKeys.audience(a),
      queryFn: () => api<{ count: number; online: number }>('POST', '/broadcast/audience', a),
      placeholderData: keepPreviousData,
    })),
  }))
  const counts = $derived(Object.fromEntries(kinds.map((k, i) => [k.kind, reaches[i]?.data])) as Partial<Record<Kind, { count: number; online: number }>>)

  const reach = $derived(counts[kind])
  const candidates = $derived<PickerItem[]>(
    commanders
      .filter((c) => !picked.includes(c.id))
      .map((c) => ({ id: c.id, name: c.name, icon: c.icon, rarity: c.rarity, ship: true, sub: `Lv.${c.level}` })),
  )
  const pickedRows = $derived(picked.map((id) => commanders.find((c) => c.id === id)).filter((c) => c !== undefined))
  const until = $derived(new Date(Date.now() + hours * 3600 * 1000))
  const today = new Date()

  function pick(c: PickerItem) {
    picked.push(c.id)
    adding = null
  }

  function clear() {
    title = sender = content = ''
    rewards = []
    compensation = false
    hours = 72
    picked = []
    kind = 'everyone'
  }

  // every recipient's mailbox changed: every commander screen refetches
  const sending = createWrite(
    (mail: object) => api<{ sent: number }>('POST', '/broadcast', mail),
    [['commander'], ['overview']],
    { success: ({ sent }) => `Sent to ${sent} commander${sent === 1 ? '' : 's'}`, onSuccess: () => clear() },
  )

  function send() {
    const n = reach?.count ?? 0
    if (!confirm(`Send "${title}" to ${n} commander${n === 1 ? '' : 's'}?`)) return
    sending.mutate({
      audience: audience(kind),
      title,
      sender,
      content,
      attachments: plainDrops(rewards),
      expire_hours: compensation ? hours : 0,
    })
  }
</script>

<Page title="Broadcast" sub="MAIL" back="/">
  <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <section class="ip-card min-w-0 flex-[1_1_760px]" aria-label="Mail everyone">
      <div class="ip-card-head items-center gap-4">
        <h2>Mail everyone</h2>
        <span class="text-sm text-soft">to <span class="num text-lg">{reach?.count ?? '…'}</span> commanders</span>
      </div>
      <div class="ip-card-body">
        <div class="grid grid-cols-[minmax(0,370px)_minmax(0,1fr)] gap-7 max-[1100px]:grid-cols-1">
          <div class="flex min-w-0 flex-col gap-3.5">
            <div class="ip-strip">AUDIENCE</div>
            <div role="group" aria-label="Audience" class="flex flex-col gap-2">
              {#each kinds as k (k.kind)}
                <button type="button" class="ip-opt flex items-center justify-between text-left" aria-pressed={kind === k.kind} onclick={() => (kind = k.kind)}>
                  {k.label}
                  <span class="num text-2xl">{counts[k.kind]?.count ?? ''}</span>
                </button>
              {/each}
            </div>
            {#if kind === 'level'}
              <div class="grid grid-cols-2 gap-3">
                <label class="ip-lab min-w-0">Level from
                  <input class="ip-field num w-full min-w-0 text-2xl" type="number" min="1" bind:value={levelMin} use:bounded={{ min: 1 }} />
                </label>
                <label class="ip-lab min-w-0">to
                  <input class="ip-field num w-full min-w-0 text-2xl" type="number" min="1" bind:value={levelMax} use:bounded={{ min: levelMin || 1 }} />
                </label>
              </div>
            {:else if kind === 'registered'}
              <label class="ip-lab">Registered before
                <input class="ip-field num text-2xl" type="date" bind:value={registeredBefore} />
              </label>
            {:else if kind === 'picked'}
              <Picker items={candidates} bind:value={adding} label="Add a commander" placeholder="Name or UID" loading={roster.isPending} onselect={pick} />
              {#if pickedRows.length}
                <ul class="m-0 flex list-none flex-wrap gap-2 p-0">
                  {#each pickedRows as c (c.id)}
                    <li class="flex items-center gap-2 rounded bg-white py-1.5 pr-1.5 pl-1.5 text-[17px] shadow-[inset_0_0_0_1px_#d6dae1]">
                      <Icon icon={c.icon} rarity={c.rarity} ship size={30} name={c.name} />
                      {c.name}<span class="num text-ink-soft">#{c.id}</span>
                      <button type="button" aria-label="Remove {c.name}" class="grid size-7 cursor-pointer place-items-center rounded-full border-0 bg-transparent text-ink-soft hover:bg-[#eef0f4]" onclick={() => (picked = picked.filter((id) => id !== c.id))}>
                        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18" /></svg>
                      </button>
                    </li>
                  {/each}
                </ul>
              {/if}
            {/if}
            <div class="flex items-baseline gap-2.5 rounded bg-navy px-3.5 py-3 text-white">
              Reaches <span class="num text-[34px] leading-none">{reach?.count ?? 0}</span> commanders
              <span class="ml-auto text-sm text-soft"><span class="num text-lg text-white">{reach?.online ?? 0}</span> online</span>
            </div>
          </div>

          <div class="flex min-w-0 flex-col gap-3.5">
            <div class="ip-strip">MAIL</div>
            <div class="flex items-center justify-between gap-3 text-lg">
              Send as a compensation
              <Seg options={[{ value: true, label: 'On' }, { value: false, label: 'Off' }]} bind:value={compensation} label="Send as a compensation" boxes />
            </div>
            <div class="grid grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)] gap-3.5">
              <label class="ip-lab min-w-0">Title
                <input class="ip-field w-full min-w-0" bind:value={title} />
              </label>
              <label class="ip-lab min-w-0">From
                <input class="ip-field w-full min-w-0" placeholder="Headquarters" bind:value={sender} />
              </label>
            </div>
            <label class="ip-lab">Text
              <textarea class="ip-field" rows="3" bind:value={content}></textarea>
            </label>
            <div class="ip-strip">ATTACHMENTS</div>
            <ContentRewards bind:rewards />
            {#if compensation}
              <div class="ip-strip">EXPIRY</div>
              <div class="flex flex-wrap items-center gap-3 text-ink-soft">
                Expires after
                <div class="w-[190px]"><Stepper bind:value={hours} min={1} max={24 * 365} label="Hours" /></div>
                <span class="text-ink">hours, until <span class="num text-2xl text-[#1f8ad0]">{stamp(until.getTime() / 1000)}</span></span>
              </div>
            {/if}
          </div>
        </div>
        <div class="grid grid-cols-[minmax(0,370px)_minmax(0,1fr)] gap-3 max-[1100px]:grid-cols-2">
          <button type="button" class="ip-btn2" onclick={clear}>Clear</button>
          <button type="button" class="ip-btn" disabled={sending.isPending || !title.trim() || !reach?.count} onclick={send}>Send to {reach?.count ?? 0} commander{reach?.count === 1 ? '' : 's'}</button>
        </div>
      </div>
    </section>

    <Panel title="In the client" sub="PREVIEW" class="min-w-[320px] flex-[0_1_380px]">
      <div class="flex flex-col gap-2 p-3">
        <span class="text-sm text-soft">{compensation ? 'Compensation box' : 'Mailbox'}</span>
        <div class="ip-row flex items-center gap-4 p-3">
          {#if rewards.length}
            <Icon icon={rewards[0].icon} rarity={rewards[0].rarity} ship={shipFramed(rewards[0].type)} count={rewards[0].count} size={72} name={rewards[0].name} />
          {:else}
            <Icon size={72} />
          {/if}
          <div class="flex min-w-0 flex-col gap-1">
            <span class="text-xl leading-tight break-words">{title || 'Title'}</span>
            <span class="text-sm text-soft">{sender || 'Headquarters'}</span>
            <span class="num text-xl text-soft">{day(today.getTime() / 1000)}</span>
          </div>
        </div>
        {#if content}
          <p class="m-0 px-1 text-[15px] leading-normal whitespace-pre-line text-[#e5edff]">{content}</p>
        {/if}
      </div>
    </Panel>
  </div>
</Page>
