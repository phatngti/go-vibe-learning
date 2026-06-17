package main

import (
	"github/master_go_cqrs/internal/user"

	"go.uber.org/fx"
)

//	@title						User Service API
//	@version					2.0
//	@description				User authentication, identity management, and user query service.
//	@host						localhost:8889
//	@BasePath					/api/v1
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.

func main() {
	fx.New(user.Options()).Run()
}
