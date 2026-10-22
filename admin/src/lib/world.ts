// A commander's Operation Siren in detail (internal/admin/world.go).

export interface WorldDrop {
  type: number
  id: number
  count: number
  name: string
  icon?: string
  rarity: number
}

export interface WorldItem {
  id: number
  count: number
  name: string
  icon?: string
  rarity: number
  usage: string
}

export interface WorldTarget {
  id: number
  desc: string
  cryptic: boolean
  achieved: boolean
  count: number
  need: number
}

export interface WorldStar {
  star: number
  drop: WorldDrop
  earned: boolean
  claimed: boolean
}

export interface WorldEntrance {
  id: number
  name: string
  achieved: number
  targets: WorldTarget[]
  stars: WorldStar[]
}

export interface WorldRegion {
  id: number
  name: string
  achieved: number
  total: number
  claimed: number
  stars: number
  entrances: WorldEntrance[]
}

export interface WorldPort {
  id: number
  name: string
  goods: { id: number; left: number; max: number; item: WorldDrop; price: WorldDrop }[]
  bought: number
  next_refresh: number
}

export interface WorldFleetPick {
  ships: number[]
  commanders: { pos: number; id: number }[] | null
}

export interface WorldActivationShip {
  id: number
  template_id: number
  name: string
  rarity: number
  icon: string
  level: number
  team: 'vanguard' | 'main' | 'submarine' | ''
}

export interface WorldActivation {
  map: number
  map_name: string
  entrance: number
  fleets: number
  submarine: boolean
  total_level: number
  defaults: WorldFleetPick[]
  sub_defaults?: WorldFleetPick
  ships: WorldActivationShip[]
}

export interface WorldCounters {
  exchanges: number
  exchanges_max: number
  daily_queued: number
  daily_limit: number
  daily_refresh: number
  task_finish: number
  tasks_doing: number
  tasks_max: number
  steps: number
  treasures: number
  cooldowns: number
  new_shop_bought: number
  out_shop_bought: number
}

export interface WorldDetail {
  started: boolean
  activated: boolean
  items: WorldItem[]
  catalog: WorldItem[]
  count_max: number
  counters: WorldCounters
  regions: WorldRegion[]
  ports: WorldPort[]
  activation?: WorldActivation
}
