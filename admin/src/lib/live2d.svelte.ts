// Which skins have a Live2D model: /api/live2d lists their paintings.
import { createQuery } from '@tanstack/svelte-query'
import { get, keys } from './query'

export function createLive2D() {
  const query = createQuery(() => ({
    queryKey: keys.catalog('live2d'),
    queryFn: get<{ paintings: string[] }>('/live2d'),
    staleTime: Infinity,
  }))
  let paintings = $derived(new Set(query.data?.paintings ?? []))
  return {
    // has tells by a skin's icon (squareicon/<painting>) or its painting
    has: (icon?: string) => !!icon && paintings.has(icon.split('/').pop()!),
  }
}
