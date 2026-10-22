<script lang="ts">
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import Card from '../../components/Card.svelte'
  import Seg from '../../components/Seg.svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import { api, type Notice } from '../../lib/api'
  import { createWrite, get } from '../../lib/query'
  import { contentKeys } from '../../lib/content'
  import { bounded } from '../../lib/bounded'

  type Kind = Notice['kind']

  let kind = $state<Kind>('board')
  const query = createQuery(() => ({ queryKey: contentKeys.notices, queryFn: get<Notice[]>('/notices') }))
  const notices = $derived(query.data ?? [])

  // the form: a new notice, or the one being edited
  let editing = $state<number | null>(null)
  let tag = $state(1)
  let button = $state('')
  let title = $state('')
  let content = $state('')
  let priority = $state(0)
  let needLevel = $state(0)

  const tags = [
    { value: 1, label: 'Event' },
    { value: 2, label: 'System' },
    { value: 3, label: 'News' },
  ]
  const tagColors: Record<number, string> = { 1: 'bg-[#f0a93a] text-[#3a2a08]', 2: 'bg-[#3d7ff0] text-white', 3: 'bg-[#3aa776] text-white' }
  const kinds: { kind: Kind; label: string; icon: string }[] = [
    { kind: 'board', label: 'Board', icon: 'M6 3h12a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2zM8 8h8M8 12h8M8 16h5' },
    { kind: 'gateway', label: 'Popups', icon: 'M5 5h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2zM3 9h18M7 13h6' },
  ]

  const shown = $derived(notices.filter((n) => n.kind === kind))


  function clear() {
    editing = null
    tag = 1
    button = title = content = ''
    priority = needLevel = 0
  }

  function edit(notice: Notice) {
    editing = notice.id
    tag = notice.tag || 1
    button = notice.button === notice.title ? '' : notice.button
    title = notice.title
    content = notice.content
    priority = notice.priority
    needLevel = notice.need_level
  }

  function pick(next: Kind) {
    kind = next
    clear()
  }

  const saving = createWrite(
    ({ id, body }: { id: number | null; body: object }) => (id === null ? api('POST', '/notices', body) : api('PUT', `/notices/${id}`, body)),
    [contentKeys.notices],
    { success: (_, { id }) => (id === null ? 'Notice published' : 'Notice saved'), onSuccess: () => clear() },
  )

  function save() {
    const body =
      kind === 'board'
        ? { kind, tag, button, title, content, priority: Number(priority) || 0, need_level: Number(needLevel) || 0 }
        : { kind, title, content }
    saving.mutate({ id: editing, body })
  }

  const removal = createWrite((notice: Notice) => api('DELETE', `/notices/${notice.id}`), [contentKeys.notices], {
    success: 'Notice deleted',
    onSuccess: (_, notice) => {
      if (editing === notice.id) clear()
    },
  })

  function remove(notice: Notice) {
    if (!confirm(`Delete "${headline(notice)}"?`)) return
    removal.mutate(notice)
  }

  // "list title&page title": the board lists the first part
  const headline = (n: Notice) => n.title.split('&')[0]
  // the content's rich text, without its tags
  const plain = (text: string) => text.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
</script>

<Page title="Notices" sub="BULLETIN" back="/">
  <div class="relative flex flex-wrap items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <nav aria-label="Notice kind" class="ip-side w-[150px] py-2">
      {#each kinds as k, i (k.kind)}
        <button type="button" class={['ip-tab cursor-pointer border-0 bg-transparent font-game font-bold', i === kinds.length - 1 && 'border-b-0']} aria-current={kind === k.kind ? 'page' : undefined} onclick={() => pick(k.kind)}>
          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[34px]" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d={k.icon} /></svg>
          {k.label}
          <span class="num text-lg text-white/70">{notices.filter((n) => n.kind === k.kind).length}</span>
        </button>
      {/each}
    </nav>

    <Panel title={kind === 'board' ? 'Bulletin board' : 'Login popups'} sub="IN THE GAME NOW" class="flex min-w-0 flex-[1_1_560px] flex-col">
      {#snippet actions()}<span class="text-sm text-soft">Highest priority first</span>{/snippet}
      <div class="flex flex-col gap-2.5 p-3">
        {#each shown as notice (notice.id)}
          <article class={['ip-row flex items-stretch overflow-hidden', editing === notice.id && 'ip-sel']}>
            {#if notice.kind === 'board'}
              <span class={['flex w-[34px] flex-none items-center justify-center text-xs tracking-[0.2em] uppercase [writing-mode:vertical-rl]', tagColors[notice.tag] ?? 'bg-strip']}>{tags.find((t) => t.value === notice.tag)?.label ?? 'Tag ' + notice.tag}</span>
            {/if}
            <div class="flex min-w-0 flex-1 flex-col gap-2 px-5 py-3.5">
              <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1.5">
                <h3 class="m-0 min-w-0 text-[22px] break-words">{headline(notice)}</h3>
                {#if notice.time}<span class="num ml-auto text-[22px] text-soft">{notice.time}</span>{/if}
              </div>
              <p class="m-0 line-clamp-3 max-w-[64ch] text-[15px] leading-normal text-[#e5edff]">{plain(notice.content)}</p>
              <div class="flex flex-wrap items-center gap-3">
                {#if notice.kind === 'board'}
                  <span class="ip-sub">PRIORITY <span class="num text-base tracking-normal text-white">{notice.priority}</span></span>
                  {#if notice.need_level}<span class="ip-sub">FROM LEVEL <span class="num text-base tracking-normal text-white">{notice.need_level}</span></span>{/if}
                {/if}
                <span class="ml-auto flex gap-2.5">
                  <button type="button" class="ip-btn2 h-10 px-[18px] text-[17px]" aria-label="Edit {headline(notice)}" onclick={() => edit(notice)}>Edit</button>
                  <button type="button" class="ip-danger h-10 px-[18px] text-[17px]" aria-label="Delete {headline(notice)}" disabled={removal.isPending} onclick={() => remove(notice)}>Delete</button>
                </span>
              </div>
            </div>
          </article>
        {:else}
          {#if query.isSuccess}<p class="m-0 px-2 py-6 text-center text-soft">No notice</p>{/if}
        {/each}
        {#if query.error}<p class="m-0 px-2 py-6 text-center text-bad">{query.error.message}</p>{/if}
      </div>
    </Panel>

    <Card title={editing === null ? 'New notice' : 'Edit notice'} class="min-w-[340px] flex-[0_1_420px]">
      {#if kind === 'board'}
        <div class="ip-strip">TAB</div>
        <Seg options={tags} bind:value={tag} label="Tab" boxes columns={3} />
        <label class="ip-lab">Tab label
          <input class="ip-field" placeholder="Same as the title" bind:value={button} />
        </label>
      {/if}
      <div class="ip-strip">TEXT</div>
      <label class="ip-lab">Title
        <input class="ip-field" bind:value={title} />
      </label>
      <label class="ip-lab">Text
        <textarea class="ip-field" rows="4" bind:value={content}></textarea>
      </label>
      {#if kind === 'board'}
        <div class="ip-strip">ORDER</div>
        <div class="grid grid-cols-2 gap-3.5">
          <label class="ip-lab min-w-0">Priority
            <input class="ip-field num w-full min-w-0 text-2xl" type="number" min="0" bind:value={priority} use:bounded={{ min: 0 }} />
          </label>
          <label class="ip-lab min-w-0">From level
            <input class="ip-field num w-full min-w-0 text-2xl" type="number" min="0" bind:value={needLevel} use:bounded={{ min: 0 }} />
          </label>
        </div>
      {/if}
      <div class="grid grid-cols-2 gap-3">
        <button type="button" class="ip-btn2" onclick={clear}>{editing === null ? 'Clear' : 'Cancel'}</button>
        <button type="button" class="ip-btn" disabled={saving.isPending || !title.trim()} onclick={save}>{editing === null ? 'Publish' : 'Save'}</button>
      </div>
    </Card>
  </div>
</Page>
