<script lang="ts">
  // Rewards picked from the catalog on a light card: their icons, a kind,
  // a picker of that kind's things with their thumbnails, a count and Add.
  import Icon from './Icon.svelte'
  import Stepper from './Stepper.svelte'
  import CatalogSelect from './CatalogSelect.svelte'
  import ShipSelect from './ShipSelect.svelte'
  import SkinSelect from './SkinSelect.svelte'
  import type { PickerItem } from './Picker.svelte'
  import { dropTypes } from '../lib/api'
  import type { Reward } from '../lib/content'

  let { rewards = $bindable([]), max = 0 }: { rewards: Reward[]; max?: number } = $props()

  const ship = 4
  const skin = 7

  let type = $state(2)
  let value = $state<number | null>(null)
  let picked = $state<PickerItem | null>(null)
  let count = $state(1)

  function kind(next: number) {
    type = next
    value = null
    picked = null
  }

  function select(item: PickerItem) {
    picked = item
  }

  function add() {
    if (!picked || count < 1) return
    const item = picked
    const same = rewards.find((r) => r.type === type && r.id === item.id)
    if (same) same.count += count
    else rewards.push({ type, id: item.id, count, name: item.name, icon: item.icon, rarity: item.rarity })
    value = null
    picked = null
    count = 1
  }
</script>

<div class="flex flex-wrap gap-3">
  {#each rewards as reward, i (reward.type + ':' + reward.id)}
    <div class="relative flex w-[78px] flex-col items-center gap-1">
      <Icon icon={reward.icon} rarity={reward.rarity} ship={reward.type === ship} count={reward.count} size={72} name={reward.name} />
      <span class="line-clamp-2 text-center text-[13px] leading-tight text-ink">{reward.name ?? reward.id}</span>
      <button type="button" aria-label="Remove {reward.name}" class="absolute -top-2 -right-1 grid size-6 cursor-pointer place-items-center rounded-full border-2 border-white bg-[#6b7280] p-0 text-white" onclick={() => rewards.splice(i, 1)}>
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-3" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18" /></svg>
      </button>
    </div>
  {/each}
</div>
{#if !max || rewards.length < max}
  <div class="flex gap-2">
    <select class="ip-field w-[130px] flex-none" value={type} aria-label="Kind" onchange={(e) => kind(Number(e.currentTarget.value))}>
      {#each dropTypes as d (d.type)}<option value={d.type}>{d.label}</option>{/each}
    </select>
    <div class="min-w-0 flex-1">
      {#key type}
        {#if type === ship}
          <ShipSelect bind:value onselect={select} />
        {:else if type === skin}
          <SkinSelect bind:value onselect={select} />
        {:else}
          <CatalogSelect {type} bind:value onselect={select} label="Pick {dropTypes.find((d) => d.type === type)?.label.toLowerCase() ?? 'one'}" />
        {/if}
      {/key}
    </div>
  </div>
  <div class="flex items-center gap-2.5">
    <div class="min-w-0 flex-1"><Stepper bind:value={count} min={1} max={1000000} label="Count" /></div>
    <button type="button" class="ip-btn" disabled={!picked} onclick={add}>Add</button>
  </div>
{/if}
