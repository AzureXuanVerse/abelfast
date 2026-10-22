<script lang="ts">
  // Drops picked to grant or attach, as icons with a remove button.
  import type { Snippet } from 'svelte'
  import { drop, shipFramed, type Queued } from '../lib/players'
  import Icon from './Icon.svelte'

  let { drops = $bindable(), size = 76, after }: { drops: Queued[]; size?: number; after?: Snippet } = $props()

  const single = new Set<number>([drop.skin, drop.iconFrame, drop.chatFrame])
  const badge = (q: Queued) => (q.type === drop.skinTimed ? `${Math.round(q.count / 86400)}d` : single.has(q.type) ? undefined : q.count)
</script>

<ul class="m-0 flex list-none flex-wrap gap-4 p-0">
  {#each drops as q, i (q.type + ':' + q.id)}
    <li class="flex w-[84px] flex-col items-center gap-1.5">
      <div class="relative">
        <Icon icon={q.icon} rarity={q.rarity} ship={shipFramed(q.type)} count={badge(q)} {size} name={q.name} />
        <button type="button" aria-label="Remove {q.name}" class="absolute -top-2 -right-2 grid size-[26px] cursor-pointer place-items-center rounded-full border-2 border-white bg-[#6b7386] p-0 text-[15px] leading-none text-white" onclick={() => drops.splice(i, 1)}>×</button>
      </div>
      <span class="line-clamp-2 text-center text-[13px] leading-tight">{q.name}</span>
    </li>
  {/each}
  {@render after?.()}
</ul>
