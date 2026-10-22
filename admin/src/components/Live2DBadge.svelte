<script lang="ts">
  // The tag of a skin with a Live2D model, over the corner of its art.
  let { class: className = 'top-1 right-1' }: { class?: string } = $props()
</script>

<span aria-label="Live2D" class={['ip-tag absolute bg-[#8a5cf5] text-white shadow-[0_1px_4px_rgb(0_0_0/0.4)]', className]}>L2D</span>
