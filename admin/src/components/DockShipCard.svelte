<script lang="ts">
  // A ship as the dock draws it: its card art on a rarity frame, the level
  // and lock on top, the fleet badge, name and stars at the bottom.
  import { shipFrames } from '../lib/dock'

  let {
    icon = '',
    name,
    rarity,
    level,
    star,
    maxStar,
    locked = false,
    married = false,
    fleet = 0,
    large = false,
  }: {
    icon?: string
    name: string
    rarity: number
    level: number
    star: number
    maxStar: number
    locked?: boolean
    married?: boolean
    fleet?: number
    large?: boolean
  } = $props()

  let broken = $state(false)
</script>

<div class={['relative block aspect-[216/300] overflow-hidden border-[3px] bg-[#1b2238] text-white shadow-[0_4px_10px_rgb(0_0_0/0.3)]', shipFrames[rarity] ?? shipFrames[2]]}>
  {#if icon && !broken}
    <img src="/api/assets/{icon}.png" alt="" loading="lazy" class="absolute inset-0 size-full object-cover object-top" onerror={() => (broken = true)} />
  {/if}
  <div aria-hidden="true" class="absolute inset-0 bg-[linear-gradient(180deg,rgb(0_0_0/0.45)_0%,transparent_18%,transparent_62%,rgb(0_0_0/0.75)_100%)]"></div>
  <span class={['num absolute top-1 right-2.5 [text-shadow:0_0_4px_#000]', large ? 'text-[38px]' : 'text-[28px]']}>Lv.{level}</span>
  {#if locked}
    <svg role="img" aria-label="Locked" viewBox="0 0 24 24" class={['absolute right-3 fill-white', large ? 'top-[52px] size-6' : 'top-10 size-5']}><path d="M7 10V8a5 5 0 0 1 10 0v2h1v11H6V10h1Zm2 0h6V8a3 3 0 0 0-6 0v2Z" /></svg>
  {/if}
  {#if married}
    <svg role="img" aria-label="Oath sworn" viewBox="0 0 24 24" class={['absolute top-2.5 left-2.5 fill-[#ff7aa8] stroke-white stroke-[1.2]', large ? 'size-7' : 'size-5']}><path d="M12 20s-7-4.4-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 10c0 5.6-7 10-7 10Z" /></svg>
  {/if}
  {#if fleet}
    <span class={['absolute left-2 flex h-6 items-center gap-1 rounded-xl bg-cyan px-2 text-[11px] tracking-[0.06em]', large ? 'bottom-[66px]' : 'bottom-[54px]']}>
      {fleet >= 11 ? 'SUB' : 'FLEET'} <span class="num text-lg">{fleet >= 11 ? fleet - 10 : fleet}</span>
    </span>
  {/if}
  <span class={['absolute inset-x-0 truncate px-1 text-center [text-shadow:0_0_4px_#000,0_0_4px_#000]', large ? 'bottom-7 text-[26px]' : 'bottom-6 text-xl']}>{name}</span>
  <span aria-label="{star} of {maxStar} stars" class={['absolute inset-x-0 text-center tracking-[1px]', large ? 'bottom-1.5 text-lg' : 'bottom-1 text-sm']}>
    <span class="text-[#ffd84a] [text-shadow:0_0_3px_#7a5600]">{'★'.repeat(star)}</span><span class="text-white/25">{'★'.repeat(Math.max(0, maxStar - star))}</span>
  </span>
</div>
