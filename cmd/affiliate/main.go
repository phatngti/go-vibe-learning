package main

import (
	"github/master_go_cqrs/internal/affiliate"

	"go.uber.org/fx"
)

func main() { fx.New(affiliate.Options()).Run() }
