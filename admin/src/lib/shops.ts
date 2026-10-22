// The Shops screen's API (internal/admin shops.go), cached under
// keys.screen(id, 'shops', 'board').
import type { LifeGoods, LifeThing } from './life'

export interface BoardGoods extends LifeGoods {
  // the most the bought count can be set to, 0 only reset
  max: number
}

// a limited goods not bought yet, in the shop kind ('packs' or 'other')
export interface CatalogGoods extends BoardGoods {
  kind: string
}

export interface BoardShop {
  kind: string
  name: string
  activity?: number
  reset_at: number
  refreshes: number
  // the manual refreshes allowed, 0 none
  refresh_max: number
  refresh_price: number
  in_guild: boolean
  bought: number
  // what the prices are paid in, count the commander's
  currencies: LifeThing[]
  goods: BoardGoods[]
}

export interface AkashiLevel {
  level: number
  exp: number
}

export interface ShopBoard {
  shops: BoardShop[]
  guild_id: number
  // limited goods that can be marked bought in the packs and Akashi's shop
  catalog: CatalogGoods[]
  oil: { bought: number; limit: number; reset_at: number }
  gems: LifeThing
  akashi: { exp: number; exp_max: number; levels: AkashiLevel[]; taps: number; daily: number; tapped_at: number }
  requisitions: { count: number; limit: number; reset_at: number; medals: LifeThing }
}

// akashiLevel is the level an affection exp reaches (Player:getChargeLevel).
export function akashiLevel(levels: AkashiLevel[], exp: number): number {
  let level = levels[0]?.level ?? 1
  for (const row of levels) if (row.exp <= exp) level = row.level
  return level
}

export const shopKey = (shop: BoardShop) => (shop.activity ? `${shop.kind}:${shop.activity}` : shop.kind)
export const goodsKey = (goods: BoardGoods) => `${goods.group ? 'g' : ''}${goods.id}`
