<script lang="ts">
  // The moderation screens' icon sidebar, marking the queues that hold
  // something. It shares the queues' queries with their pages, so a write
  // there (invalidating ['moderation']) counts again.
  import { createQuery } from '@tanstack/svelte-query'
  import IconTabs from './IconTabs.svelte'
  import { get } from '../lib/query'
  import { moderationKeys, type Flags, type ReportedContent } from '../lib/moderation'

  const reports = createQuery(() => ({ queryKey: moderationKeys.reports, queryFn: get<ReportedContent[]>('/moderation/reports') }))
  const flags = createQuery(() => ({ queryKey: moderationKeys.flags, queryFn: get<Flags>('/moderation/flags') }))

  const tabs = $derived([
    { href: '/moderation', label: 'Reports', icon: 'M5 21V4M5 4h12l-2.5 4L17 12H5', alert: (reports.data?.length ?? 0) > 0 },
    { href: '/moderation/auto-flags', label: 'Auto-flags', icon: 'M12 3.5l9 16H3zM12 10v4.5M12 17.2v.3', alert: (flags.data?.commanders.length ?? 0) > 0 },
    {
      href: '/moderation/shared',
      label: 'Shared',
      icon: 'M8.3 10.8l7.4-3.6M8.3 13.2l7.4 3.6M3.4 12a2.6 2.6 0 1 0 5.2 0a2.6 2.6 0 1 0-5.2 0M15.4 6a2.6 2.6 0 1 0 5.2 0a2.6 2.6 0 1 0-5.2 0M15.4 18a2.6 2.6 0 1 0 5.2 0a2.6 2.6 0 1 0-5.2 0',
    },
  ])
</script>

<IconTabs {tabs} label="Moderation" />
