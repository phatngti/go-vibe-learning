package main

import (
	"github/master_go_cqrs/services/settlement"

	"go.uber.org/fx"
)

func main() { fx.New(settlement.Options()).Run() }
