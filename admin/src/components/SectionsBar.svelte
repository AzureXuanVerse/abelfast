<script lang="ts">
  // The back-office's sections, along the bottom of the dashboard and of
  // each section's own screen (depth 1); deeper screens go without it.
  import { link, route } from '../lib/router.svelte'

  const sections = [
    { label: 'Online now', href: '/online', path: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M4 21c0-4 3.6-6.5 8-6.5s8 2.5 8 6.5' },
    { label: 'Commanders', href: '/commanders', path: 'M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z M2.5 20c0-3.5 2.9-5.5 6.5-5.5s6.5 2 6.5 5.5 M16 11a3 3 0 1 0 0-6 M17.5 14.6c2.4.5 4 2.3 4 5.4' },
    { label: 'Moderation', href: '/moderation', path: 'M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6z M9 12l2 2 4-4' },
    { label: 'Guilds', href: '/guilds', path: 'M5 21V4 M5 4h12l-2 4 2 4H5' },
    { label: 'Notices', href: '/notices', path: 'M6 3h9l4 4v14H6z M14 3v5h5 M9 12h7 M9 16h7' },
    { label: 'Broadcast', href: '/broadcast', path: 'M4 10v4h3l7 4V6l-7 4z M17.5 9a4 4 0 0 1 0 6 M20 6.5a8 8 0 0 1 0 11' },
    { label: 'Codes', href: '/codes', path: 'M3 7h18v3a2 2 0 0 0 0 4v3H3v-3a2 2 0 0 0 0-4z M14 7v10' },
    { label: 'Events', href: '/events', path: 'M4 5h16v15H4z M4 9h16 M8 3v4 M16 3v4 M12 11.5l1.2 2.4 2.6.4-1.9 1.8.5 2.6-2.4-1.3-2.4 1.3.5-2.6-1.9-1.8 2.6-.4z' },
    { label: 'Accounts', href: '/accounts', path: 'M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M2.5 21c0-4 2.9-6.5 6.5-6.5 1.4 0 2.6.3 3.6.9 M16.5 15.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z M18.3 14.8l3.2 3.2 M20 19.5l1.5-1.5' },
    { label: 'Settings', href: '/settings', path: 'M12 8.8a3.2 3.2 0 1 0 0 6.4 3.2 3.2 0 0 0 0-6.4z M12 2.5v3 M12 18.5v3 M2.5 12h3 M18.5 12h3 M5.3 5.3l2.1 2.1 M16.6 16.6l2.1 2.1 M5.3 18.7l2.1-2.1 M16.6 7.4l2.1-2.1' },
    { label: 'Logs', href: '/logs', path: 'M5 3h14v18H5z M8.5 8h7 M8.5 12h7 M8.5 16h4' },
  ]
</script>

<nav aria-label="Sections" class="ip-tabs sticky bottom-0 z-10 mt-auto justify-between px-5 py-2.5">
  {#each sections as section (section.href)}
    <a href={section.href} use:link aria-current={route.path === section.href ? 'page' : undefined} class="px-[9px] text-base">
      <svg aria-hidden="true" viewBox="0 0 24 24" class="size-[22px]" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d={section.path} /></svg>
      <span>{section.label}</span>
    </a>
  {/each}
</nav>
