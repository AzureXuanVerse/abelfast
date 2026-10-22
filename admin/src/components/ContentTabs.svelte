<script lang="ts">
  // The events screens' sidebar: the schedule and the server-wide data.
  import IconTabs, { type IconTab } from './IconTabs.svelte'

  const tabs: IconTab[] = [
    { href: '/events', label: 'Events', icon: 'M5 5h14a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1zM4 10h16M8 3v4M16 3v4M8 14h2M14 14h2' },
    { href: '/events/shared', label: 'Shared', icon: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3c2.5 2.5 3.5 5.5 3.5 9s-1 6.5-3.5 9c-2.5-2.5-3.5-5.5-3.5-9s1-6.5 3.5-9z' },
  ]
</script>

<IconTabs {tabs} label="Events" />
