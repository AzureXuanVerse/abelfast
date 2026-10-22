// The content screens' API (internal/admin/area_content.go) and query keys.
import type { Drop } from './api'
import { pad } from './format'

export const contentKeys = {
  notices: ['notices'] as const,
  codes: ['codes'] as const,
  events: ['events'] as const,
  shared: ['shared'] as const,
  // how many an audience reaches, by audience
  audience: (audience: Audience) => ['broadcast', 'audience', audience] as const,
}

// a drop with its picture, as the catalog gives it
export interface Reward extends Drop {
  icon?: string
  rarity?: number
}

// what the API takes of rewards
export const plainDrops = (rewards: Reward[]): Drop[] => rewards.map(({ type, id, count }) => ({ type, id, count }))

export interface Audience {
  kind: 'everyone' | 'online' | 'level' | 'registered' | 'picked'
  level_min: number
  level_max: number
  // YYYY-MM-DD, server clock
  registered_before: string
  ids: number[]
}

export interface Code {
  code: string
  title: string
  sender: string
  content: string
  rewards: Reward[]
  // unix seconds, 0 without a window
  start: number
  end: number
  status: 'active' | 'scheduled' | 'ended'
  uses: number
}

export interface Redemption {
  at: number
  commander_id: number
  name: string
  icon?: string
  rarity?: number
  code: string
  rewards: Reward[]
}

export interface Codes {
  codes: Code[]
  redemptions: Redemption[]
  offset: number
}

export interface Activity {
  id: number
  type: number
  name: string
  shown: boolean
  start: number
  end: number
  status: 'live' | 'upcoming' | 'ended'
}

export interface Events {
  region: string
  data: string
  activities: Activity[]
  shops: { name: string; rule: string; next: number }[]
  pools: { id: number; name: string; rates: Record<string, number>; ships: number }[]
  clock: { now: number; zone: string; offset: number; next_day: number; next_week: number; next_month: number }
}

export interface Boss {
  activity_id: number
  boss_id: number
  collab: boolean
  series: number
  name: string
  open: boolean
  // left, in 1/10000
  hp: number
  marks: { mark: number; reached_at: number }[]
}

export interface Survey {
  id: number
  name: string
  open: boolean
  taken: number
  latest: number
}

export interface Shared {
  region: string
  data: string
  bosses: Boss[]
  surveys: Survey[]
  commanders: number
  offset: number
}

// A time on the server clock (offset seconds east of UTC), as the fields
// of a UTC date.
function shifted(unix: number, offset: number): Date {
  return new Date((unix + offset) * 1000)
}

// 2026-10-21
export function serverDay(unix: number, offset: number): string {
  const d = shifted(unix, offset)
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`
}

// 2026-10-21 14:32
export function serverDate(unix: number, offset: number): string {
  const d = shifted(unix, offset)
  return `${serverDay(unix, offset)} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`
}

// 10/21
export function serverMonthDay(unix: number, offset: number): string {
  const d = shifted(unix, offset)
  return `${d.getUTCMonth() + 1}/${d.getUTCDate()}`
}

// 10-21 09:42
export function serverShort(unix: number, offset: number): string {
  const d = shifted(unix, offset)
  return `${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`
}

// UTC-7
export function zoneName(offset: number): string {
  const hours = offset / 3600
  return hours === 0 ? 'UTC' : `UTC${hours > 0 ? '+' : ''}${hours}`
}

// ActivityConst.ACTIVITY_TYPE_* (model/const/activityconst.lua), the
// activities' kinds.
const activityTypes: Record<number, string> = {
  1: 'Build', 3: '7-day login', 4: 'Level award', 5: 'Level plan', 6: 'Month sign-in', 7: 'Exercise', 8: 'Progress login',
  9: 'Charge award', 11: 'Event', 12: 'Z project', 13: 'Tasks', 14: 'Shop', 15: 'Collection', 16: 'PT rank', 18: 'Task list',
  19: 'Vote', 20: 'BB', 21: 'Puzzle', 22: 'Lottery', 24: 'Challenge rank', 26: 'Build', 27: 'Shop street', 29: 'Love letter',
  30: 'Buff', 32: 'Hit monster', 33: 'Card pairs', 36: 'Extra chapter rank', 37: 'Challenge', 38: 'Summary', 39: 'Task monitor',
  40: 'Task reward', 41: 'Virtual bag', 42: 'Reflux', 43: 'Coloring', 44: 'Guide tasks', 45: 'Link link', 46: 'Boss rank',
  48: 'Shop discount', 49: 'Lottery award', 50: 'Dodgem', 51: 'Monopoly', 52: 'Boss battle', 53: 'Black white', 54: 'Cruise pass',
  55: 'Build pray', 56: 'Return award', 57: 'Mini-game', 58: 'Ship buff', 59: 'Story award', 60: 'Juustagram', 61: 'Turntable',
  63: 'Shrine', 64: 'Red packets', 66: 'Pizza PT', 67: 'Building buff', 68: 'Skin coupon', 69: 'Blueprint catch-up',
  70: 'Client display', 71: 'Tech catch-up', 72: 'Submarine run', 73: 'Expedition', 74: 'PT', 75: 'Air fight', 76: 'Collection',
  77: 'Shop progress', 78: 'Shake beads', 79: 'World in picture', 80: 'Free build', 81: 'April reward', 82: 'New server tasks',
  83: 'New server shop', 84: 'New server gift', 85: 'New server build', 86: 'Task pool', 87: 'PT', 88: 'Atelier', 89: 'Tasks',
  90: 'Red packet lottery', 91: 'Hot spring', 92: 'Firework', 93: 'Boss rush', 94: 'Boss rush rank', 95: 'Island', 96: 'Workbench',
  97: 'Building buff', 98: 'Flower field', 99: 'Sculpture', 100: 'Hot spring', 101: 'Survey', 102: 'Card puzzle',
  103: 'Link collect', 104: 'Feast', 105: 'Zuma', 106: 'Castle', 107: 'Train', 108: 'Gift up', 109: 'Manual sign-in',
  110: 'Random daily task', 111: 'Single boss', 112: 'Single event', 113: 'Store date', 114: 'Liner', 115: 'UR exchange',
  116: 'Town', 118: 'Dreamland', 119: 'Selectable shop', 120: 'Black Friday shop', 121: 'Not traceable', 123: 'Single boss',
  124: 'Skin coupon', 125: 'Holiday villa', 126: 'Daily stage bonus', 127: 'Stronghold', 128: 'Buff', 129: 'City rebuild',
  130: 'PT', 131: 'Skin package', 132: 'Collab boss rush', 133: 'Love letter', 134: 'Love letter mail', 135: 'Town',
  136: 'Auction', 137: 'Reverse pacman', 140: 'Mall', 141: 'Login record', 142: 'Package', 174: 'PT', 800: 'Island draw',
  801: 'Island cheat bar', 890: 'Island PT', 999: 'Island tavern', 1001: 'Puzzle connect',
}

export function activityType(type: number): string {
  return activityTypes[type] ?? `Type ${type}`
}

// ACTIVITY_SNAKE_NAME -> Activity snake name; a page class stays as it is.
export function activityName(activity: Activity): string {
  if (!activity.name) return `Activity ${activity.id}`
  if (!/^[A-Z0-9_]+$/.test(activity.name)) return activity.name
  const words = activity.name.replace(/^ACTIVITY_(ID_)?/, '').toLowerCase().split('_').filter(Boolean)
  const text = words.join(' ')
  return text ? text[0].toUpperCase() + text.slice(1) : activity.name
}
