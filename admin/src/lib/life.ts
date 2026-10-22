// The Shops, Activities and Archives screens' API (internal/admin area_life.go),
// cached under keys.screen(id, 'shops' | 'activities' | 'archives').
import { clock } from './format'

export interface LifeThing {
  type: number
  id: number
  name: string
  rarity: number
  icon?: string
  count: number
}

export interface LifeGoods {
  id: number
  group?: boolean
  goods: LifeThing
  price: LifeThing
  bought: number
  // 0 no limit
  limit: number
  // the purchases never reset
  never?: boolean
}

export interface LifeTable {
  table: string
  label: string
  columns: string[]
  rows: (string | number | boolean)[][]
  total: number
}

export type LifeStatus = 'running' | 'permanent' | 'finished'

export interface LifeActivity {
  key: string
  name: string
  kind: string
  category: string
  activity?: number
  type?: number
  status: LifeStatus
  start: number
  end: number
  progress: string
  tasks: number
  tables: LifeTable[]
}

export interface LifeLook {
  secretaries: number[]
  random_mode: number
  random_enabled: boolean
  marry_ship: number
  child_display: number
  icon_frame: number
  chat_frame: number
  cover: number
  max_secretaries?: number
}

export interface LifeOption {
  id: number
  name: string
  icon?: string
  // the commander's ships only
  rarity?: number
  level?: number
}

export interface LifeArchives {
  ships: {
    group: number
    name: string
    icon?: string
    rarity: number
    level: number
    intimacy: number
    stars: number
    married: boolean
    retrofit: boolean
  }[]
  medals: { id: number; name: string; progress: number; target: number; claimed: number; tiers: number; shown: boolean }[]
  displayed: number
  collections: { id: number; name: string; claimed: number; total: number }[]
  handbook: { id: number; name: string; done: number; tasks: number; awards: number; awards_total: number }[]
  seen: { key: string; label: string; count: number; liked: number }[]
  music: { album: number; mode: number }
  look: LifeLook
  options: {
    ships: LifeOption[]
    oath: LifeOption[]
    specials: LifeOption[]
    icon_frames: LifeOption[]
    chat_frames: LifeOption[]
    covers: LifeOption[]
  }
}

// a unix time without the year nor leading zeros: 1/24 00:00
export function short(unix: number): string {
  if (!unix) return '-'
  const d = new Date(unix * 1000)
  return `${d.getMonth() + 1}/${d.getDate()} ${clock(unix)}`
}

// the colours of an activity category's tag: background, text
export const categoryColors: Record<string, [string, string]> = {
  login: ['#3d8bff', '#ffffff'],
  tasks: ['#2fb9f2', '#0b2a3a'],
  shop: ['#ffd84a', '#2a1a00'],
  minigame: ['#8d5cf6', '#ffffff'],
  build: ['#ff9f2e', '#2a1a00'],
  island: ['#2f9e5c', '#ffffff'],
  social: ['#e0559b', '#ffffff'],
  story: ['#5a6276', '#ffffff'],
  rerun: ['#2fb9f2', '#0b2a3a'],
  return: ['#8d96a8', '#ffffff'],
  event: ['#4a5163', '#ffffff'],
}
