// /commanders/{id}/backyard/detail and /catalog/furniture.

import type { Backyard, SocialShip } from './social'

export interface BackyardDetailShip extends SocialShip {
  floor: number
  pop_icon: number
  pop_intimacy: number
  exp: number
  // the most exp its level holds, 0 when it holds none
  exp_max: number
  max_level: number
  intimacy: number
  // what settling now would give it
  pending_exp: number
}

export interface BackyardCandidate extends SocialShip {
  // why the game's selection hides it
  busy?: string
}

export interface BackyardFurniture {
  id: number
  name: string
  icon?: string
  rarity: number
  count: number
  max: number
  placed: number
  comfortable: number
}

export interface FurnitureEntry {
  id: number
  name: string
  icon?: string
  rarity: number
  max: number
  comfortable: number
}

export interface BackyardDetail extends Omit<Backyard, 'ships'> {
  ships: BackyardDetailShip[]
  candidates: BackyardCandidate[]
  furniture_list: BackyardFurniture[]
  name_max: number
  min_slots: number
  floor_level: number
  food_extensions_max: number
  // the capacity added by 0, 1, ... extensions
  food_capacities: number[]
  pop_coins_max: number
  pop_intimacy_max: number
  shared_max: number
  tick_seconds: number
  tick_exp: number
  tick_food: number
  exp_cap: number
  pending_exp: number
  pending_food: number
  food_until: number
  load_time: number
  dorm_money: number
}
