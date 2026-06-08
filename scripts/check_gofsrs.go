package main

import (
	"fmt"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

func main() {
	p := gofsrs.DefaultParam()
	fmt.Printf("Default weights length: %d\n", len(p.W))
	fmt.Printf("Default weights: %v\n", p.W)
}
