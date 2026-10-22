<script lang="ts">
  // The commander's shops, each section with its own actions: Akashi's
  // affection, the requisitions and the oil bought today, then every shop's
  // stock with what was bought of it, its refreshes and its restock.
  import { createQuery } from '@tanstack/svelte-query'
  import CommanderPage from '../../components/CommanderPage.svelte'
  import Icon from '../../components/Icon.svelte'
  import Picker, { type PickerItem } from '../../components/Picker.svelte'
  import { bounded } from '../../lib/bounded'
  import { count, stamp } from '../../lib/format'
  import type { LifeThing } from '../../lib/life'
  import { short } from '../../lib/life'
  import { InlineEdit } from '../../lib/inline.svelte'
  import { shipFramed } from '../../lib/players'
  import { createCommanderWrite, get, keys } from '../../lib/query'
  import { link } from '../../lib/router.svelte'
  import { akashiLevel, goodsKey, shopKey, type BoardGoods, type BoardShop, type ShopBoard } from '../../lib/shops'

  let { id }: { id: number } = $props()

  const query = createQuery(() => ({
    queryKey: keys.screen(id, 'shops', 'board'),
    queryFn: get<ShopBoard>(`/commanders/${id}/shops/board`),
  }))
  const board = $derived(query.data)

  const { mutation: edit, run } = createCommanderWrite(() => id)
  const busy = $derived(edit.isPending)

  // a number edited in place
  const inline = new InlineEdit()

  // Akashi: the exp and its level, edited together
  let level = $state(1)
  function startAffection() {
    if (!board) return
    inline.start('akashi:exp', board.akashi.exp)
    level = akashiLevel(board.akashi.levels, board.akashi.exp)
  }
  function pickLevel(picked: number) {
    level = picked
    inline.value = board?.akashi.levels.find((row) => row.level === picked)?.exp ?? inline.value
  }

  const bodyOf = (shop: BoardShop, goods: BoardGoods, n: number) => ({ kind: shop.kind, activity: shop.activity ?? 0, id: goods.id, group: goods.group ?? false, count: n })
  const setBought = (shop: BoardShop, goods: BoardGoods, n: number) => run('/shops/bought', n ? `${goods.goods.name}: ${n} bought` : `${goods.goods.name} reset`, bodyOf(shop, goods, n))
  const restock = (shop: BoardShop) => run('/shops/reset', `${shop.name} restocked`, { kind: shop.kind, activity: shop.activity ?? 0, id: 0, group: false })

  // the limited goods marked bought from the catalog
  let catalogPicker = $state<Picker>()
  let adding = $state<BoardShop | null>(null)
  const catalogFor = (shop: BoardShop) => (board?.catalog ?? []).filter((goods) => goods.kind === shop.kind)
  const catalogItems = $derived<PickerItem[]>(
    adding
      ? catalogFor(adding).map((goods) => ({ id: goods.id, name: goods.goods.name, icon: goods.goods.icon, rarity: goods.goods.rarity, ship: shipFramed(goods.goods.type), sub: `#${goods.id} · ${goods.limit}x`, search: [goods.id] }))
      : [],
  )
  function openCatalog(shop: BoardShop) {
    adding = shop
    queueMicrotask(() => catalogPicker?.show())
  }
  function pickGoods(item: PickerItem) {
    const goods = board?.catalog.find((row) => row.id === item.id)
    if (goods && adding) run('/shops/bought', `${goods.goods.name}: 1 bought`, { kind: goods.kind, activity: 0, id: goods.id, group: false, count: 1 })
    adding = null
  }

  function timer(shop: BoardShop): string {
    if (shop.kind === 'newserver') return shop.reset_at ? `Until ${stamp(shop.reset_at)}` : ''
    if (shop.kind === 'packs' || shop.kind === 'other' || !shop.reset_at) return ''
    return `Restock ${short(shop.reset_at)}`
  }
  // the shops with many goods show what was bought until opened
  const longShop = 20
  let expanded = $state<Record<string, boolean>>({})
  const shown = (shop: BoardShop) => (shop.goods.length > longShop && !expanded[shopKey(shop)] ? shop.goods.filter((goods) => goods.bought > 0) : shop.goods)
  const ratio = (goods: BoardGoods) => (goods.limit ? Math.min(100, (goods.bought / goods.limit) * 100) : goods.bought ? 100 : 0)
</script>

{#snippet number(key: string, current: number, max: number, label: string, apply: (v: number) => void, size = 'text-[30px]', suffix?: number)}
  {#if inline.editing === key}
    <form class="flex items-center gap-1.5" onsubmit={(e) => inline.save(e, current, apply)}>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="ip-field num h-9 w-24 min-w-0 text-lg" type="number" aria-label={label} autofocus bind:value={inline.value} min="0" {max} use:bounded={{ min: 0, max }} onkeydown={inline.escape} />
      <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
    </form>
  {:else}
    <button type="button" aria-label="Edit {label}" class="cursor-pointer rounded border-0 bg-transparent px-1 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" disabled={busy} onclick={() => inline.start(key, current)}>
      <span class={['num', size]}>{count(current)}</span>{#if suffix !== undefined}<span class="num text-lg text-dim"> /{count(suffix)}</span>{/if}
    </button>
  {/if}
{/snippet}

{#snippet wallet(things: LifeThing[])}
  {#each things as thing (`${thing.type}:${thing.id}`)}
    <a href="/commanders/{id}/items" use:link class="flex items-center gap-1.5 text-link" title={thing.name}>
      {#if thing.icon}<img src="/api/assets/{thing.icon}.png" alt={thing.name} class="size-[22px] object-contain" />{:else}<span class="text-[15px]">{thing.name}</span>{/if}
      <span class="num text-lg">{count(thing.count)}</span>
    </a>
  {/each}
{/snippet}

{#snippet tile(shop: BoardShop, goods: BoardGoods)}
  {@const key = `${shopKey(shop)}/${goodsKey(goods)}`}
  {@const full = goods.limit > 0 && goods.bought >= goods.limit}
  <div class="ip-row flex items-center gap-3 px-2.5 py-2">
    <Icon icon={goods.goods.icon} rarity={goods.goods.rarity} ship={shipFramed(goods.goods.type)} count={goods.goods.count > 1 ? goods.goods.count : undefined} name={goods.goods.name} size={48} />
    <div class="flex min-w-0 flex-1 flex-col gap-1">
      <span class="flex items-baseline gap-2">
        <span class="truncate text-[15px]" title={goods.goods.name}>{goods.goods.name}</span>
        {#if goods.group}<span class="ip-tag shrink-0 bg-white/10 text-[12px] text-soft">Group</span>{:else if goods.never}<span class="ip-tag shrink-0 bg-white/10 text-[12px] text-soft">Permanent</span>{/if}
      </span>
      <span class="flex min-h-9 items-center gap-2">
        {#if goods.price.count}
          <span class="inline-flex items-center gap-1 text-soft">
            {#if goods.price.icon}<img src="/api/assets/{goods.price.icon}.png" alt={goods.price.name} title={goods.price.name} class="size-[18px] object-contain" />{/if}
            <span class="num text-base">{count(goods.price.count)}</span>
          </span>
        {/if}
        <span class="ml-auto flex items-center">
          {#if goods.max > 0}
            {#if inline.editing === key}
              <form class="flex items-center gap-1.5" onsubmit={(e) => inline.save(e, goods.bought, (v) => setBought(shop, goods, v))}>
                <!-- svelte-ignore a11y_autofocus -->
                <input class="ip-field num h-9 w-16 min-w-0 text-lg" type="number" aria-label="{goods.goods.name} bought" autofocus bind:value={inline.value} min="0" max={goods.max} use:bounded={{ min: 0, max: goods.max }} onkeydown={inline.escape} />
                <button type="submit" class="ip-btn ip-sm h-9" disabled={busy}>Save</button>
              </form>
            {:else}
              <button type="button" aria-label="Edit {goods.goods.name} bought" class="cursor-pointer rounded border-0 bg-transparent px-1 text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" disabled={busy} onclick={() => inline.start(key, goods.bought)}>
                <span class={['num text-xl', full && 'text-red']}>{goods.bought}</span><span class="num text-base text-dim">/{goods.limit}</span>
              </button>
            {/if}
          {:else}
            <span class="num px-1 text-xl">{goods.bought}</span>
            {#if goods.limit}<span class="num text-base text-dim">/{goods.limit}</span>{/if}
            {#if goods.bought > 0}
              <button type="button" class="ip-btn2 ip-sm ml-2" aria-label="Reset {goods.goods.name}" disabled={busy} onclick={() => setBought(shop, goods, 0)}>Reset</button>
            {/if}
          {/if}
        </span>
      </span>
      <span aria-hidden="true" class="block h-1 rounded-[2px] bg-black/40"><span class={['block h-full rounded-[2px]', full ? 'bg-red' : 'bg-cyan']} style:width="{ratio(goods)}%"></span></span>
    </div>
  </div>
{/snippet}

<CommanderPage {id} title="Shops">
  {#snippet children()}
    {#if board}
      {@const akashi = board.akashi}
      {@const req = board.requisitions}
      <div class="flex flex-col gap-5">
        <div class="grid grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_minmax(0,1fr)] gap-5">
          <section aria-label="Akashi" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Akashi</h2>
              <button type="button" class="ip-btn2 ip-sm ml-auto" disabled={busy || (akashi.exp === 0 && akashi.taps === 0)} onclick={() => run('/akashi', 'Affection reset', { reset: true })}>Reset</button>
            </div>
            <div class="grid grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)] gap-3 p-3.5">
              {#if inline.editing === 'akashi:exp'}
                <form class="ip-row flex flex-col gap-2 px-3.5 py-3" onsubmit={(e) => inline.save(e, akashi.exp, (v) => run('/akashi', `Affection set to ${count(v)}`, { exp: v, taps: akashi.taps }))}>
                  <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-2">
                    <!-- svelte-ignore a11y_autofocus -->
                    <input class="ip-field num h-10 min-w-0 text-xl" type="number" aria-label="Affection" autofocus bind:value={inline.value} min="0" max={akashi.exp_max} use:bounded={{ min: 0, max: akashi.exp_max }} oninput={() => (level = akashiLevel(akashi.levels, inline.value))} onkeydown={inline.escape} />
                    <select class="ip-field h-10 min-w-0" aria-label="Level" value={level} onchange={(e) => pickLevel(Number(e.currentTarget.value))} onkeydown={inline.escape}>
                      {#each akashi.levels as row (row.level)}<option value={row.level}>Lv.{row.level}</option>{/each}
                    </select>
                  </div>
                  <div class="flex justify-end gap-2">
                    <button type="button" class="ip-btn2 ip-sm" onclick={() => inline.stop()}>Cancel</button>
                    <button type="submit" class="ip-btn ip-sm" disabled={busy}>Save</button>
                  </div>
                </form>
              {:else}
                <button type="button" aria-label="Edit the affection" class="ip-row flex cursor-pointer flex-col gap-1 border-0 px-3.5 py-3 text-left text-white hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" disabled={busy} onclick={startAffection}>
                  <span class="ip-sub">Affection</span>
                  <span class="flex items-baseline gap-2.5">
                    <span class="text-2xl text-gold">Lv.<span class="num">{akashiLevel(akashi.levels, akashi.exp)}</span></span>
                    <span class="num text-[22px] text-soft">{count(akashi.exp)}</span>
                    <span class="text-sm text-dim">exp</span>
                  </span>
                </button>
              {/if}
              <div class="ip-row flex flex-col gap-1 px-3.5 py-3">
                <span class="ip-sub">Taps today</span>
                {@render number('akashi:taps', akashi.taps, akashi.daily, 'Taps today', (v) => run('/akashi', `Taps set to ${v}`, { exp: akashi.exp, taps: v }), 'text-[30px]', akashi.daily)}
                <span class="text-[13px] text-dim">Last <span class="num text-base text-soft">{stamp(akashi.tapped_at)}</span></span>
              </div>
            </div>
          </section>

          <section aria-label="Requisitions" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Requisitions</h2>
              <span class="ml-auto flex items-center gap-3">
                {@render wallet([req.medals])}
                <button type="button" class="ip-btn2 ip-sm" disabled={busy || req.count === 0} onclick={() => run('/requisitions/reset', 'Requisitions reset')}>Reset</button>
              </span>
            </div>
            <div class="p-3.5">
              <div class="ip-row flex flex-col gap-1 px-3.5 py-3">
                <span class="ip-sub">This month</span>
                {@render number('requisitions', req.count, req.limit, 'Requisitions', (v) => run('/requisitions', `Requisitions set to ${count(v)}`, { count: v }), 'text-[30px]', req.limit)}
                <span class="text-[13px] text-dim">Resets <span class="num text-base text-soft">{short(req.reset_at)}</span></span>
              </div>
            </div>
          </section>

          <section aria-label="Oil" class="ip-panel">
            <div class="ip-head items-center">
              <h2>Oil</h2>
              <span class="ml-auto flex items-center gap-3">
                {@render wallet([board.gems])}
                <button type="button" class="ip-btn2 ip-sm" disabled={busy || board.oil.bought === 0} onclick={() => run('/shops/oil', 'Oil purchases reset', { count: 0 })}>Reset</button>
              </span>
            </div>
            <div class="p-3.5">
              <div class="ip-row flex flex-col gap-1 px-3.5 py-3">
                <span class="ip-sub">Bought today</span>
                {@render number('oil', board.oil.bought, board.oil.limit, 'Oil bought today', (v) => run('/shops/oil', `Oil bought set to ${v}`, { count: v }), 'text-[30px]', board.oil.limit)}
                <span class="text-[13px] text-dim">Resets <span class="num text-base text-soft">{short(board.oil.reset_at)}</span></span>
              </div>
            </div>
          </section>
        </div>

        {#each board.shops as shop (shopKey(shop))}
          <section aria-label={shop.name} class="ip-panel">
            <div class="ip-head flex-wrap items-center gap-y-2">
              <h2 class="whitespace-nowrap">{shop.name}</h2>
              {#if timer(shop)}<span class="num text-[17px] text-soft">{timer(shop)}</span>{/if}
              {#if shop.refresh_max}
                <span class="flex items-center gap-1.5 text-[15px] text-soft">
                  Refreshes
                  {@render number(`refresh:${shopKey(shop)}`, shop.refreshes, shop.refresh_max, `${shop.name} refreshes`, (v) => run('/shops/refreshes', `${shop.name}: ${v} refreshes`, { kind: shop.kind, count: v }), 'text-xl', shop.refresh_max)}
                </span>
              {/if}
              {#if shop.kind === 'arena'}
                <a href="/commanders/{id}/combat" use:link class="text-[15px] text-link">Exercises</a>
              {:else if shop.kind === 'guild' && board.guild_id}
                <a href="/guilds/{board.guild_id}" use:link class="text-[15px] text-link">Guild</a>
              {:else if shop.kind === 'newserver'}
                <a href="/commanders/{id}/activities" use:link class="text-[15px] text-link">Activity <span class="num">{shop.activity}</span></a>
              {/if}
              {#if shop.goods.length > longShop}
                {@const open = !!expanded[shopKey(shop)]}
                <button type="button" aria-expanded={open} class="flex cursor-pointer items-center gap-1.5 rounded border-0 bg-transparent px-1.5 py-0.5 text-[15px] text-soft hover:shadow-[inset_0_0_0_2px_var(--color-cyan)]" onclick={() => (expanded[shopKey(shop)] = !open)}>
                  <span class="num text-lg text-white">{shop.goods.length}</span> goods
                  <svg aria-hidden="true" viewBox="0 0 24 24" class={['size-4 text-dim transition-transform', open && 'rotate-180']} fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round"><path d="M6 9l6 6 6-6" /></svg>
                </button>
              {/if}
              <span class="ml-auto flex items-center gap-3">
                {@render wallet(shop.currencies)}
                {#if shop.kind === 'packs' || shop.kind === 'other'}
                  <button type="button" class="ip-btn2 ip-sm" disabled={busy || catalogFor(shop).length === 0} onclick={() => openCatalog(shop)}>Add</button>
                {/if}
                <button type="button" class="ip-btn2 ip-sm" disabled={busy || shop.bought === 0} onclick={() => restock(shop)}>{shop.kind === 'packs' || shop.kind === 'other' ? 'Reset all' : 'Restock'}</button>
              </span>
            </div>
            {#if shop.kind === 'guild' && !shop.in_guild}
              <div class="px-3.5 py-3 text-dim">Not in a guild</div>
            {:else if shop.goods.length === 0}
              <div class="px-3.5 py-3 text-dim">{shop.kind === 'packs' || shop.kind === 'other' ? 'Nothing bought' : 'No stock'}</div>
            {:else if shown(shop).length === 0}
              <div class="px-3.5 py-3 text-dim">Nothing bought</div>
            {:else}
              <div class="grid grid-cols-[repeat(auto-fill,minmax(270px,1fr))] gap-2 p-3">
                {#each shown(shop) as goods (goodsKey(goods))}
                  {@render tile(shop, goods)}
                {/each}
              </div>
            {/if}
          </section>
        {/each}
      </div>

      <Picker bind:this={catalogPicker} trigger={false} label="Limited goods" items={catalogItems} onselect={pickGoods} />
    {:else}
      <div class="text-dim">{query.isError ? query.error.message : 'Loading'}</div>
    {/if}
  {/snippet}
</CommanderPage>
