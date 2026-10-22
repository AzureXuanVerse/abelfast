// GET /commanders/{id}/meta: the META Showdown's coordinates, boss and every
// META ship's Sync Points.

export interface MetaReward {
  type: number
  id: number
  count: number
  name?: string
  rarity?: number
  icon?: string
}

export interface MetaStep {
  target: number
  claimed: boolean
  award: MetaReward
}

export interface MetaShip {
  group: number
  name: string
  icon: string
  rarity: number
  pt: number
  max: number
  owned: boolean
  // its boss template, 0 when none
  boss: number
  steps: MetaStep[]
}

export interface MetaBossChoice {
  template: number
  group: number
  name: string
  icon: string
  rarity: number
  current: boolean
  min_level: number
  max_level: number
}

export interface MetaBoss {
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

export interface MetaScreen {
  summon_pt: number
  summon_pt_max: number
  summon_pt_old: number
  summon_pt_old_max: number
  self_boss_lv: number
  archives_id: number
  boss?: MetaBoss
  boss_name?: string
  bosses: MetaBossChoice[]
  ships: MetaShip[]
}

// WorldBoss.SUPPORT_TYPE_*
export const supports = [
  { type: 2, key: 'guild_support', label: 'Guild support' },
  { type: 1, key: 'friend_support', label: 'Friend support' },
  { type: 3, key: 'world_support', label: 'World support' },
] as const
