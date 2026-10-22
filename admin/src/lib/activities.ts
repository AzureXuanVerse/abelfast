// The Activities screen's detail of one event (internal/admin activities.go),
// cached under keys.screen(id, 'activities', key).

export interface ActivityReward {
  type: number
  id: number
  count: number
  name?: string
  rarity?: number
  icon?: string
}

export interface ActivityTask {
  id: number
  desc: string
  progress: number
  target: number
  claimed: boolean
  // a task list's task the commander was given
  held: boolean
  awards: ActivityReward[]
}

export interface ActivityGroup {
  // a cruise week (0 the seasonal tab), a task list day, 0 for a plain list
  id: number
  label: string
  locked: boolean
  tasks: ActivityTask[]
}

export interface ActivityLevel {
  level: number
  pt: number
  key: boolean
  free: ActivityReward
  paid: ActivityReward
  free_claimed: boolean
  paid_claimed: boolean
}

export interface ActivityCruise {
  item: ActivityReward
  // the server's counter, and what the pass page reads (data1)
  pt: number
  shown: number
  max: number
  level: number
  paid: boolean
  levels: ActivityLevel[]
}

export interface ActivitySigns {
  count: number
  max: number
  days: ActivityReward[][]
  last?: number
  year?: number
  month?: number
  dates: number[]
  resigns: number
}

export interface ActivityScore {
  score: number
  best: number
  rank: number
  brackets: { id: number; lower: number; upper: number }[]
}

export interface JuusPost {
  id: number
  title?: string
  ship: string
  icon?: string
  rarity: number
  liked: boolean
  read: boolean
  commented: boolean
}

export interface JuusTopic {
  id: number
  name: string
  ship: string
  icon?: string
  rarity: number
  read: boolean
  time: number
}

export type ActivityKind = '' | 'tasks' | 'cruise' | 'tasklist' | 'login7' | 'monthsign' | 'score' | 'juus'

export interface ActivityDetail {
  kind: ActivityKind
  // the client's names: week, seasonal, get, got, phase, base, bonus
  labels: Record<string, string>
  groups: ActivityGroup[]
  cruise?: ActivityCruise
  signs?: ActivitySigns
  score?: ActivityScore
  juus?: { posts: JuusPost[]; topics: JuusTopic[] }
}
