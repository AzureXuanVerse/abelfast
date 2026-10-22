package admin

import "net/http"

// apiFunc registers a JSON API handler behind the session (see authed).
type apiFunc func(pattern string, handler func(*http.Request) (any, error))

// registerAreas registers each back-office area's API.
func (server *Server) registerAreas(api apiFunc) {
	server.registerOverview(api)
	server.registerPlayers(api)
	server.registerDock(api)
	server.registerProgress(api)
	server.registerLife(api)
	server.registerSocial(api)
	server.registerWorld(api)
	server.registerMeta(api)
	server.registerMetaShips(api)
	server.registerContent(api)
}
