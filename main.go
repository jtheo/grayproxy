package main

import (
	"log"
	_ "net/http/pprof"
)

func main() {
	app := new(app)
	log.Fatalf("%+v", app.run())
}
