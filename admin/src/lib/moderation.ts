// The moderation and guild screens' API types (internal/admin
// area_moderation.go, moderation_guilds.go) and query keys.

// The moderation queues' keys: a write to one invalidates ['moderation'],
// so the sidebar's alerts count again.
export const moderationKeys = {
  all: ['moderation'] as const,
  reports: ['moderation', 'reports'] as const,
  flags: ['moderation', 'flags'] as const,
  shared: ['moderation', 'shared'] as const,
}

// The guilds' keys: the list is ['guilds'], so invalidating it refreshes
// every guild screen too.
export const guildKeys = {
  all: ['guilds'] as const,
  guild: (id: number) => ['guilds', id] as const,
  ops: (id: number) => ['guilds', id, 'operations'] as const,
}

export interface ShipRef {
  group: number
  name: string
  rarity: number
  icon?: string
}

export interface ReportLine {
  commander_id: number
  name: string
  reason: string
  at: number
  // the reporter's flagship icon
  icon?: string
  rarity?: number
}

export interface ReportedContent {
  kind: 'comment' | 'gear'
  id: number
  ship: ShipRef
  author_id: number
  author: string
  author_icon?: string
  author_rarity?: number
  content: string
  posted_at: number
  reports: ReportLine[]
}

export interface MarkRow {
  id: number
  reason: number
  kind: 'battle' | 'client'
  detail: string
  at: number
}

export interface FlaggedCommander {
  id: number
  name: string
  account_id: number
  icon?: string
  rarity?: number
  online: boolean
  first: number
  last: number
  tallies: { label: string; kind: 'battle' | 'client'; count: number }[]
  marks: MarkRow[]
}

export interface Flags {
  today: number
  week: number
  commanders: FlaggedCommander[]
}

export interface SharedComment {
  id: number
  ship: ShipRef
  author_id: number
  author: string
  author_icon?: string
  author_rarity?: number
  content: string
  posted_at: number
  good: number
  bad: number
  reports: ReportLine[]
}

export interface SharedCode {
  id: number
  ship: ShipRef
  author_id: number
  author: string
  author_icon?: string
  author_rarity?: number
  code: string
  shared_at: number
  likes: number
  nonsense: number
  outdated: number
}

export interface SharedTheme {
  author_id: number
  author: string
  author_icon?: string
  author_rarity?: number
  pos: number
  name: string
  shared_at: number
  likes: number
  favs: number
}

export interface Shared {
  comments: SharedComment[]
  codes: SharedCode[]
  themes: SharedTheme[]
}

export interface GuildRow {
  id: number
  name: string
  faction: number
  policy: number
  level: number
  exp: number
  exp_next: number
  capital: number
  members: number
  max_members: number
  requests: number
  leader: string
  created_at: number
}

export interface GuildMember {
  id: number
  name: string
  level: number
  duty: number
  liveness: number
  join_time: number
  online: boolean
  last_online: number
  icon?: string
  rarity?: number
}

export interface GuildLog {
  time: number
  kind: 'joined' | 'duty' | 'left' | 'removed'
  name: string
  duty?: number
}

export interface OperationSummary {
  name: string
  open: boolean
  ended: boolean
  start_time: number
  end_time: number
  main_done: number
  main_count: number
  joined: number
}

export interface GuildDetail extends GuildRow {
  announce: string
  manifesto: string
  tech_level: number
  tech_max: number
  operation: OperationSummary | null
  roster: GuildMember[]
  logs: GuildLog[]
}

export interface GuildTech {
  group: number
  name: string
  level: number
  level_max: number
  progress: number
  exp: number
  active: boolean
}

export interface Mission {
  name: string
  position: number
  elite: boolean
  side: boolean
  progress: number
  started: boolean
  finished: boolean
  ships: number
  // [position percent, state: 0 ahead, 1 success, 2 failure]
  nodes: [number, number][]
}

export interface OperationDetail extends OperationSummary {
  boss: { name: string; hp: number; damage: number; active: boolean }
  missions: Mission[]
  ships: number
  reports: number
}

export interface RankRow {
  id: number
  name: string
  week: number
  month: number
  total: number
}

export interface GuildOps {
  id: number
  name: string
  capital: number
  members: number
  requests: { id: number; name: string; level: number; content: string; time: number; icon?: string; rarity?: number }[]
  techs: GuildTech[]
  operation: OperationDetail | null
  logistics: {
    task: string
    task_progress: number
    task_max: number
    supply_until: number
    donated_capital: number
    donated_tech: number
  }
  capital_log: { time: number; name: string; text: string; amount: number }[]
  ranks: Record<'donations' | 'missions' | 'operations', RankRow[]>
}

// GuildConst.DUTY_*
export const duties: Record<number, string> = { 1: 'Leader', 2: 'Deputy', 3: 'Officer', 4: 'Member', 5: 'Trainee' }

// GuildConst.FACTION_TYPE_*
export const factions: Record<number, string> = { 1: 'Azur Lane', 2: 'Crimson Axis' }
