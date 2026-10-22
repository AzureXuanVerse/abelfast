<script lang="ts">
  // The dock stage's editors of a ship, each a modal saving only what it
  // changes: open('level') for its limit break, level, exp and retrofit,
  // open('status') for affinity, oath, morale and fleet, open('skills') for
  // its skill levels, open('meta') for a META ship's repairs and research,
  // gear(pos) for the gear picker of a slot.
  import { tick } from 'svelte'
  import Modal from '../../components/Modal.svelte'
  import Icon from '../../components/Icon.svelte'
  import Picker, { type PickerItem } from '../../components/Picker.svelte'
  import Stepper from '../../components/Stepper.svelte'
  import { api } from '../../lib/api'
  import { fleetName, teamLabel, type ShipDetail, type ShipEditBody } from '../../lib/dock'
  import { createCommanderMutation } from '../../lib/query'
  import { bounded } from '../../lib/bounded'
  import { metaAttrLabels, metaExpMax, repairPercent, type MetaShip, type MetaShipEditBody } from '../../lib/metaships'

  let { id, ship, meta }: { id: number; ship: ShipDetail; meta?: MetaShip } = $props()

  let levelModal = $state<Modal>()
  let statusModal = $state<Modal>()
  let skillsModal = $state<Modal>()
  let metaModal = $state<Modal>()
  let picker = $state<Picker>()
  let open: Modal | undefined

  // the forms, set from the ship when opened
  let templateID = $state(0)
  let level = $state(1)
  let exp = $state(0)
  let retrofit = $state<'keep' | 'full' | 'none'>('keep')
  let affinity = $state(0)
  let oath = $state(false)
  let energy = $state(0)
  let fleet = $state(0)
  let slot = $state(0)
  let skills = $state<Record<number, number>>({})
  let gearPos = $state(1)
  let repairs = $state<Record<string, number>>({})
  let studying = $state(0)
  let research = $state<Record<number, { level: number; exp: number }>>({})

  export function edit(kind: 'level' | 'status' | 'skills' | 'meta') {
    templateID = ship.template_id
    level = ship.level
    exp = ship.exp
    retrofit = 'keep'
    affinity = ship.intimacy / 100
    oath = ship.propose !== 0
    energy = ship.energy
    fleet = ship.fleet
    slot = ship.slot
    skills = Object.fromEntries(ship.skills.map((s) => [s.id, s.level]))
    repairs = Object.fromEntries((meta?.repair?.attrs ?? []).map((a) => [a.attr, a.done]))
    studying = meta?.tactics?.skill_id ?? 0
    research = Object.fromEntries((meta?.tactics?.skills ?? []).map((s) => [s.id, { level: s.level, exp: s.exp }]))
    open = { level: levelModal, status: statusModal, skills: skillsModal, meta: metaModal }[kind]
    open?.show()
  }

  export function gear(pos: number) {
    gearPos = pos
    tick().then(() => picker?.show())
  }

  const save = createCommanderMutation(
    () => id,
    (body: ShipEditBody) => api<ShipDetail>('POST', `/commanders/${id}/dock/${ship.id}`, body),
    { success: (saved) => `${saved.name} saved`, onSuccess: () => open?.close() },
  )

  const star = $derived(ship.stars.find((s) => s.template_id === templateID))
  const levelMax = $derived(templateID === ship.template_id ? ship.level_cap : Math.max(star?.max_level ?? 1, 1))
  const starsLocked = $derived(retrofit === 'full' || ((ship.retrofit?.done ?? 0) > 0 && retrofit !== 'none'))

  function saveLevel() {
    const body: ShipEditBody = {}
    if (templateID !== ship.template_id) body.template_id = templateID
    if (level !== ship.level) body.level = level
    if (exp !== ship.exp) body.exp = exp
    if (retrofit !== 'keep') body.retrofit = retrofit === 'full'
    save.mutate(body)
  }

  function saveStatus() {
    const body: ShipEditBody = {}
    if (oath !== (ship.propose !== 0)) body.propose = oath
    if (oath && !ship.locked) body.locked = true
    if (Math.round(affinity * 100) !== ship.intimacy) body.intimacy = Math.round(affinity * 100)
    if (energy !== ship.energy) body.energy = energy
    if (fleet !== ship.fleet || (fleet && slot !== ship.slot)) {
      body.fleet = fleet
      body.slot = slot
    }
    save.mutate(body)
  }

  function saveSkills() {
    const changed = Object.fromEntries(ship.skills.filter((s) => skills[s.id] !== s.level).map((s) => [s.id, skills[s.id]]))
    save.mutate({ skills: changed })
  }

  const metaPath = () => `/commanders/${id}/dock/${ship.id}/meta`
  const saveMeta = createCommanderMutation(
    () => id,
    (body: MetaShipEditBody) => api<MetaShip>('POST', metaPath(), body),
    { success: () => `${ship.name} saved`, onSuccess: () => open?.close() },
  )
  const resetDay = createCommanderMutation(
    () => id,
    () => api<MetaShip>('POST', metaPath(), { reset_day: true }),
    { success: () => `${ship.name}: today's research reset` },
  )

  function saveMetaEdit() {
    if (!meta?.repair || !meta.tactics) return
    const body: MetaShipEditBody = {}
    const changedRepairs = meta.repair.attrs.filter((a) => repairs[a.attr] !== a.done)
    if (changedRepairs.length) body.repairs = Object.fromEntries(changedRepairs.map((a) => [a.attr, repairs[a.attr]]))
    const changedSkills = meta.tactics.skills.filter((s) => research[s.id].level !== s.level || research[s.id].exp !== s.exp)
    if (changedSkills.length) body.skills = Object.fromEntries(changedSkills.map((s) => [s.id, research[s.id]]))
    // a locked skill can't be studied
    const study = studying && research[studying]?.level === 0 ? 0 : studying
    if (study !== meta.tactics.skill_id) body.skill_id = study
    if (Object.keys(body).length) saveMeta.mutate(body)
    else open?.close()
  }

  const label = (g: { name: string; plus: number; id: number }) => `${g.name || '#' + g.id}${g.plus ? ' +' + g.plus : ''}`
  const slotGear = $derived(ship.gear.find((g) => g.pos === gearPos))
  // what the slot can take: what it holds, the depot's fitting gear, empty
  const gearItems = $derived.by<PickerItem[]>(() => {
    if (!slotGear) return []
    const list: PickerItem[] = slotGear.fits.map((fit) => ({ id: fit.id, name: label(fit), icon: fit.icon, rarity: fit.rarity ?? 1, sub: `×${fit.count ?? 0} in the depot` }))
    const equip = slotGear.equip
    if (equip) {
      if (!list.some((item) => item.id === equip.id)) list.unshift({ id: equip.id, name: label(equip), icon: equip.icon, rarity: equip.rarity ?? 1, sub: 'Equipped' })
      list.unshift({ id: 0, name: 'Empty', rarity: 1, sub: 'Unequip' })
    }
    return list
  })
  function pickGear(item: PickerItem) {
    if (item.id === (slotGear?.equip?.id ?? 0)) return
    open = undefined
    save.mutate({ gear: { [gearPos]: item.id } })
  }

  const dark = 'h-12 w-full min-w-0 rounded border-0 bg-black/35 px-3.5 text-lg text-white shadow-[inset_0_0_0_1px_rgb(255_255_255/0.14)] focus:shadow-[inset_0_0_0_2px_var(--color-cyan)] focus:outline-none disabled:bg-black/20 disabled:text-dim'
  const field = 'flex min-w-0 flex-col gap-1.5 text-[15px] text-soft'
</script>

<Modal bind:this={levelModal} title="{ship.name}: level">
  <form class="flex flex-col gap-4" onsubmit={(e) => (e.preventDefault(), saveLevel())}>
    <div class={field}>
      <span>Limit break <span class="text-[#ffd84a]">{'★'.repeat(star?.star ?? 0)}</span></span>
      <div role="radiogroup" aria-label="Limit break" class="ip-seg flex-nowrap">
        {#each ship.stars as option, i (option.template_id)}
          <button
            type="button"
            class="num flex-1 justify-center px-1.5 text-2xl"
            aria-pressed={templateID === option.template_id}
            disabled={starsLocked}
            onclick={() => ((templateID = option.template_id), (level = Math.min(level, Math.max(option.max_level, templateID === ship.template_id ? ship.level_cap : 0))))}>{i}</button
          >
        {/each}
      </div>
    </div>
    <div class="grid grid-cols-2 gap-4">
      <div class={field}>
        <span>Level <span class="num text-dim">1-{levelMax}</span></span>
        <Stepper label="Level" bind:value={level} min={1} max={levelMax} />
      </div>
      <label class={field}>
        Exp
        <input class={[dark, 'num h-[52px] text-[26px]']} type="number" min="0" bind:value={exp} use:bounded={{ min: 0, max: ship.exp_maxes?.[level - 1] ?? ship.exp_max }} />
      </label>
    </div>
    <label class={field}>
      Retrofit
      <select class={dark} bind:value={retrofit} disabled={!ship.retrofit}>
        {#if !ship.retrofit}
          <option value="keep">{ship.name} has no retrofit</option>
        {:else}
          <option value="keep">{ship.retrofit.full ? 'Retrofitted' : ship.retrofit.done ? `${ship.retrofit.done} of ${ship.retrofit.nodes} nodes` : 'Not retrofitted'}</option>
          {#if !ship.retrofit.full}<option value="full">Retrofit fully (Lv.{ship.retrofit.level}, {ship.retrofit.star} stars)</option>{/if}
          {#if ship.retrofit.done}<option value="none">Undo the retrofit</option>{/if}
        {/if}
      </select>
    </label>
    <button type="submit" class="ip-btn h-12" disabled={save.isPending}>Save</button>
  </form>
</Modal>

<Modal bind:this={statusModal} title="{ship.name}: status">
  <form class="flex flex-col gap-4" onsubmit={(e) => (e.preventDefault(), saveStatus())}>
    <div class={field}>
      <span>Affinity <span class="num text-dim">0-{oath ? 200 : 100}</span></span>
      <div class="flex items-center gap-2.5">
        <input aria-label="Affinity" class={[dark, 'num w-28! text-[26px] text-[#ff9cc0]']} type="number" min="0" max={oath ? 200 : 100} step="0.01" bind:value={affinity} use:bounded={{ min: 0, max: oath ? 200 : 100, step: 0.01 }} />
        <div class="ip-seg min-w-0 flex-1">
          <button type="button" class="min-w-0 flex-1 justify-center gap-1.5" aria-pressed={oath} onclick={() => ((oath = !oath), !oath && affinity > 100 && (affinity = 100))}>
            <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[18px] fill-[#ff5c93]"><path d="M12 20s-7-4.4-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 10c0 5.6-7 10-7 10Z" /></svg>{oath ? 'Oath sworn' : 'Swear oath'}
          </button>
        </div>
      </div>
    </div>
    <label class={field}>
      <span>Morale <span class="num text-dim">0-{ship.energy_max}</span></span>
      <input class={[dark, 'num w-28! text-[26px]']} type="number" min="0" max={ship.energy_max} bind:value={energy} use:bounded={{ min: 0, max: ship.energy_max }} />
    </label>
    <div class="grid grid-cols-2 gap-4">
      <label class={field}>
        Fleet
        <select class={dark} bind:value={fleet} onchange={() => (slot = 0)}>
          <option value={0}>Not in a fleet</option>
          {#each ship.fleets as option (option.id)}<option value={option.id} disabled={option.locked}>{fleetName(option.id)}{option.locked ? ' (locked)' : ''}</option>{/each}
        </select>
      </label>
      <div class={field}>
        <span>{teamLabel[ship.team] ?? ''} slot</span>
        <div role="radiogroup" aria-label="Slot" class="ip-seg flex-nowrap">
          {#each [1, 2, 3] as n (n)}
            <button type="button" class="num flex-1 justify-center text-2xl" aria-pressed={fleet !== 0 && slot === n} disabled={!fleet} onclick={() => (slot = n)}>{n}</button>
          {/each}
        </div>
      </div>
    </div>
    {#if ship.in_chapter}<p class="m-0 text-[15px] text-red">In the chapter being played</p>{/if}
    <button type="submit" class="ip-btn h-12" disabled={save.isPending}>Save</button>
  </form>
</Modal>

<Modal bind:this={skillsModal} title="{ship.name}: skills">
  <form class="flex flex-col gap-2.5" onsubmit={(e) => (e.preventDefault(), saveSkills())}>
    {#each ship.skills as skill (skill.id)}
      <div class="flex items-center gap-3">
        <span class="min-w-0 flex-1 text-base leading-tight">{skill.name || `Skill ${skill.id}`}</span>
        <div class="w-[170px] flex-none"><Stepper label="{skill.name} level" bind:value={skills[skill.id]} min={1} max={skill.max} /></div>
      </div>
    {/each}
    <button type="submit" class="ip-btn mt-1.5 h-12" disabled={save.isPending}>Save</button>
  </form>
</Modal>

{#if meta?.repair && meta.tactics}
  {@const repair = meta.repair}
  {@const tactics = meta.tactics}
  <Modal bind:this={metaModal} title="{ship.name}: META" width="w-[min(680px,92vw)]">
    <form class="flex flex-col gap-4" onsubmit={(e) => (e.preventDefault(), saveMetaEdit())}>
      <section aria-label="Repair" class="flex flex-col gap-2.5">
        <div class="flex items-baseline gap-2.5">
          <h3 class="m-0 text-lg">Repair</h3>
          <span class="num text-2xl">{repairPercent(repair)}%</span>
          <span class="num text-base text-dim">{repair.exp}/{repair.exp_max}</span>
          <button type="button" class="ip-btn2 ip-sm ml-auto" onclick={() => (repairs = Object.fromEntries(repair.attrs.map((a) => [a.attr, a.max])))}>Repair fully</button>
        </div>
        {#each repair.attrs as attr (attr.attr)}
          <div class="flex items-center gap-3">
            <Icon icon={attr.item.icon ?? ''} rarity={attr.item.rarity || 1} size={44} name={attr.item.name} label={false} />
            <span class="flex min-w-0 flex-1 flex-col leading-tight">
              <span class="text-base">{metaAttrLabels[attr.attr] ?? attr.attr} <span class="num text-dim">0-{attr.max}</span></span>
              <span class="num text-[15px] text-soft">+{attr.addition}/{attr.addition_max}</span>
            </span>
            <div class="w-[190px] flex-none"><Stepper label="{metaAttrLabels[attr.attr] ?? attr.attr} repairs" bind:value={repairs[attr.attr]} min={0} max={attr.max} /></div>
          </div>
        {/each}
        {#if repair.effects.length}
          <ul aria-label="Repair effects" class="m-0 flex list-none flex-wrap gap-1.5 p-0">
            {#each repair.effects as effect (effect.id)}
              <li
                class={['ip-tag num text-white', effect.reached ? 'bg-cyan' : 'bg-white/10']}
                title={effect.attrs.map((a) => `${a.attr} +${a.value}`).join(', ')}>
                {effect.progress}%
              </li>
            {/each}
          </ul>
        {/if}
      </section>

      <section aria-label="Tactics" class="flex flex-col gap-2.5 border-t border-white/10 pt-4">
        <h3 class="m-0 text-lg">Tactics</h3>
        <label class={field}>
          Studying
          <select class={dark} bind:value={studying}>
            <option value={0}>None</option>
            {#each tactics.skills as skill (skill.id)}
              <option value={skill.id} disabled={research[skill.id]?.level === 0}>{skill.name || `Skill ${skill.id}`}</option>
            {/each}
          </select>
        </label>
        {#each tactics.skills as skill (skill.id)}
          {@const top = metaExpMax(skill, research[skill.id].level)}
          <div class="grid grid-cols-[1fr_170px_150px] items-center gap-3">
            <span class="min-w-0 text-base leading-tight">{skill.name || `Skill ${skill.id}`} <span class="num text-dim">0-{skill.max}</span></span>
            <Stepper label="{skill.name} level" bind:value={research[skill.id].level} min={0} max={skill.max} />
            <label class="flex items-center gap-1.5">
              <span class="sr-only">{skill.name} exp</span>
              <input class={[dark, 'num w-[86px]! px-2 text-right']} type="number" min="0" max={top} disabled={top === 0} bind:value={research[skill.id].exp} use:bounded={{ min: 0, max: top }} />
              <span class="num text-[15px] text-dim">/{top || 'Max'}</span>
            </label>
          </div>
        {/each}
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-base text-soft">
          <span>Today</span>
          <span>Exp <span class="num text-xl text-white">{tactics.day_exp}</span><span class="num text-dim">/{tactics.day_exp_max}</span></span>
          <span>Double <span class="num text-xl text-white">{tactics.double_exp}</span><span class="num text-dim">/{tactics.double_exp_max}</span></span>
          <span>Switches <span class="num text-xl text-white">{tactics.switches}</span><span class="num text-dim">/{tactics.switches_max}</span></span>
          <button
            type="button"
            class="ip-btn2 ip-sm ml-auto"
            disabled={resetDay.isPending || (tactics.day_exp === 0 && tactics.double_exp === 0 && tactics.switches === 0)}
            onclick={() => resetDay.mutate()}>Reset today</button
          >
        </div>
      </section>
      <button type="submit" class="ip-btn h-12" disabled={saveMeta.isPending}>Save</button>
    </form>
  </Modal>
{/if}

<Picker
  bind:this={picker} trigger={false} label="Slot {gearPos}" items={gearItems} placeholder="Gear name or id" value={slotGear?.equip?.id ?? 0} onselect={pickGear} />
