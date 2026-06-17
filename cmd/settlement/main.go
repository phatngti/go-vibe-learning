package main

import (
	"github/master_go_cqrs/internal/settlement"

	"go.uber.org/fx"
)

func main() { fx.New(settlement.Options()).Run() }
