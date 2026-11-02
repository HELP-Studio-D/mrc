package mrc

import "path"

type RouterGroup struct {
	middleware HandlersChain
	prefix     string
	engine     *Engine
}

// Handle registers a new route and its handler chain within the router group.
func (g *RouterGroup) Handle(
	route string,
	handlers ...HandlerFunc,
) {
	fullRoute := g.joinRoute(g.prefix, route)

	chain := make(
		HandlersChain,
		0,
		len(g.middleware)+len(handlers),
	)

	chain = append(chain, g.middleware...)
	chain = append(chain, handlers...)
	
	g.engine.Handle(fullRoute, chain...)
	return 
}

// Group creates a new router group with the specified prefix and middleware.
func (g *RouterGroup) Group(
    prefix string,
    handlers ...HandlerFunc,
) *RouterGroup {
    middleware := make(
        HandlersChain,
        0,
        len(g.middleware)+len(handlers),
    )

    middleware = append(middleware, g.middleware...)
    middleware = append(middleware, handlers...)

    return &RouterGroup{
        middleware: middleware,
        prefix:     g.joinRoute(g.prefix, prefix),
        engine:     g.engine,
    }
}

// Use registers middleware for the router group.
func (g *RouterGroup) Use(handlers ...HandlerFunc) {
	g.engine.mu.Lock()
	defer g.engine.mu.Unlock()

	g.middleware = append(g.middleware, handlers...)
}

func (g *RouterGroup) joinRoute(prefix, route string) string {
	return path.Join(prefix, route)
}
