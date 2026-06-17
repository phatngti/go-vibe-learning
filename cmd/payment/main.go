package main

import (
	"github/master_go_cqrs/internal/payment"

	"go.uber.org/fx"
)

func main() { fx.New(payment.Options()).Run() }
