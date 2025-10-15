package main

import (
	"github.com/invopop/jsonschema"
	"github.com/luccadibe/benchctl/internal/config"
)

func main() {
	schema := jsonschema.Reflect(&config.Config{})
	json, err := schema.MarshalJSON()
	if err != nil {
		panic(err)
	}
	println(string(json))
}
