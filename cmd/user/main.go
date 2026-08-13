package main

import (
	"github/master_go_cqrs/services/user"

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

//	@securityDefinitions.oauth2.password	OAuth2
//	@tokenUrl								http://api-gateway.master-go-cqrs.svc.cluster.local:8888/realms/backend/protocol/openid-connect/token
//	@scope.openid							OpenID Connect scope

func main() {
	fx.New(user.Options()).Run()
}
