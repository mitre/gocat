package core

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/mitre/gocat/agent"
	"github.com/mitre/gocat/contact"
	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/util"

	_ "github.com/mitre/gocat/execute/shells" // necessary to initialize all submodules
)

// Initializes and returns sandcat agent.
func initializeCore(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool) *agent.Agent {
	output.SetVerbose(verbose)
	output.VerbosePrint("Starting sandcat in verbose mode.")
	output.VerbosePrint(fmt.Sprintf("initial delay=%d", delay))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", c2["c2Name"]))
	output.VerbosePrint(fmt.Sprintf("heartbeat channel=%s", c2["c2Name"]))
	util.Sleep(float64(delay))
	sandcatAgent := &agent.Agent{}
	sandcatAgent.Initialize(server, group, p2pReceiversOn)
	return sandcatAgent
}

//Core is the main function as wrapped by sandcat.go
func Core(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	sandcatAgent := initializeCore(server, group, delay, c2, p2pReceiversOn, verbose)
	runAgent(sandcatAgent, c2)
	sandcatAgent.Terminate()
}

// Establish contact with C2 and run instructions.
func runAgent (sandcatAgent *agent.Agent, c2Config map[string]string) {
	// Set communication channels.
	for {
		if err := sandcatAgent.SetCommunicationChannels(c2Config); err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] %s", err.Error()))
			util.Sleep(300)
		} else {
			output.VerbosePrint("[*] Set communication channels for sandcat agent.")
			break
		}
	}

	// Start main execution loop.
	watchdog := 0
	checkin := time.Now()
	for (util.EvaluateWatchdog(checkin, watchdog)) {
		// TODO - heartbeat will be incorporated later

		// Send beacon and get response.
		beacon := sandcatAgent.Beacon()

		// Process beacon response.
		if len(beacon) != 0 {
			sandcatAgent.Paw = beacon["paw"].(string)
			checkin = time.Now()

			// We have established comms. Run p2p receivers if allowed.
			// TODO
		}
		if beacon["instructions"] != nil && len(beacon["instructions"].([]interface{})) > 0 {
			// Run commands and send results.
			cmds := reflect.ValueOf(beacon["instructions"])
			for i := 0; i < cmds.Len(); i++ {
				cmd := cmds.Index(i).Elem().String()
				command := util.Unpack([]byte(cmd))
				output.VerbosePrint(fmt.Sprintf("[*] Running instruction %s", command["id"]))
				droppedPayloads := contact.DownloadPayloads(sandcatAgent.GetTrimmedProfile(), command["payloads"].([]interface{}), sandcatAgent.BeaconContact)
				go sandcatAgent.RunInstruction(command, droppedPayloads)
				util.Sleep(command["sleep"].(float64))
			}
		} else {
			if len(beacon) > 0 {
				util.Sleep(float64(beacon["sleep"].(int)))
				watchdog = beacon["watchdog"].(int)
			} else {
				util.Sleep(float64(15))
			}
		}
	}
}