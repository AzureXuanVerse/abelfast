<script lang="ts" module>
  // Live2D's Cubism Core is proprietary and not part of belfast: it is
  // loaded from Live2D's site the first time a model is shown.
  const coreURL = 'https://cubism.live2d.com/sdk-web/cubismcore/live2dcubismcore.min.js'
  let core: Promise<void> | undefined

  function loadCore(): Promise<void> {
    if ('Live2DCubismCore' in window) return Promise.resolve()
    core ??= new Promise<void>((resolve, reject) => {
      const script = document.createElement('script')
      script.src = coreURL
      script.onload = () => ('Live2DCubismCore' in window ? resolve() : reject(new Error('Cubism Core missing')))
      script.onerror = () => {
        script.remove()
        reject(new Error('Cubism Core unavailable'))
      }
      document.head.append(script)
    }).catch((err) => {
      core = undefined
      throw err
    })
    return core
  }
</script>

<script lang="ts">
  // A skin's Live2D model (/api/live2d), playing its idle motion; a tap plays
  // another of its motions. Fills its parent unless given a size.
  import { onDestroy } from 'svelte'
  import type { Application } from 'pixi.js'
  import type { Live2DModel } from 'pixi-live2d-display/cubism4'

  let {
    name,
    width,
    height,
  }: {
    // the painting, e.g. biaoqiang
    name: string
    width?: number
    height?: number
  } = $props()

  let box: HTMLDivElement
  let canvas: HTMLCanvasElement
  let status = $state<'loading' | 'ready' | 'error'>('loading')
  let error = $state('')

  let app: Application | undefined
  let model: Live2DModel | undefined
  let motions: string[] = []
  let observer: ResizeObserver | undefined
  let generation = 0

  // fit the model in the box, centred
  function fit() {
    if (!app || !model) return
    const w = width ?? box.clientWidth
    const h = height ?? box.clientHeight
    app.renderer.resize(w, h)
    const scale = Math.min(w / (model.width / model.scale.x), h / (model.height / model.scale.y))
    model.scale.set(scale)
    model.position.set(w / 2, h / 2)
  }

  function tap() {
    if (!model || motions.length === 0) return
    const group = motions[Math.floor(Math.random() * motions.length)]
    // MotionPriority.FORCE
    model.motion(group, 0, 3)
  }

  async function show(painting: string) {
    const current = ++generation
    status = 'loading'
    clear()
    try {
      await loadCore()
      const PIXI = await import('pixi.js')
      const { Live2DModel } = await import('pixi-live2d-display/cubism4')
      if (current !== generation) return
      Live2DModel.registerTicker(PIXI.Ticker)
      const url = `/api/live2d/${encodeURIComponent(painting)}/${encodeURIComponent(painting)}.model3.json`
      const res = await fetch(url)
      if (!res.ok) throw new Error(((await res.json().catch(() => null)) as { error?: string } | null)?.error ?? res.statusText)
      const loaded = await Live2DModel.from({ ...(await res.json()), url }, { autoInteract: false })
      if (current !== generation) {
        loaded.destroy()
        return
      }
      app = new PIXI.Application({
        view: canvas,
        backgroundAlpha: 0,
        antialias: true,
        autoDensity: true,
        resolution: window.devicePixelRatio || 1,
        width: width ?? box.clientWidth,
        height: height ?? box.clientHeight,
      })
      model = loaded
      model.anchor.set(0.5, 0.5)
      app.stage.addChild(model)
      const groups = Object.keys(model.internalModel.motionManager.definitions)
      motions = groups.filter((group) => group !== 'Idle')
      if (groups.includes('Idle')) model.motion('Idle', 0, 1)
      fit()
      observer = new ResizeObserver(fit)
      observer.observe(box)
      status = 'ready'
    } catch (err) {
      if (current !== generation) return
      error = err instanceof Error ? err.message : String(err)
      status = 'error'
    }
  }

  function clear() {
    observer?.disconnect()
    observer = undefined
    model?.destroy({ children: true })
    model = undefined
    // keep the canvas, Svelte owns it
    app?.destroy(false, { children: true })
    app = undefined
    motions = []
  }

  $effect(() => {
    show(name)
  })

  $effect(() => {
    void [width, height]
    fit()
  })

  onDestroy(() => {
    generation++
    clear()
  })
</script>

<div
  bind:this={box}
  class={['relative overflow-hidden', width === undefined && 'size-full']}
  style:width={width === undefined ? undefined : `${width}px`}
  style:height={height === undefined ? undefined : `${height}px`}
>
  <canvas bind:this={canvas} class={['block cursor-pointer', status !== 'ready' && 'invisible']} onpointerdown={tap}></canvas>
  {#if status === 'loading'}
    <div class="absolute inset-0 grid place-items-center text-soft">Loading</div>
  {:else if status === 'error'}
    <div class="absolute inset-0 grid place-items-center px-4 text-center text-bad">Live2D: {error}</div>
  {/if}
</div>
