<script lang="ts">
  // Picks anything the catalog names, by drop type (dropTypes in lib/api):
  // items, gear, furniture, virtual items... with its icon.
  import { createQuery } from '@tanstack/svelte-query'
  import type { CatalogEntry } from '../lib/api'
  import { get, keys } from '../lib/query'
  import Picker, { type PickerItem } from './Picker.svelte'

  let {
    type,
    value = $bindable(null),
    label = 'Pick',
    onselect,
  }: {
    type: number
    value?: number | null
    label?: string
    onselect?: (item: PickerItem) => void
  } = $props()

  const catalog = createQuery(() => ({
    queryKey: keys.catalog('all', type),
    queryFn: get<CatalogEntry[]>(`/catalog/all?type=${type}`),
    staleTime: Infinity,
  }))

  const items = $derived<PickerItem[]>((catalog.data ?? []).map((entry) => ({ ...entry })))
</script>

<Picker {items} bind:value {label} placeholder="Name or id" loading={catalog.isPending} {onselect} />
