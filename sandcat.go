package main

import (
	"flag"
	"strconv"

	"github.com/mitre/gocat/core"
)

/*
These default  values can be overridden during linking - server, group, and sleep can also be overridden
with command-line arguments at runtime.
*/
var (
	key       = "JWHQZM9Z4HQOYICDHW4OCJAXPPNHBA"
	server    = "http://localhost:8888"
	paw       = ""
	group     = "red"
	c2Name    = "HTTP"
	c2Key     = ""
	listenP2P = "false" // need to set as string to allow ldflags -X build-time variable change on server-side.
)

func main() {
	parsedListenP2P, err := strconv.ParseBool(listenP2P)
	if err != nil {
		parsedListenP2P = false
	}
	server := flag.String("server", server, "The FQDN of the server")
	paw := flag.String("paw", paw, "Optionally specify a PAW on intialization")
	group := flag.String("group", group, "Attach a group to this agent")
	c2 := flag.String("c2", c2Name, "C2 Channel for agent")
	delay := flag.Int("delay", 0, "Delay starting this agent by n-seconds")
	verbose := flag.Bool("v", false, "Enable verbose output")
	listenP2P := flag.Bool("listenP2P", parsedListenP2P, "Enable peer-to-peer receivers")

	flag.Parse()

	c2Config := map[string]string{"c2Name": *c2, "c2Key": c2Key}
	core.Core(*server, *group, *delay, c2Config, *listenP2P, *verbose, *paw)
}
