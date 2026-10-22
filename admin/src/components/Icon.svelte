<script lang="ts">
  // A thing's icon as the client draws it: its picture from the game's
  // bundles (/api/assets) on a rarity frame, with a count badge.
  import { tier } from '../lib/format'

  let {
    icon = '',
    rarity = 1,
    ship = false,
    count,
    size = 64,
    name = '',
    label = true,
  }: {
    // the bundle path (an API row's icon), empty for a frame only
    icon?: string
    rarity?: number
    // ships' rarity starts at 2
    ship?: boolean
    count?: number | string
    size?: number
    name?: string
    // write the name in the frame when there's no picture (off where the
    // name is printed next to it already)
    label?: boolean
  } = $props()

  let broken = $state(false)
  const frames = ['', 'from-[#7d858f] to-[#b9c0c8]', 'from-[#2f6fb8] to-[#78b6f0]', 'from-[#6c3eb0] to-[#b88ae8]', 'from-[#c07a12] to-[#f6cf6a]', 'from-[#c0397a] via-[#7a7ae8] to-[#3cc8c8]']
</script>

<div
  class={['relative grid shrink-0 place-items-center rounded-[4px] bg-linear-to-b p-[3px] shadow-[0_2px_6px_rgb(0_0_0/0.3)]', frames[tier(rarity, ship)]]}
  style:width="{size}px"
  style:height="{size}px"
  title={name || undefined}
>
  <div class="grid size-full place-items-center overflow-hidden rounded-[2px] bg-[radial-gradient(circle,rgb(255_255_255/0.25),rgb(255_255_255/0.08))]">
    {#if icon && !broken}
      <img src="/api/assets/{icon}.png" alt={name} loading="lazy" class="size-full object-contain" onerror={() => (broken = true)} />
    {:else if name && label}
      <span class="px-1 text-center text-[10px] leading-tight text-white/80">{name}</span>
    {/if}
  </div>
  {#if count !== undefined}
    <span class="num absolute -right-1.5 bottom-1 h-6 min-w-7 rounded-xl bg-strip px-2 text-center text-lg leading-6 text-white">{count}</span>
  {/if}
</div>
