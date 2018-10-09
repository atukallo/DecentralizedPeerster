package main

import (
	"flag"
	"fmt"
)

// command line arguments
var (
	//UIPort     = flag.Int("UIPort", 4848, "Port, where client is launched. Client is launched on 127.0.0.1:{port}")
	//gossipAddr = flag.String("gossipAddr", "127.0.0.1:1212", "Address, where gossiper is launched")
	//name       = flag.String("name", "go_rbachev", "Gossiper name")
	//peers      = flag.String("peers", "", "Other gossipers' addresses separated with \",\" in the form ip:port")
	//simple     = flag.Bool("simple", true, "True, if mode is simple??")
)

func main() {
	flag.Parse()

	fmt.Println("client ran")
}
