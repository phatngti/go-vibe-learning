package main

import (
	"log"

	"github/master_go_cqrs/services/orchestrator"

	"go.uber.org/fx"
)

func main() {
	options, err := orchestrator.Options()
	if err != nil {
		log.Fatal(err)
	}
	fx.New(options).Run()
}
