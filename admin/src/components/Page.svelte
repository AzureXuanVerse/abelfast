<script lang="ts">
  // A screen: the white back button over the navy title pill, the screen's
  // controls on the right and the round home button.
  import type { Snippet } from 'svelte'
  import { link } from '../lib/router.svelte'

  let {
    title,
    sub = '',
    back = '/',
    backLabel = 'Back',
    badge,
    actions,
    children,
  }: {
    title: string
    sub?: string
    // the back button's target, null for none (the dashboard)
    back?: string | null
    backLabel?: string
    // drawn where the back button goes when there is none
    badge?: Snippet
    actions?: Snippet
    children: Snippet
  } = $props()
</script>

<div class="relative min-h-screen">
  <header class="relative flex flex-wrap items-start gap-x-6 gap-y-3.5 pt-[18px] pr-10 pl-12">
    <div class="relative h-[82px]">
      {#if back}
        <a href={back} use:link class="ip-back" aria-label={backLabel}>
          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[34px]" fill="none" stroke="#3a3f4a" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M15 5l-7 7 7 7" /></svg>
        </a>
      {:else if badge}
        <div class="ip-back cursor-default hover:bg-white">{@render badge()}</div>
      {/if}
      <div class={['ip-pill', !back && !badge && 'pl-8']}>
        <h1 class="m-0 text-[32px]">{title}</h1>
        {#if sub}<span class="ip-sub text-soft">{sub}</span>{/if}
      </div>
      {#if back || badge}
        <span aria-hidden="true" class="absolute top-14 left-[186px] text-sm tracking-[3px] text-[#9aa0ab]">•••</span>
      {/if}
    </div>
    <div class="ml-auto flex flex-wrap items-center gap-x-3.5 gap-y-2.5 pt-1">
      {@render actions?.()}
      <a href="/" use:link class="ip-home" aria-label="Home, dashboard">
        <svg aria-hidden="true" viewBox="0 0 24 24" class="size-7" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"><path d="M4 11l8-7 8 7v9h-5v-6H9v6H4z" /></svg>
      </a>
    </div>
  </header>
  {@render children()}
</div>
