<script lang="ts">
  // A dialog in the Picker's style: a titled bar with Close over a dark
  // body. Opened by show(), closed by Close, Escape or a click outside.
  import type { Snippet } from 'svelte'

  let { title, width = 'w-[min(560px,92vw)]', children }: { title: string; width?: string; children: Snippet } = $props()

  const uid = $props.id()
  let dialog = $state<HTMLDialogElement>()
  let open = $state(false)

  export function show() {
    open = true
    dialog?.showModal()
  }

  export function close() {
    dialog?.close()
  }
</script>

<dialog
  bind:this={dialog}
  aria-labelledby="{uid}-title"
  class={['m-auto max-h-[88vh] overflow-hidden rounded border-0 bg-transparent p-0 text-white shadow-[0_16px_48px_rgb(0_0_0/0.55)] backdrop:bg-[rgb(8_10_18/0.7)]', width]}
  onclose={() => (open = false)}
  onclick={(event) => event.target === dialog && close()}
>
  {#if open}
    <div class="flex max-h-[88vh] flex-col bg-[rgb(20_24_34/0.97)]">
      <div class="flex items-center gap-3 border-b-4 border-cyan bg-[#2b2e35] px-5 py-3">
        <h2 id="{uid}-title" class="m-0 truncate rounded-[18px] bg-pill px-7 py-1 text-xl">{title}</h2>
        <button type="button" class="ml-auto grid size-10 flex-none cursor-pointer place-items-center rounded-full border-0 bg-white/10 text-white hover:bg-white/20" aria-label="Close" onclick={close}>
          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18" /></svg>
        </button>
      </div>
      <div class="min-h-0 overflow-y-auto px-5 py-4">{@render children()}</div>
    </div>
  {/if}
</dialog>
