<script module lang="ts">
  let message = $state<{ text: string; error: boolean } | null>(null)
  let timer: ReturnType<typeof setTimeout>

  export function toast(text: string, error = false) {
    message = { text, error }
    clearTimeout(timer)
    timer = setTimeout(() => (message = null), error ? 6000 : 3000)
  }
</script>

{#if message}
  <div role="status" class={['fixed right-5 bottom-5 z-50 max-w-sm rounded px-5 py-3 text-base text-white shadow-[0_8px_24px_rgb(0_0_0/0.35)]', message.error ? 'bg-red' : 'bg-navy']}>
    {message.text}
  </div>
{/if}
