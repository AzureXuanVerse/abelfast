export function ago(unix: number, now = Date.now() / 1000): string {
  if (!unix) return 'never'
  const seconds = Math.max(0, now - unix)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} h ago`
  return `${Math.floor(seconds / 86400)} d ago`
}

export function date(unix: number): string {
  return unix ? new Date(unix * 1000).toLocaleString() : '-'
}

export function count(n: number): string {
  return n.toLocaleString('en-US')
}

export function size(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`
  if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MB`
  return `${Math.ceil(bytes / 1024)} KB`
}

// tier is a rarity frame, 1 (grey) to 5 (rainbow); ships' rarity starts at 2.
export function tier(rarity: number | undefined, ship = false): number {
  const t = (rarity ?? 1) - (ship ? 1 : 0)
  return Math.min(5, Math.max(1, t))
}

// The screens' unix times, local, '-' for none.
export const pad = (n: number) => String(n).padStart(2, '0')

// 2026-10-24
export function day(unix: number): string {
  if (!unix) return '-'
  const at = new Date(unix * 1000)
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`
}

// 14:32, 14:32:05 with the seconds
export function clock(unix: number, seconds = false): string {
  if (!unix) return '-'
  const at = new Date(unix * 1000)
  return `${pad(at.getHours())}:${pad(at.getMinutes())}` + (seconds ? `:${pad(at.getSeconds())}` : '')
}

// 2026-10-24 14:32
export function stamp(unix: number): string {
  return unix ? `${day(unix)} ${clock(unix)}` : '-'
}

// 10-24 14:32 without the year, 10/24 14:32 with sep '/'
export function short(unix: number, sep = '-'): string {
  if (!unix) return '-'
  const at = new Date(unix * 1000)
  return `${pad(at.getMonth() + 1)}${sep}${pad(at.getDate())} ${clock(unix)}`
}

// short, with the year when it isn't this one
export function recent(unix: number): string {
  return unix && new Date(unix * 1000).getFullYear() !== new Date().getFullYear() ? stamp(unix) : short(unix)
}
