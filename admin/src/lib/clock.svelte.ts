// A clock for the screens that count down: now, in unix seconds, ticks
// every ms. Given serverNow (the server's time as its last answer read it),
// it keeps the server's whole seconds instead of the browser's.
export function createNow(ms: number, serverNow?: () => number | undefined) {
  const browser = () => (serverNow ? Math.floor(Date.now() / 1000) : Date.now() / 1000)
  let offset = 0
  let now = $state(browser())
  $effect(() => {
    const server = serverNow?.()
    if (server === undefined) return
    offset = server - Math.floor(Date.now() / 1000)
    now = server
  })
  $effect(() => {
    const timer = setInterval(() => (now = browser() + offset), ms)
    return () => clearInterval(timer)
  })
  return {
    get now() {
      return now
    },
  }
}
