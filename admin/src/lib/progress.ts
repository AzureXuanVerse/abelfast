// The timers, progress and research screens' API (internal/admin
// area_progress.go).
import { pad } from './format'

export interface TimerShip {
  id: number
  name: string
  rarity: number
  icon?: string
}

export interface Timers {
  now: number
  builds: { id: number; ship: TimerShip; pool: string; start: number; finish: number }[]
  build_points: number
  build_points_max: number
  // how many builds can run at once
  build_slots: number
  commissions: { id: number; name: string; start: number; finish: number; ships: TimerShip[] }[]
  daily_drawn: number
  daily_draws: number
  lessons: { room: number; ship: TimerShip; skill: string; book: string; start: number; finish: number }[]
  rooms: number
  quick_used: number
  quick_allowed: number
  fields: { key: number; level: number; max_level: number; upgrade_at: number; upgrade_time: number; stored: number }[]
  auto_used: number
  auto_limit: number
  auto_tickets: { type: number; count: number; expire_at: number }[]
  auto_battles: { seq: number; type: number; chapter: number; name: string; seconds: number; finish: number }[]
  research: { kind: string; key: number; name: string; sub: string; time: number; finish: number }[]
  blueprint_cold: number
  fleet_tech: { group: number; name: string; level: number; time: number; finish: number }[]
  cooldowns: { key: number; until: number }[]
  guild_wait: number
}

export interface StageRow {
  id: number
  name: string
  title: string
  progress: number
  stars: { label: string; done: boolean }[]
  pass_count: number
  defeat_count: number
  unlock_level: number
}

export interface CampaignMap {
  id: number
  title: string
  name: string
  level_limit: number
  normal: StageRow[]
  hard: StageRow[]
}

export interface Mission {
  kind: 'task' | 'weekly' | 'activity'
  act?: number
  id: number
  type: number
  name: string
  desc: string
  progress: number
  target: number
}

export interface ProgressView {
  level: number
  maps: CampaignMap[]
  sortie: {
    stage: number
    name: string
    title: string
    round: number
    fleets: number[]
    ammo: number[]
    ammo_total: number
    start: number
    kills: number
  } | null
  missions: Mission[]
  guide: number
  new_guide: number
  collection: { ship_groups: number; stories: number; trophies: number }
}

export interface Project {
  pos: number
  pool: number
  id: number
  tech: number
  name: string
  sub: string
  version: number
  rarity: number
  time: number
  finish: number
}

export interface BlueprintDev {
  level: number
  max_level: number
  fate: number
  max_fate: number
  exp: number
  need_exp: number
  item_exp: number
}

export interface BlueprintShip {
  id: number
  name: string
  rarity: number
  icon?: string
  version: number
  state: 'locked' | 'stopped' | 'running' | 'done'
  ship_id: number
  ship_level: number
  start: number
  dev: BlueprintDev | null
}

export interface FleetTechGroup {
  id: number
  name: string
  nation: number
  level: number
  max_level: number
  study: number
  study_end: number
  study_time: number
  rewarded: number
  points: number
  next_points: number
}

export interface FleetTechAttr {
  ship_type: number
  type_name: string
  attr: number
  max: number
  set: number
}

export interface ResearchView {
  now: number
  refreshed_at: number
  offers: Project[]
  queue: Project[]
  catchup_version: number
  catchup_target: string
  catchups: { version: number; group: number; name: string; number: number; max: number }[]
  blueprints: BlueprintShip[]
  blueprint_cold: number
  pursued: number
  pursued_ur: number
  fleet_tech: FleetTechGroup[]
  fleet_tech_attrs: FleetTechAttr[]
}

// TECH_NATION_ATTRS (view/technology/technologyconst.lua), by attr_type
export const techAttrs = ['', 'HP', 'FP', 'TRP', 'AA', 'AVI', 'RLD', 'ARM', 'ACC', 'EVA', 'SPD', 'LCK', 'ASW']

// Task.TYPE_* (model/vo/task/task.lua)
export function missionTag(mission: Mission): string {
  if (mission.kind === 'weekly') return 'Weekly'
  if (mission.kind === 'activity') return 'Event'
  return ({ 1: 'Main', 2: 'Branch', 3: 'Daily', 4: 'Weekly', 6: 'Event', 13: 'Weekly', 17: 'Manual', 20: 'Repeat' } as Record<number, string>)[mission.type] ?? 'Other'
}

// left is a time left, "01:12:33", "1d 06:08" past a day
export function left(seconds: number): string {
  seconds = Math.max(0, Math.floor(seconds))
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${pad(h)}:${pad(m)}`
  return `${pad(h)}:${pad(m)}:${pad(seconds % 60)}`
}

// hours is a duration as "2h 10m"
export function hours(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return h ? `${h}h ${pad(m)}m` : `${m}m`
}

// pct is how far start..finish got by now, 0 to 100
export function pct(start: number, finish: number, now: number): number {
  if (finish <= start) return now >= finish ? 100 : 0
  return Math.min(100, Math.max(0, ((now - start) / (finish - start)) * 100))
}
