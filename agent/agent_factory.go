package agent

// Creates and initializes a new Agent. Upon success, returns a pointer to the agent and nil Error.
// Upon failure, returns nil and an error.
func AgentFactory(server string, group string, c2Config map[string]string, enableP2pReceivers bool, initialDelay int) (*Agent, error) {
	newAgent := &Agent{}
	if err := newAgent.Initialize(server, group, c2Config, enableP2pReceivers, initialDelay); err != nil {
		return nil, err
	} else {
		newAgent.Sleep(newAgent.initialDelay)
		return newAgent, nil
	}
}