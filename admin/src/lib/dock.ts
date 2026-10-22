// The dock, fleets and meowfficers API (internal/admin/area_dock.go).

export interface DockShip {
  id: number
  template_id: number
  name: string
  rarity: number
  star: number
  max_star: number
  level: number
  locked: boolean
  married: boolean
  fleet: number
  type: string
  team: 'vanguard' | 'main' | 'submarine' | ''
  nation: string
  skin_id: number
  // the dock card art and the square icon, bundle paths for /api/assets
  icon?: string
  square_icon?: string
  created_at: number
}

export interface Dock {
  capacity: number
  ships: DockShip[]
}

export interface NamedRef {
  id: number
  name: string
  rarity?: number
  icon?: string
  count?: number
}

export interface GearRef extends NamedRef {
  plus: number
}

export interface ShipDetail extends DockShip {
  exp: number
  exp_next: number
  exp_max: number
  // exp_max at each level from 1 to level_cap
  exp_maxes?: number[]
  max_level: number
  level_cap: number
  intimacy: number
  intimacy_max: number
  energy: number
  energy_max: number
  propose: number
  // its ship group (the skins it can wear) and its default skin, worn as 0
  group: number
  default_skin: number
  stars: { template_id: number; star: number; max_level: number }[]
  skins: NamedRef[]
  retrofit: { nodes: number; done: number; full: boolean; level: number; star: number } | null
  fleets: { id: number; locked: boolean }[]
  slot: number
  skills: { id: number; name: string; level: number; max: number; type: number }[]
  gear: { pos: number; equip: GearRef | null; skin_id: number; fits: GearRef[] }[]
  stats: { name: string; value: number }[]
  in_chapter: boolean
}

export interface ShipEditBody {
  template_id?: number
  level?: number
  exp?: number
  intimacy?: number
  energy?: number
  locked?: boolean
  propose?: boolean
  skin_id?: number
  retrofit?: boolean
  fleet?: number
  slot?: number
  skills?: Record<number, number>
  gear?: Record<number, number>
}

export interface FleetShipRef {
  id: number
  name: string
  level: number
  rarity: number
  team: string
  icon?: string
  // its hull's name and number, its square icon
  type?: string
  type_id?: number
  square_icon?: string
}

export interface FleetInfo {
  id: number
  name: string
  locked: boolean
  ships: FleetShipRef[]
  meowfficers: number[]
  // the oil a sortie costs (Fleet:GetCostSum)
  oil?: number
  // while locked: the chapter that unlocks it, its map
  unlock_chapter?: string
  unlock_map?: number
  // its ships that also sit in another fleet of its group
  duplicates?: number[]
}

// a team of a chapter formation's saved fleets
export interface EliteTeam {
  kind: 'main' | 'submarine' | 'support'
  ships: FleetShipRef[]
  meowfficers: number[]
  oil?: number
  // Chapter:singleEliteFleetVertify's verdict, '' when legal
  problem?: '' | 'empty' | 'teamCount' | 'typeLimitation'
  // the hulls the chapter asks of it
  limits?: string[]
  duplicates?: number[]
}

export interface EliteFleets {
  formation: number
  chapters: string[] | null
  teams: EliteTeam[]
  // the chapter's team counts by kind
  max?: { main: number; submarine: number; support: number }
}

export interface Fleets {
  fleets: FleetInfo[]
  elite: EliteFleets[]
  activity: { id: number; fleets: FleetInfo[] }[]
  meowfficers: NamedRef[]
  // the exercise defense fleet, fleet 1's ships until set
  defense?: { ships: FleetShipRef[]; set: boolean }
}

export interface Meowfficer {
  id: number
  template_id: number
  name: string
  template_name: string
  icon?: string
  rarity: number
  nation: string
  level: number
  max_level: number
  exp: number
  exp_next: number
  locked: boolean
  fleet: number
  cattery: number
  // its skill, count its level; skill_exp toward skill_exp_next (0 at the last)
  skill: NamedRef
  skill_exp: number
  skill_exp_next: number
  skill_max_level: number
  // the EXP each level takes to the next, from Lv.1 (0 at the last)
  level_exps: number[]
  skill_level_exps: number[]
  talents: NamedRef[]
}

export interface Meowfficers {
  meowfficers: Meowfficer[]
  capacity: number
  boxes: { id: number; box: NamedRef; holds: string; begin_time: number; finish_time: number; ready: boolean }[]
  box_count: number
  bought_today: number
  bought_total: number
  cattery: {
    level: number
    exp: number
    exp_next: number
    clean: number
    clean_max: number
    slots: { id: number; meowfficer: number; style: number; cleaned: boolean; fed: boolean; played: boolean }[]
  }
  presets: { id: number; name: string; meowfficers: number[] }[]
  templates: NamedRef[]
}

// a fleet's name as the client shows it when it has none of its own
// (Fleet.DEFAULT_NAME)
export function fleetName(id: number, name = ''): string {
  if (name) return name
  if (id === 101) return 'Exercise'
  if (id >= 11) return `Submarine ${id - 10}`
  return `Fleet ${id}`
}

// the dock card's rarity frame, ship rarity 2 (N) to 6 (UR)
export const shipFrames: Record<number, string> = {
  2: 'border-[#b9c0c8]',
  3: 'border-[#78b6f0]',
  4: 'border-[#b88ae8]',
  5: 'border-[#f6cf6a]',
  6: 'border-[#ff8ec2]',
}

export const teamLabel: Record<string, string> = { vanguard: 'Vanguard', main: 'Main', submarine: 'Submarine' }
