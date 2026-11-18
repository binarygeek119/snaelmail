package main

import (
	"fmt"
	"snailmail/internal/auth"
)

func main() {
	password := "admin123456"
	hash, err := auth.HashPassword(password)
	if err != nil {
		panic(err)
	}
	fmt.Println(hash)
}
