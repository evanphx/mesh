package main

import (
	"log"

	"github.com/jessevdk/go-flags"
)

func main() {
	var (
		gen       Gen
		authGen   AuthGen
		configSet ConfigSet
	)

	parser := flags.NewNamedParser("mesh", flags.Default)
	parser.AddCommand("gen", "generate key", "Generate a key and place in into ~/.config/mesh", &gen)
	parser.AddCommand("auth-gen", "generate a new authorizer", "Generate a new authorizer and place it into ~/.config/mesh", &authGen)
	parser.AddCommand("config-set", "set a value in the config", "", &configSet)

	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}
}
