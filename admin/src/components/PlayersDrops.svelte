<script lang="ts" module>
  export interface DropKind {
    type: number
    label: string
  }
</script>

<script lang="ts">
  // Picks drops into a list: a kind, the thing in that kind's picker (a
  // gear's enhancement, a skin's duration), a count, Add.
  import { createQuery } from '@tanstack/svelte-query'
  import { get } from '../lib/query'
  import { drop, playerKeys, type GearCatalogEntry, type Queued } from '../lib/players'
  import CatalogSelect from './CatalogSelect.svelte'
  import Picker, { type PickerItem } from './Picker.svelte'
  import Seg from './Seg.svelte'
  import ShipSelect from './ShipSelect.svelte'
  import SkinSelect from './SkinSelect.svelte'
  import Stepper from './Stepper.svelte'
  import { toast } from './Toast.svelte'

  let {
    kinds,
    drops = $bindable([]),
    owned,
  }: {
    kinds: DropKind[]
    drops: Queued[]
    // what the commander has, by `${type}:${id}`, tagged in the pickers
    owned?: Set<string>
  } = $props()

  // the kinds /catalog/more lists: gear with its enhanced ids, gear skins,
  // frames, augment modules (/catalog/all doesn't name them)
  const more = new Set<number>([drop.equip, drop.equipSkin, drop.iconFrame, drop.chatFrame, drop.spweapon])
  // granted once, no count
  const single = new Set<number>([drop.skin, drop.iconFrame, drop.chatFrame])

  // svelte-ignore state_referenced_locally
  let type = $state(kinds[0].type)
  let value = $state<number | null>(null)
  let picked = $state<PickerItem | null>(null)
  let count = $state(1)
  let enhance = $state(0)
  let duration = $state<'permanent' | '7' | '30' | 'until'>('permanent')
  let until = $state('')

  const catalog = createQuery(() => ({
    queryKey: playerKeys.more(type),
    queryFn: get<GearCatalogEntry[]>(`/catalog/more?type=${type}`),
    staleTime: Infinity,
    enabled: more.has(type),
  }))
  const entries = $derived(more.has(type) ? (catalog.data ?? []) : [])
  const moreItems = $derived<PickerItem[]>(
    entries.map((entry) => ({
      id: entry.id,
      name: entry.name,
      icon: entry.icon,
      rarity: entry.rarity,
      sub: owned?.has(`${type}:${entry.id}`) ? 'Owned' : undefined,
    })),
  )
  const levels = $derived(type === drop.equip ? (entries.find((entry) => entry.id === value)?.levels ?? []) : [])
  const skins = $derived(new Set([...(owned ?? [])].filter((key) => key.startsWith(`${drop.skin}:`)).map((key) => Number(key.split(':')[1]))))

  function pickKind(next: number) {
    type = next
    value = null
    picked = null
    enhance = 0
  }

  function select(item: PickerItem) {
    picked = item
    enhance = 0
  }

  function add() {
    if (!picked) return
    let entry: Queued = { type, id: picked.id, count, name: picked.name, icon: picked.icon, rarity: picked.rarity }
    if (type === drop.equip && levels.length) {
      entry.id = levels[Math.min(enhance, levels.length - 1)]
      if (enhance) entry.name = `${picked.name} +${enhance}`
    }
    if (single.has(type)) entry.count = 1
    if (type === drop.skin && duration !== 'permanent') {
      const end = duration === 'until' ? new Date(until).getTime() / 1000 : Date.now() / 1000 + Number(duration) * 86400
      const seconds = Math.round(end - Date.now() / 1000)
      if (!(seconds > 0)) return toast('Pick a time to come', true)
      entry = { ...entry, type: drop.skinTimed, count: seconds }
    }
    const same = drops.find((q) => q.type === entry.type && q.id === entry.id)
    if (same && !single.has(entry.type) && entry.type !== drop.skinTimed) same.count += entry.count
    else if (!same) drops.push(entry)
  }
</script>

{#if kinds.length > 1 && kinds.length <= 4}
  <Seg boxes columns={kinds.length === 4 ? 2 : kinds.length} label="Kind" value={type} onchange={pickKind} options={kinds.map((k) => ({ value: k.type, label: k.label }))} />
{:else if kinds.length > 1}
  <select class="ip-field w-full" aria-label="Kind" value={type} onchange={(e) => pickKind(Number(e.currentTarget.value))}>
    {#each kinds as k (k.type)}<option value={k.type}>{k.label}</option>{/each}
  </select>
{/if}
{#key type}
  {#if type === drop.ship}
    <ShipSelect bind:value onselect={select} />
  {:else if type === drop.skin}
    <SkinSelect bind:value owned={skins} onselect={select} />
  {:else if more.has(type)}
    <Picker items={moreItems} bind:value label="Pick" placeholder="Name or id" loading={catalog.isPending} onselect={select} />
  {:else}
    <CatalogSelect {type} bind:value onselect={select} />
  {/if}
{/key}
{#if type === drop.equip && levels.length > 1}
  <label class="ip-lab">
    Enhance
    <select class="ip-field" bind:value={enhance}>
      {#each levels as _, i (i)}<option value={i}>+{i}</option>{/each}
    </select>
  </label>
{/if}
{#if type === drop.skin}
  <div class="ip-strip">DURATION</div>
  <Seg
    boxes
    columns={2}
    label="Duration"
    bind:value={duration}
    options={[
      { value: 'permanent', label: 'Permanent' },
      { value: '7', label: '7 days' },
      { value: '30', label: '30 days' },
      { value: 'until', label: 'Until…' },
    ]}
  />
  {#if duration === 'until'}<input class="ip-field num text-xl" type="datetime-local" aria-label="Until" bind:value={until} />{/if}
{/if}
<div class="flex items-center gap-2">
  {#if !single.has(type)}
    <div class="min-w-0 flex-1"><Stepper label="Count" bind:value={count} min={1} /></div>
  {/if}
  <button type="button" class={['ip-btn', single.has(type) ? 'flex-1' : 'w-[88px] px-0']} disabled={!picked} onclick={add}>Add</button>
</div>
