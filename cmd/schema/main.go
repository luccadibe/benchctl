package main

import (
	"github.com/luccadibe/benchctl/internal"

	"github.com/invopop/jsonschema"
)

func main() {
	schema := jsonschema.Reflect(&internal.Config{})
	json, err := schema.MarshalJSON()
	if err != nil {
		panic(err)
	}
	println(string(json))
}
