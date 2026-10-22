<script lang="ts">
  // A section's sidebar of icon tabs (moderation, content, server screens).
  import { link, route } from '../lib/router.svelte'

  export interface IconTab {
    href: string
    label: string
    // an SVG path, drawn on a 24 x 24 stroke icon
    icon: string
    count?: number | string
    // the red unread marker
    alert?: boolean
  }

  let { tabs, label }: { tabs: IconTab[]; label: string } = $props()
</script>

<nav aria-label={label} class="ip-side max-h-[calc(100vh-32px)] w-[150px] py-2">
  {#each tabs as tab, i (tab.href)}
    <a href={tab.href} use:link class={['ip-tab', i === tabs.length - 1 && 'border-b-0']} aria-current={route.path === tab.href ? 'page' : undefined}>
      {#if tab.alert}<span aria-label="New" class="absolute top-3 right-3 size-3 rotate-45 bg-[#d0634a] shadow-[0_0_0_2px_#f2c3a6]"></span>{/if}
      <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[34px]" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d={tab.icon} /></svg>
      {tab.label}
      {#if tab.count !== undefined}<span class="num text-lg text-white/70">{tab.count}</span>{/if}
    </a>
  {/each}
</nav>
