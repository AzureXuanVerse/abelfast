<script lang="ts">
  // A light card, for forms: the title in a dark pill over a cyan rule.
  import type { Snippet } from 'svelte'

  let { title, class: className = '', children }: { title: string; class?: string; children: Snippet } = $props()
</script>

<section class={['ip-card', className]} aria-label={title}>
  <div class="ip-card-head"><h2>{title}</h2></div>
  <div class="ip-card-body">{@render children()}</div>
</section>
