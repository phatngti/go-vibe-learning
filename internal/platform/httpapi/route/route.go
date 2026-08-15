package route

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type Registrar interface {
	Pattern(prefix *echo.Group) string
	RegisterRoutes(group *echo.Group)
}

func AsRegistrar(f any) any {
	return fx.Annotate(
		f, fx.As(new(Registrar)),
		fx.ResultTags(`group:"routes"`),
	)
}

// AsV2Registrar registers an HTTP route set below /api/v2 without changing
// the existing v1 route group or its dependency-injection contract.
func AsV2Registrar(f any) any {
	return fx.Annotate(
		f, fx.As(new(Registrar)),
		fx.ResultTags(`group:"routes_v2"`),
	)
}

func MapRoutes(prefix *echo.Group, routes []Registrar) error {
	for _, route := range routes {
		pattern := route.Pattern(prefix)
		g := prefix.Group(pattern)
		route.RegisterRoutes(g)
	}
	return nil
}
