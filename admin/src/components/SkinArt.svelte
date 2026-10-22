<script lang="ts">
  // A skin's full art: its painting (/api/painting), or its Live2D model
  // when the skin has one, with the game's no-background variant when there
  // is one, the switches over its top right. Expands to a full-screen
  // dialog. children is drawn over the art (the skin's name, its edit card),
  // tools next to the switches.
  import type { Snippet } from 'svelte'
  import { createQuery } from '@tanstack/svelte-query'
  import { get, keys } from '../lib/query'
  import { createLive2D } from '../lib/live2d.svelte'
  import Live2DView from './Live2DView.svelte'
  import Seg from './Seg.svelte'

  let {
    painting,
    name = '',
    class: className = 'h-[420px]',
    children,
    tools,
  }: {
    // the skin's painting, e.g. biaoqiang_4 (ship_skin_template.painting)
    painting: string
    name?: string
    class?: string
    children?: Snippet
    tools?: Snippet
  } = $props()

  interface Variants {
    painting: string
    variants: { name: string; suffix: string }[]
  }

  const variants = createQuery(() => ({
    queryKey: keys.catalog('painting', painting),
    queryFn: get<Variants>(`/painting/${painting}/variants`),
    staleTime: Infinity,
  }))
  const live2d = createLive2D()

  const hasLive2D = $derived(live2d.has(painting))
  const noBackground = $derived(variants.data?.variants.find((v) => v.name === `${painting}_n`)?.name)
  // Live2D is shown when the skin has a model, unless unchecked
  let useLive2D = $state(true)
  const mode = $derived(hasLive2D && useLive2D ? 'live2d' : 'painting')
  let bare = $state(false)
  let loaded = $state(false)
  let failed = $state(false)
  let dialog = $state<HTMLDialogElement>()
  let expanded = $state(false)

  // a new skin starts on its Live2D model, or its painting with background
  $effect(() => {
    void painting
    useLive2D = true
    bare = false
  })
  const shown = $derived(bare && noBackground ? noBackground : painting)
  $effect(() => {
    void shown
    loaded = false
    failed = false
  })

  function expand() {
    expanded = true
    dialog?.showModal()
  }
</script>

{#snippet art(big: boolean)}
  <div class="relative size-full">{@render artInner(big)}</div>
{/snippet}

{#snippet artInner(big: boolean)}
  {#if mode === 'live2d'}
    {#key painting}<Live2DView name={painting} />{/key}
  {:else}
    {#if !loaded && !failed}<span class="absolute inset-0 grid place-items-center text-base text-dim">Loading the painting…</span>{/if}
    {#if failed}<span class="absolute inset-0 grid place-items-center text-base text-bad">No painting</span>{/if}
    <img
      src="/api/painting/{shown}.png"
      alt={name}
      class={['size-full object-contain transition-opacity', loaded ? 'opacity-100' : 'opacity-0', big && 'cursor-zoom-out']}
      onload={() => (loaded = true)}
      onerror={() => (failed = true)}
    />
  {/if}
{/snippet}

{#snippet controls()}
  <div class="flex flex-wrap items-center gap-2">
    {#if hasLive2D}
      <label class="flex h-12 cursor-pointer items-center gap-2.5 rounded-3xl bg-[rgb(30_33_40/0.88)] px-4 text-base text-white">
        <input type="checkbox" class="size-5 accent-cyan" bind:checked={useLive2D} />
        Live2D
      </label>
    {/if}
    {#if noBackground && mode === 'painting'}
      <Seg
        label="Background"
        bind:value={bare}
        options={[
          { value: false, label: 'Background' },
          { value: true, label: 'None' },
        ]}
      />
    {/if}
  </div>
{/snippet}

<div class={['relative overflow-hidden rounded bg-[radial-gradient(ellipse_at_42%_38%,#3a4670_0%,#222a44_45%,#121624_100%)] shadow-[0_8px_24px_rgb(0_0_0/0.35)]', className]}>
  <div aria-hidden="true" class="absolute inset-0 bg-[repeating-linear-gradient(62deg,transparent_0_220px,rgb(140_180_255/0.05)_220px_222px)]"></div>
  <div class="absolute inset-0 p-4 drop-shadow-[0_12px_24px_rgb(0_0_0/0.45)]">{@render art(false)}</div>
  {@render children?.()}
  <div class="absolute top-5 right-5 flex flex-wrap justify-end gap-2.5">
    {#if hasLive2D || noBackground}{@render controls()}{/if}
    {@render tools?.()}
    <button type="button" class="grid size-12 cursor-pointer place-items-center rounded-full border-0 bg-[rgb(30_33_40/0.88)] text-white hover:bg-[rgb(50_54_64/0.95)]" aria-label="Show {name} full screen" onclick={expand}>
      <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[22px]" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5" /></svg>
    </button>
  </div>
</div>

<dialog
  bind:this={dialog}
  aria-label={name}
  class="m-auto h-[94vh] w-[94vw] max-w-none overflow-hidden rounded border-0 bg-transparent p-0 text-white backdrop:bg-[rgb(8_10_18/0.85)]"
  onclose={() => (expanded = false)}
  onclick={(event) => event.target === dialog && dialog?.close()}
>
  {#if expanded}
    <div class="flex h-full flex-col gap-3 bg-[radial-gradient(ellipse_at_50%_35%,#2c3550,#10141f)] p-4">
      <div class="flex items-center gap-3">
        <h2 class="m-0 rounded-[18px] bg-pill px-7 py-1 text-xl">{name}</h2>
        {@render controls()}
        <button type="button" class="ml-auto grid size-10 cursor-pointer place-items-center rounded-full border-0 bg-white/10 text-white hover:bg-white/20" aria-label="Close" onclick={() => dialog?.close()}>
          <svg aria-hidden="true" viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18" /></svg>
        </button>
      </div>
      <div class="relative min-h-0 flex-1">{@render art(true)}</div>
    </div>
  {/if}
</dialog>
