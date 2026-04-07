package main

import (
	"context"
	"log"
	_ "time/tzdata"

	"kanban/internal/app"
)

func main() {
	generator, err := app.NewGenerator()
	if err != nil {
		log.Fatalf("initialize generator: %v", err)
	}

	if err := generator.Run(context.Background()); err != nil {
		log.Fatalf("generate site: %v", err)
	}

	log.Printf("site generated successfully")
}
