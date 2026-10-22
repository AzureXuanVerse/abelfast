<script lang="ts" module>
  export interface CatalogSkin {
    id: number
    name: string
    group: number
    ship_name: string
    icon?: string
  }
</script>

<script lang="ts">
  // Picks a ship skin, with its thumbnail: every skin, or a ship group's
  // only. Fuzzy-searched by skin name, ship name and id.
  import { createQuery } from '@tanstack/svelte-query'
  import { get, keys } from '../lib/query'
  import { createLive2D } from '../lib/live2d.svelte'
  import Picker, { type PickerFacet, type PickerItem } from './Picker.svelte'

  let {
    value = $bindable(null),
    group,
    owned,
    onlyOwned = false,
    label = 'Pick a skin',
    onselect,
  }: {
    value?: number | null
    // only this ship group's skins
    group?: number
    // skin ids to tag as owned
    owned?: Set<number>
    // only the owned skins can be picked (the others are shown dimmed)
    onlyOwned?: boolean
    label?: string
    onselect?: (item: PickerItem) => void
  } = $props()

  const catalog = createQuery(() => ({
    queryKey: keys.catalog('skins'),
    queryFn: get<CatalogSkin[]>('/catalog/skins'),
    staleTime: Infinity,
  }))

  const live2d = createLive2D()
  const ownedFacet: PickerFacet[] = [{ label: 'Owned', value: (item) => (owned?.has(item.id) ? 'Owned' : 'Not owned') }]

  const items = $derived<PickerItem[]>(
    (catalog.data ?? [])
      .filter((skin) => group === undefined || skin.group === group)
      .map((skin) => ({
        id: skin.id,
        name: skin.name,
        icon: skin.icon,
        rarity: 4,
        ship: false,
        sub: owned?.has(skin.id) ? `${skin.ship_name} · Owned` : skin.ship_name,
        search: [skin.ship_name],
        disabled: onlyOwned && !owned?.has(skin.id),
        live2d: live2d.has(skin.icon),
      })),
  )
</script>

<Picker {items} facets={owned ? ownedFacet : []} bind:value {label} placeholder="Skin or ship name" loading={catalog.isPending} {onselect} />
