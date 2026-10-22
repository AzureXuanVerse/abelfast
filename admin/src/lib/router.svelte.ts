// A history router: the Go server answers index.html for every app path.

// path is matched against the routes, search picks within a page (e.g.
// ?ship=7 selects a ship of the dock)
export const route = $state({ path: location.pathname, search: location.search })

export function navigate(path: string) {
  const url = new URL(path, location.href)
  if (url.pathname === route.path && url.search === route.search) return
  history.pushState(null, '', path)
  const page = url.pathname !== route.path
  route.path = url.pathname
  route.search = url.search
  if (page) window.scrollTo(0, 0)
}

// redirect replaces the current path, for paths that moved
export function redirect(path: string) {
  const url = new URL(path, location.href)
  history.replaceState(null, '', path)
  route.path = url.pathname
  route.search = url.search
}

addEventListener('popstate', () => {
  route.path = location.pathname
  route.search = location.search
})

// param is a number in the URL's search, e.g. param('ship') for ?ship=7
export function param(name: string): number | null {
  const value = new URLSearchParams(route.search).get(name)
  return value && /^\d+$/.test(value) ? Number(value) : null
}

// link makes a plain <a href> navigate without a reload.
export function link(node: HTMLAnchorElement) {
  const click = (event: MouseEvent) => {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey) return
    event.preventDefault()
    navigate(node.getAttribute('href')!)
  }
  node.addEventListener('click', click)
  return { destroy: () => node.removeEventListener('click', click) }
}

// match reads a path against a pattern whose :name segments are numbers,
// e.g. match('/commanders/:id/dock/:ship', path) -> { id: 1, ship: 7 }.
export function match(pattern: string, path: string): Record<string, number> | null {
  const want = pattern.split('/')
  const got = path.replace(/\/$/, '').split('/')
  if (want.length !== got.length) return null
  const params: Record<string, number> = {}
  for (let i = 0; i < want.length; i++) {
    if (want[i].startsWith(':')) {
      if (!/^\d+$/.test(got[i])) return null
      params[want[i].slice(1)] = Number(got[i])
    } else if (want[i] !== got[i]) {
      return null
    }
  }
  return params
}
