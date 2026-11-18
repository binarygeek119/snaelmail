package main

import (
	"fmt"
	"log"

	"snailmail/testdata/routes"
)

func main() {
	if err := routes.WriteGoldenRoutes("testdata/routes/golden.json"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Generated testdata/routes/golden.json")
}
