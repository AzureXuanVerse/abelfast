<script lang="ts">
  // The inventory screens' tabs: the Items and Gear pages.
  import { link } from '../lib/router.svelte'

  let { id, current }: { id: number; current: 'Items' | 'Gear' } = $props()

  const tabs = $derived([
    { label: 'Items', page: `/commanders/${id}/items` },
    { label: 'Gear', page: `/commanders/${id}/gear` },
  ])
</script>

<nav aria-label="Inventory" class="ip-tabs flex-nowrap overflow-x-auto px-2 [&>a]:px-3.5">
  {#each tabs as tab (tab.label)}
    <a href={tab.page} use:link aria-current={tab.label === current ? 'page' : undefined}>{tab.label}</a>
  {/each}
</nav>
