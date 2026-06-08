package main

import (
	"github/master_go_cqrs/internal/app"

	"go.uber.org/fx"
)

//	@title						Master Go CQRS API
//	@version					2.0
//	@description				HTTP API and service route documentation generated from DTO-backed handlers.
//	@host						localhost:8889
//	@BasePath					/api/v1
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.

func main() {
	fx.New(app.Options()).Run()
}
