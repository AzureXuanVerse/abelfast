// The app's server state, cached by TanStack Query.
//
// Keys are arrays. Everything about one commander lives under
// ['commander', id, ...]: its summary (the header's resources and the
// sidebar's counts) and every commander screen. A write to a commander
// invalidates that prefix, so after granting gems both the header and the
// screen refetch. Lists of several commanders, the server and the catalog
// have their own roots.

import { QueryClient, createMutation, useQueryClient } from '@tanstack/svelte-query'
import { api, ApiError } from './api'
import type { RosterRow } from './overview'
import { toast } from '../components/Toast.svelte'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      // a 4xx won't get better by asking again
      retry: (failures, error) => !(error instanceof ApiError && error.status < 500) && failures < 2,
    },
  },
})

export const keys = {
  // every query about a commander starts with this
  commander: (id: number) => ['commander', id] as const,
  summary: (id: number) => ['commander', id, 'summary'] as const,
  // a commander screen's data: keys.screen(id, 'dock'), keys.screen(id, 'dock', shipID)
  screen: (id: number, ...rest: (string | number)[]) => ['commander', id, ...rest] as const,
  commanders: ['commanders'] as const,
  online: ['online'] as const,
  // config names, ship templates...: they only change when belfast restarts
  catalog: (...rest: (string | number)[]) => ['catalog', ...rest] as const,
}

// get is a queryFn for a GET of path.
export const get =
  <T>(path: string) =>
  () =>
    api<T>('GET', path)

// rosterQuery is every commander with its flagship avatar, the one source
// of keys.commanders (Commanders, Logs' names, Broadcast's picker).
export const rosterQuery = () => ({ queryKey: keys.commanders, queryFn: get<RosterRow[]>('/overview/commanders') })

// The answer of every write: whether the commander was playing and got
// kicked first.
export interface Kicked {
  kicked: boolean
}

// a write's toast, telling when the commander was kicked for it: "Saved,
// commander kicked", or "Saved, Belfast was kicked" given their name
export const done = (what: string, commander?: string) => (answer: Kicked) =>
  answer.kicked ? (commander ? `${what}, ${commander} was kicked` : `${what}, commander kicked`) : what

// invalidateCommander refetches a commander's screens after a write and,
// when it kicked them, the lists of who is online.
export function invalidateCommander(client: QueryClient, id: number, kicked?: boolean) {
  return Promise.all([
    client.invalidateQueries({ queryKey: keys.commander(id) }),
    kicked && client.invalidateQueries({ queryKey: keys.online }),
    kicked && client.invalidateQueries({ queryKey: keys.commanders }),
  ])
}

// createCommanderMutation is a write to a commander: on success it toasts
// success (when given), invalidates everything under ['commander', id] and,
// when the commander was kicked, the online lists. Errors are toasted.
export function createCommanderMutation<TVars = void, TData = unknown>(
  id: () => number,
  fn: (vars: TVars) => Promise<TData>,
  options: { success?: string | ((data: TData, vars: TVars) => string); onSuccess?: (data: TData, vars: TVars) => void } = {},
) {
  const client = useQueryClient()
  return createMutation(() => ({
    mutationFn: fn,
    onSuccess: async (data: TData, vars: TVars) => {
      const message = typeof options.success === 'function' ? options.success(data, vars) : options.success
      if (message) toast(message)
      await invalidateCommander(client, id(), (data as Kicked | undefined)?.kicked)
      options.onSuccess?.(data, vars)
    },
    onError: (error: Error) => toast(error.message, true),
  }))
}

// A commander screen's write: a method (POST unless given) to a path under
// /commanders/{id}, and its toast with the commander's name when given.
export interface CommanderWrite {
  path: string
  done: string
  body?: unknown
  method?: string
  commander?: string
}

// createCommanderWrite is createCommanderMutation for the screens' writes,
// with run to send one: run('/akashi', 'Affection reset', { reset: true }).
export function createCommanderWrite(id: () => number) {
  const mutation = createCommanderMutation(
    id,
    ({ path, body, method = 'POST' }: CommanderWrite) => api<Kicked>(method, `/commanders/${id()}${path}`, body),
    { success: (answer, write) => done(write.done, write.commander)(answer) },
  )
  const run = (path: string, what: string, body?: unknown, method = 'POST', onSuccess?: () => void) => mutation.mutate({ path, done: what, body, method }, { onSuccess })
  return { mutation, run }
}

// createWrite is a write that isn't about one commander: it invalidates
// the keys given (e.g. [keys.online], [['guilds']]).
export function createWrite<TVars = void, TData = unknown>(
  fn: (vars: TVars) => Promise<TData>,
  invalidate: readonly (readonly unknown[])[],
  options: { success?: string | ((data: TData, vars: TVars) => string); onSuccess?: (data: TData, vars: TVars) => void } = {},
) {
  const client = useQueryClient()
  return createMutation(() => ({
    mutationFn: fn,
    onSuccess: async (data: TData, vars: TVars) => {
      const message = typeof options.success === 'function' ? options.success(data, vars) : options.success
      if (message) toast(message)
      await Promise.all(invalidate.map((queryKey) => client.invalidateQueries({ queryKey })))
      options.onSuccess?.(data, vars)
    },
    onError: (error: Error) => toast(error.message, true),
  }))
}
