package main

type Gen struct {
	Network string `short:"n" long:"network" description:"name to associate with key"`
}

func (g *Gen) Execute(args []string) error {
	return nil
}
