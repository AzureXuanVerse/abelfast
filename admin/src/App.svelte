<script lang="ts">
  import type { Component } from 'svelte'
  import { QueryClientProvider } from '@tanstack/svelte-query'
  import { api, whenLoggedOut } from './lib/api'
  import { queryClient } from './lib/query'
  import { match, redirect, route } from './lib/router.svelte'
  import SectionsBar from './components/SectionsBar.svelte'
  import { startLive } from './lib/live'
  import Toast from './components/Toast.svelte'
  import Login from './pages/Login.svelte'
  import Dashboard from './pages/Dashboard.svelte'
  import Online from './pages/Online.svelte'
  import Commanders from './pages/Commanders.svelte'
  import Profile from './pages/commander/Profile.svelte'
  import Items from './pages/commander/Items.svelte'
  import Gear from './pages/commander/Gear.svelte'
  import Dock from './pages/commander/Dock.svelte'
  import Fleets from './pages/commander/Fleets.svelte'
  import Meowfficers from './pages/commander/Meowfficers.svelte'
  import Timers from './pages/commander/Timers.svelte'
  import Progress from './pages/commander/Progress.svelte'
  import Research from './pages/commander/Research.svelte'
  import Combat from './pages/commander/Combat.svelte'
  import Backyard from './pages/commander/Backyard.svelte'
  import Friends from './pages/commander/Friends.svelte'
  import Shops from './pages/commander/Shops.svelte'
  import Activities from './pages/commander/Activities.svelte'
  import Archives from './pages/commander/Archives.svelte'
  import Mail from './pages/commander/Mail.svelte'
  import Reports from './pages/moderation/Reports.svelte'
  import AutoFlags from './pages/moderation/AutoFlags.svelte'
  import Shared from './pages/moderation/Shared.svelte'
  import Guilds from './pages/guilds/Guilds.svelte'
  import GuildOps from './pages/guilds/GuildOps.svelte'
  import Notices from './pages/content/Notices.svelte'
  import Broadcast from './pages/content/Broadcast.svelte'
  import Codes from './pages/content/Codes.svelte'
  import Events from './pages/content/Events.svelte'
  import SharedData from './pages/content/SharedData.svelte'
  import Settings from './pages/server/Settings.svelte'
  import Logs from './pages/server/Logs.svelte'
  import Accounts from './pages/server/Accounts.svelte'

  let session = $state<'checking' | 'in' | 'out'>('checking')
  whenLoggedOut(() => (session = 'out'))
  // what changes elsewhere refreshes the pages open here, once logged in
  $effect(() => {
    if (session === 'in') return startLive(queryClient)
  })
  api('GET', '/session').then(
    () => (session = 'in'),
    () => (session = 'out'),
  )

  // every screen, by path; :params are numbers handed to the page as props
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const routes: [string, Component<any>][] = [
    ['/', Dashboard],
    ['/online', Online],
    ['/commanders', Commanders],
    ['/commanders/:id', Profile],
    ['/commanders/:id/items', Items],
    ['/commanders/:id/gear', Gear],
    ['/commanders/:id/dock', Dock],
    ['/commanders/:id/dock/:ship', Dock],
    ['/commanders/:id/fleets', Fleets],
    ['/commanders/:id/meowfficers', Meowfficers],
    ['/commanders/:id/timers', Timers],
    ['/commanders/:id/progress', Progress],
    ['/commanders/:id/research', Research],
    ['/commanders/:id/combat', Combat],
    ['/commanders/:id/backyard', Backyard],
    ['/commanders/:id/friends', Friends],
    ['/commanders/:id/shops', Shops],
    ['/commanders/:id/activities', Activities],
    ['/commanders/:id/archives', Archives],
    ['/commanders/:id/mail', Mail],
    ['/moderation', Reports],
    ['/moderation/auto-flags', AutoFlags],
    ['/moderation/shared', Shared],
    ['/guilds', Guilds],
    ['/guilds/:id', Guilds],
    ['/guilds/:id/operations', GuildOps],
    ['/notices', Notices],
    ['/broadcast', Broadcast],
    ['/codes', Codes],
    ['/events', Events],
    ['/events/shared', SharedData],
    ['/settings', Settings],
    ['/logs', Logs],
    ['/accounts', Accounts],
  ]

  // Wardrobe became Dock's Skins view
  $effect(() => {
    const wardrobe = match('/commanders/:id/wardrobe', route.path)
    if (wardrobe) redirect(`/commanders/${wardrobe.id}/dock${route.search || '?view=skins'}`)
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const page = $derived.by((): { component: Component<any>; params: Record<string, number> } => {
    for (const [pattern, component] of routes) {
      const params = match(pattern, route.path)
      if (params) return { component, params }
    }
    return { component: Dashboard, params: {} }
  })
</script>

<QueryClientProvider client={queryClient}>
  {#if session === 'out'}
    <Login onlogin={() => (session = 'in')} />
  {:else if session === 'in'}
    <div class="flex min-h-screen flex-col">
      {#key route.path}
        <page.component {...page.params} />
      {/key}
      <!-- the sections, on the dashboard and each section's screen only -->
      {#if route.path.split('/').filter(Boolean).length <= 1}<SectionsBar />{/if}
    </div>
  {/if}
  {#if import.meta.env.DEV}
    {#await import('@tanstack/svelte-query-devtools') then { SvelteQueryDevtools }}
      <SvelteQueryDevtools buttonPosition="bottom-left" />
    {/await}
  {/if}
</QueryClientProvider>
<Toast />
