// The back-office's live updates: one Server-Sent Events stream (GET
// /api/live) tells what changed in the game or from another tab, and the
// queries showing it are refetched. Bursts are batched; a stream reopened
// after a drop refetches everything it may have missed, the catalog only
// when the server restarted (it changes with nothing else).
import type { QueryClient, QueryKey } from '@tanstack/svelte-query'
import { contentKeys } from './content'
import { guildKeys, moderationKeys } from './moderation'
import { overviewKeys, type Overview } from './overview'
import { get, keys } from './query'
import { serverKeys } from './server'

interface LiveEvent {
  kind: 'commander' | 'online' | 'guild' | 'area'
  id?: number
  name?: string
}

// what each event refreshes
function affected(event: LiveEvent): QueryKey[] {
  switch (event.kind) {
    case 'commander':
      // the commander's screens, the lists showing its level or name, and
      // what a player changes in passing: guilds (the game's guild writes
      // come as the member's), codes redeemed, reports, comments and shares.
      // The dashboard's counts change with logins (online) and the
      // back-office's mail (areas).
      return [keys.commander(event.id ?? 0), keys.commanders, guildKeys.all, contentKeys.codes, moderationKeys.all]
    case 'online':
      return [keys.online, keys.commanders, overviewKeys.overview]
    case 'guild':
      return [guildKeys.all]
    case 'area':
      switch (event.name) {
        case 'notices':
          return [contentKeys.notices]
        case 'codes':
          return [contentKeys.codes]
        case 'events':
          return [contentKeys.events]
        case 'moderation':
          return [moderationKeys.all]
        case 'accounts':
          return [serverKeys.accounts]
        case 'settings':
          return [serverKeys.settings]
        case 'logs':
          return [serverKeys.logs]
        case 'broadcast':
        case 'commanders':
          // a mail or a grant to many: every commander
          return [['commander'], keys.commanders, overviewKeys.overview]
      }
      return [[event.name ?? '']]
  }
}

// how long events gather before their queries are refetched
const batch = 300

// startLive opens the stream; the function returned closes it.
export function startLive(client: QueryClient): () => void {
  const pending = new Map<string, QueryKey>()
  let timer: ReturnType<typeof setTimeout> | undefined
  const flush = () => {
    timer = undefined
    for (const key of pending.values()) client.invalidateQueries({ queryKey: key })
    pending.clear()
  }

  // when the server started, as the stream last (re)opened
  const startedAt = () =>
    client
      .fetchQuery({ queryKey: overviewKeys.overview, queryFn: get<Overview>('/overview') })
      .then((overview) => overview.server.started_at)
      .catch(() => undefined)
  let started: number | undefined
  const source = new EventSource('/api/live')
  let opened = false
  source.onopen = async () => {
    if (!opened) {
      opened = true
      started = await startedAt()
      return
    }
    client.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'catalog' })
    const now = await startedAt()
    if (now === undefined || now !== started) client.invalidateQueries({ queryKey: keys.catalog() })
    started = now
  }
  source.onmessage = (message) => {
    let event: LiveEvent
    try {
      event = JSON.parse(message.data)
    } catch {
      return
    }
    for (const key of affected(event)) pending.set(JSON.stringify(key), key)
    timer ??= setTimeout(flush, batch)
  }
  return () => {
    clearTimeout(timer)
    source.close()
  }
}
