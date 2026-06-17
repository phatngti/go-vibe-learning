package main

import (
	"log"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/orchestrator"

	"go.uber.org/fx"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
	}
	options, err := orchestrator.Options(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fx.New(options).Run()
}
