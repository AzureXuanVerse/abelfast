// The dashboard, online now and the commander roster (internal/admin/area_overview.go).
import type { CommanderRow } from './api'

export interface ServerStatus {
  // the game server address handed out in the server list
  address: string
  region: string
  started_at: number
  // from the latest SC_10801 relayed from the official gateway
  client_version: string
  asset_hash: string
  assets_at: number
  assets_error?: string
  database: string
  database_bytes: number
}

export interface Activity {
  kind: 'new' | 'login' | 'logout' | 'mail'
  time: number
  actor: string
  actor_id?: number
  what: string
}

export interface Overview {
  server: ServerStatus
  counts: { online: number; commanders: number; new_today: number; mails_today: number }
  day_start: number
  next_reset: number
  activity: Activity[]
}

export interface RosterRow extends CommanderRow {
  // the flagship's squareicon bundle and its rarity
  icon?: string
  rarity?: number
  // online now only: exp into the level and the level's exp to the next
  exp?: number
  exp_next?: number
}

// The dashboard's queries (the roster and online now are keys.commanders
// and keys.online, shared with the other screens).
export const overviewKeys = {
  overview: ['overview'] as const,
  session: ['session'] as const,
}
