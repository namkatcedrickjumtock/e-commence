package main

import (
	"fmt"
	"os"
)

func fm() error {
	data, err := os.ReadFile("example.txt")


	if err != nil {
		return err // probably wrap it with some context and returned it.
	}

	fmt.Println(string(data))
}
