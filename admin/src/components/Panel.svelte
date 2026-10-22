<script lang="ts">
  // A dark panel with its light title strip: the title, a short tag, and the
  // panel's controls pushed to the right.
  import type { Snippet } from 'svelte'

  let {
    title,
    sub = '',
    class: className = '',
    actions,
    children,
  }: { title: string; sub?: string; class?: string; actions?: Snippet; children: Snippet } = $props()
</script>

<section class={['ip-panel', className]} aria-label={title}>
  <div class="ip-head items-center">
    <h2>{title}</h2>
    {#if sub}<span class="ip-sub">{sub}</span>{/if}
    {#if actions}<div class="ml-auto flex flex-wrap items-center gap-2.5">{@render actions()}</div>{/if}
  </div>
  {@render children()}
</section>
