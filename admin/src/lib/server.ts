// The server area's API (internal/admin/area_server.go): settings, logs and
// the inner login accounts.

export interface UpstreamVersion {
  at: number
  versions: string[]
  client_version: string
  asset_hash: string
}

export interface ServerSettings {
  region: string
  listen_addr: string
  admin_addr: string
  password_set: boolean
  sessions: number
  servers: { id: number; name: string; host: string; port: number }[]
  upstream: string
  official: string
  version: UpstreamVersion | null
  version_error?: string
  database: string
  database_bytes: number
  data_dir: string
  region_dir: string
  tables: number
  server_files: number
}

export interface LogEvent {
  seq: number
  // unix milliseconds
  time: number
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'
  scope: string
  fields: [string, string][]
  message: string
}

export interface Logs {
  events: LogEvent[]
  // the seq to ask after next time
  next: number
}

export interface AccountCommander {
  id: number
  name: string
  level: number
  icon?: string
  rarity?: number
  online: boolean
}

// The server screens' queries.
export const serverKeys = {
  settings: ['server', 'settings'] as const,
  logs: ['server', 'logs'] as const,
  accounts: ['server', 'accounts'] as const,
  account: (id: number) => ['server', 'accounts', id] as const,
}

export interface Account {
  username: string
  mail_box: string
  account_id: number
  registered_at: number
  commander: AccountCommander | null
  // the commander's secondary password
  secondary: '' | 'set' | 'locked'
}

export interface AccountDetail extends Account {
  secondary_password: { state: 'set' | 'locked'; fail_count: number; fail_cd: number; systems: number[] } | null
  codes: { code: string; at: number }[]
}

// SecondaryPWDMgr's operations a secondary password protects
export const secondarySystems: Record<number, string> = {
  1: 'Ship unlock',
  2: 'Meowfficer unlock',
  3: 'Gear disassembly',
  4: 'Transfer code',
}
