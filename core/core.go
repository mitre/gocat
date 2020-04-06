package core

import (
	"crypto/tls"
	"fmt"
	"net/http"

	//"github.com/mitre/gocat/agent"
	//"github.com/mitre/gocat/output"
	//"github.com/mitre/sandcat/gocat/util"
	"../agent"
	"../output"
	"../util"
)

//Core is the main function as wrapped by sandcat.go
func Core(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	output.SetVerbose(verbose)
	output.VerbosePrint("Started sandcat in verbose mode.")
	output.VerbosePrint(fmt.Sprintf("initial delay=%d", delay))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", c2["c2Name"]))
	output.VerbosePrint(fmt.Sprintf("heartbeat channel=%s", c2["c2Name"]))

	// Initialize and run new agent.
	sandcatAgent := &agent.Agent{}
	sandcatAgent.Initialize(server, group, p2pReceiversOn)
	util.Sleep(float64(delay))
	sandcatAgent.Run(c2)
	sandcatAgent.Terminate()
}