// A commander's combat, backyard and friends (internal/admin/social.go).

export interface SocialShip {
  id: number
  template_id: number
  name: string
  rarity: number
  icon: string
  level: number
}

export interface ExerciseRank {
  id: number
  name: string
  point: number
  order: number
}

export interface Exercise {
  season: number
  season_end: number
  score: number
  standing: number
  rank: ExerciseRank
  next?: ExerciseRank
  best_rank: number
  fight_count: number
  fight_max: number
  refilled_at: number
  next_refill: number
  flash_count: number
  flash_max: number
  shop_refreshes: number
  vanguard: SocialShip[]
  main: SocialShip[]
  rivals: { id: number; name: string; level: number; icon: string; rarity: number; score: number; standing: number; rank: string }[]
  ranks: ExerciseRank[]
}

export interface World {
  started: boolean
  activated: boolean
  progress: number
  reset_stage: number
  expired_time: number
  stamina: number
  stamina_max: number
  atlas_maps: number
  cleared: number
  maps: { random_id: number; name: string; clear: boolean; active: boolean }[]
  resettable: boolean
}

export interface Meta {
  summon_pt: number
  summon_pt_old: number
  self_boss_lv: number
  current_ship: number
  current_name: string
  boss_name?: string
  boss?: {
    template_id: number
    meta_ship: number
    level: number
    hp: number
    max_hp: number
    expires_at: number
    killed_at: number
    fights: number
    damage: number
    guild_support: boolean
    friend_support: boolean
    world_support: boolean
  }
  groups: { group: number; name: string; icon: string; rarity: number; pt: number; claimed: number; steps: number; max: number }[]
}

export interface Combat {
  exercise: Exercise
  world: World
  meta: Meta
}

export interface BackyardShip extends SocialShip {
  floor: number
  pop_icon: number
  pop_intimacy: number
}

export interface Backyard {
  name: string
  level: number
  max_level: number
  floors: number
  max_floors: number
  slots: number
  max_slots: number
  food: number
  food_max: number
  food_extensions: number
  comfort: number
  furniture: number
  next_settle: number
  ships: BackyardShip[]
  floor_list: { floor: number; placed: number; starter: boolean }[]
  themes: { pos: number; name: string; pieces: number; shared_at: number; likes: number; favs: number }[]
  levels: { id: number; capacity: number; slots: number }[]
}

export interface FriendRow {
  id: number
  name: string
  level: number
  icon: string
  rarity: number
  online: boolean
  last_online?: number
  since?: number
  content?: string
  sent_at?: number
  blocked_at?: number
}

export interface Friends {
  friends: FriendRow[]
  requests: FriendRow[]
  blacklist: FriendRow[]
  max: number
}
