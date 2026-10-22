<script lang="ts" module>
  export interface CatalogShip {
    id: number
    group: number
    name: string
    rarity: number
    icon?: string
    type: number
    type_name: string
    nation: string
  }

  // a commander's ship, as given in ships
  export interface OwnedShip {
    id: number
    name: string
    rarity: number
    icon?: string
    level?: number
    sub?: string
  }
</script>

<script lang="ts">
  // Picks a ship: every ship of the game by default (to grant one), or the
  // ships given (a commander's, with their level). Fuzzy-searched by name,
  // id, hull type and faction.
  import { createQuery } from '@tanstack/svelte-query'
  import { get, keys } from '../lib/query'
  import Picker, { type PickerFacet, type PickerItem } from './Picker.svelte'

  let {
    value = $bindable(null),
    ships,
    label = 'Pick a ship',
    onselect,
  }: {
    value?: number | null
    ships?: OwnedShip[]
    label?: string
    onselect?: (item: PickerItem) => void
  } = $props()

  const catalog = createQuery(() => ({
    queryKey: keys.catalog('ships'),
    queryFn: get<CatalogShip[]>('/catalog/ships'),
    staleTime: Infinity,
    enabled: !ships,
  }))

  // the catalog's hull and faction, by ship id; a commander's own ships only
  // have their level
  const details = $derived(new Map((catalog.data ?? []).map((ship) => [ship.id, ship])))
  const rarities = ['', '', 'Common', 'Rare', 'Elite', 'Super Rare', 'Ultra Rare']
  const facets = $derived<PickerFacet[]>(
    ships
      ? [{ label: 'Rarity', value: (item) => rarities[item.rarity ?? 0] }]
      : [
          { label: 'Hull', value: (item) => details.get(item.id)?.type_name },
          { label: 'Faction', value: (item) => details.get(item.id)?.nation },
          { label: 'Rarity', value: (item) => rarities[item.rarity ?? 0] },
        ],
  )

  const items = $derived<PickerItem[]>(
    ships
      ? ships.map((ship) => ({ ...ship, ship: true, sub: ship.sub ?? (ship.level ? `Lv.${ship.level}` : undefined) }))
      : (catalog.data ?? []).map((ship) => ({
          id: ship.id,
          name: ship.name,
          icon: ship.icon,
          rarity: ship.rarity,
          ship: true,
          sub: [ship.type_name, ship.nation].filter(Boolean).join(' · '),
          search: [ship.type_name, ship.nation],
        })),
  )
</script>

<Picker {items} {facets} bind:value {label} placeholder="Name, id, hull or faction" loading={!ships && catalog.isPending} {onselect} />
