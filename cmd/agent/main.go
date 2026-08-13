package main

import (
	"github/master_go_cqrs/services/agent"

	"go.uber.org/fx"
)

func main() { fx.New(agent.Options()).Run() }
