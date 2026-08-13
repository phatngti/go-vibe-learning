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

func MapRoutes(prefix *echo.Group, routes []Registrar) error {
	for _, route := range routes {
		pattern := route.Pattern(prefix)
		g := prefix.Group(pattern)
		route.RegisterRoutes(g)
	}
	return nil
}
