package main

import (
	"github/master_go_cqrs/services/payment"

	"go.uber.org/fx"
)

func main() { fx.New(payment.Options()).Run() }
