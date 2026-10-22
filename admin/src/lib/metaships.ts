// A META ship's own progress: its Rigging Fortification repairs and its
// Tactical Research (internal/admin/metaships.go).

export interface MetaRepairAttr {
  // MetaCharacterConst.REPAIR_ATTRS: cannon, torpedo, air or reload
  attr: string
  done: number
  max: number
  item: { name: string; rarity: number; icon?: string }
  item_id: number
  item_num: number
  addition: number
  addition_max: number
}

export interface MetaRepairEffect {
  id: number
  progress: number
  reached: boolean
  attrs: { attr: string; value: number }[]
}

export interface MetaSkill {
  id: number
  name: string
  // 0 while locked
  level: number
  max: number
  exp: number
  exp_max: number
  // the exp each level holds, from level 1 below max
  need_exp: number[]
}

export interface MetaShip {
  meta: boolean
  repair?: {
    exp: number
    exp_max: number
    attrs: MetaRepairAttr[]
    effects: MetaRepairEffect[]
  }
  // Somatic Activation's next breakout, absent at the last
  activation?: { level: number; repair_exp: number }
  tactics?: {
    skill_id: number
    day_exp: number
    day_exp_max: number
    double_exp: number
    double_exp_max: number
    switches: number
    switches_max: number
    skills: MetaSkill[]
  }
}

export interface MetaShipEditBody {
  repairs?: Record<string, number>
  repair_full?: boolean
  skill_id?: number
  skills?: Record<number, { level: number; exp: number }>
  reset_day?: boolean
}

export const metaAttrLabels: Record<string, string> = { cannon: 'Firepower', torpedo: 'Torpedo', air: 'Aviation', reload: 'Reload' }

// the exp a research skill holds at level: 0 while locked or at its max
export const metaExpMax = (skill: MetaSkill, level: number) => (level === 0 || level >= skill.max ? 0 : (skill.need_exp[level - 1] ?? 0))

export const repairPercent = (repair: NonNullable<MetaShip['repair']>) => (repair.exp_max ? Math.floor((repair.exp / repair.exp_max) * 100) : 0)
