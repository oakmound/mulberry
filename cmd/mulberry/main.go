package main

import (
	"fmt"
	"log"
	"os"

	"github.com/oakmound/mulberry"
	"github.com/oakmound/oak"
	"github.com/oakmound/oak/render"
	"github.com/oakmound/oak/scene"
)

func main() {
	if len(os.Args) == 1 || len(os.Args) > 2 {
		fmt.Println("Usage: mulberry <file>")
	}
	oak.Add("mulberry",
		func(string, interface{}) {
			v, err := mulberry.NewFromFile(os.Args[1])
			if err != nil {
				log.Fatal(err)
			}
			render.Draw(v)
		},
		func() bool {
			return true
		},
		func() (string, *scene.Result) {
			return "", nil
		},
	)
	oak.Init("mulberry")
}
