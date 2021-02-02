package agent

import (
	"errors"
	"fmt"

	"github.com/mitre/gocat/contact"
	"github.com/mitre/gocat/output"
)

type AgentCommsChannel struct {
	address string
	c2Protocol string
	c2Key string
	contactObj contact.Contact
	validated bool
}

// AgentCommsChannel methods

func AgentCommsFactory(address string, c2Protocol string, c2Key string) (*AgentCommsChannel, error) {
	newCommsChannel := &AgentCommsChannel{}
	if err := newCommsChannel.Initialize(address, c2Protocol, c2Key); err != nil {
		return nil, err
	}
	return newCommsChannel, nil
}

// Does not attempt to check C2 contact requirements
func (a *AgentCommsChannel) Initialize(address string, c2Protocol string, c2Key string) error {
	a.address = address
	a.c2Protocol = c2Protocol
	a.c2Key = c2Key
	a.validated = false

	// Get the contact object for the specified c2 protocol
	var err error
	a.contactObj, err = contact.GetContactByName(c2Protocol)
	if err != nil {
		return err
	}
	return nil
}

func (a *AgentCommsChannel) Validate(agentProfile map[string]interface{}) (bool, map[string]string) {
	if a.contactObj == nil {
		return false, nil
	}
	valid, modifications := a.contactObj.C2RequirementsMet(agentProfile, a.GetConfig())
	a.validated = valid
	return valid, modifications
}

func (a *AgentCommsChannel) GetConfig() map[string]string {
	return map[string]string{
		"c2Name": a.c2Protocol,
		"c2Key": a.c2Key,
		"address": a.address,
	}
}

func (a *AgentCommsChannel) GetKey() string {
	return a.c2Key
}

func (a *AgentCommsChannel) GetContact() contact.Contact {
	return a.contactObj
}

func (a *AgentCommsChannel) GetAddress() string {
	return a.address
}

func (a *AgentCommsChannel) GetContactName() string {
	if a.contactObj == nil {
		return ""
	}
	return a.contactObj.GetName()
}

func (a *AgentCommsChannel) GetProtocol() string {
	return a.c2Protocol
}

func (a *AgentCommsChannel) IsValidated() bool {
	return a.validated
}

func (a *AgentCommsChannel) GetIdentifier() string {
	return getChannelIdentifier(a.c2Protocol, a.address)
}

// Agent methods

// Validate and establish a comms channel using the provided server address and C2 configuration (protocol and key).
func (a *Agent) setInitialCommsChannel(server string, c2Config map[string]string) error {
	c2Protocol, ok := c2Config["c2Name"]
	if !ok {
		return errors.New("C2 config does not contain c2 protocol. Missing key: c2Name")
	}
	c2Key, ok := c2Config["c2Key"]
	if !ok {
		c2Key = ""
	}
	return a.ValidateAndSetCommsChannel(server, c2Protocol, c2Key)
}

// Add the given comms channel to the list of validated comms channels for the agent.
func (a *Agent) addValidatedCommsChannel(commsChannel AgentCommsChannel) {
	if commsChannel.IsValidated() {
		a.validatedCommsChannels[commsChannel.GetIdentifier()] = commsChannel
	} else {
		output.VerbosePrint(fmt.Sprintf("[!] Cannot add invalid comms channel %s", commsChannel.GetIdentifier()))
	}
}

// Will return the AgentComms previously used for the given server and c2Protocol, or will return a new AgentComms
// that binds to that server/protocol pair with the provided c2 key.
func (a *Agent) GetCommunicationChannel(server string, c2Protocol string, c2Key string) (AgentCommsChannel, error) {
	commsChannelIdentifier := getChannelIdentifier(c2Protocol, server)
	commsChannel, ok := a.validatedCommsChannels[commsChannelIdentifier]
	if !ok {
		// Create new comms channel
		newChannel, err := AgentCommsFactory(server, c2Protocol, c2Key)
		if err != nil {
			return AgentCommsChannel{}, err
		}
		output.VerbosePrint(fmt.Sprintf("[*] Initialized comms channel using c2 contact %s", c2Protocol))
		return *newChannel, nil
	}
	return commsChannel, nil
}

// Validate and set the agent comms channel for the provided server, c2 protocol, and c2 key.
func (a *Agent) ValidateAndSetCommsChannel(server string, c2Protocol string, c2Key string) error {
	commsChannel, err := a.GetCommunicationChannel(server, c2Protocol, c2Key)
	if err != nil {
		return err
	}
	return a.validateAndSetCommsChannelObj(commsChannel)
}

// Validate and set the agent comms channel to the given comms channel object.
func (a *Agent) validateAndSetCommsChannelObj(commsChannel AgentCommsChannel) error {
	valid, profileModifications := commsChannel.Validate(a.GetFullProfile())
	c2Protocol := commsChannel.GetProtocol()
	output.VerbosePrint(fmt.Sprintf("[*] Attempting to validate channel %s", c2Protocol))
	if valid {
		a.setCommsChannel(commsChannel, profileModifications)
		output.VerbosePrint(fmt.Sprintf("[*] Set communication channel to %s", c2Protocol))
		return nil
	} else {
		return errors.New(fmt.Sprintf("Requirements not met for C2 channel %s for server %s", c2Protocol, commsChannel.GetAddress()))
	}
}

// Set the agent comms channel to the given comms channel object, making any provided modifications to the agent profile.
func (a *Agent) setCommsChannel(commsChannel AgentCommsChannel, profileModifications map[string]string) {
	a.addValidatedCommsChannel(commsChannel)
	if profileModifications != nil {
		a.modifyAgentConfiguration(profileModifications)
	}
	a.agentComms = commsChannel
	if a.localP2pReceivers != nil {
		for _, receiver := range a.localP2pReceivers {
			receiver.UpdateUpstreamComs(commsChannel.GetContact())
			receiver.UpdateUpstreamServer(commsChannel.GetAddress())
		}
	}
}

// Switch to a new comms channel using the same c2 address as before, but with the new provided c2 protocol.
// If a new C2 key is provided, it will be used. Otherwise, the current c2 key will continue to be used.
func (a *Agent) SwitchC2Contact(newContactName string, newKey string) error {
	// Keep same address. If new key is not specified, use same key as current comms channel
	keyToUse := a.getC2Key()
	if len(newKey) >0 {
		keyToUse = newKey
	}
	return a.ValidateAndSetCommsChannel(a.getCurrentServerAddress(), newContactName, keyToUse)
}

// If too many consecutive failures occur for the current communication method, switch to a new proxy method.
// If no proxy methods are available, or if switch fails, return error.
func (a *Agent) HandleBeaconFailure() error {
	a.failedBeaconCounter += 1
	if a.failedBeaconCounter >= beaconFailureThreshold {
		// Reset counter and try switching proxy methods
		a.failedBeaconCounter = 0
		output.VerbosePrint("[!] Reached beacon failure threshold. Attempting to switch to new peer proxy method.")
		return a.switchToFirstAvailablePeerProxyClient()
	}
	return nil
}

// Getters

func (a *Agent) GetCurrentContact() contact.Contact {
	return a.agentComms.GetContact()
}

func (a *Agent) getCurrentServerAddress() string {
	return a.agentComms.address
}

func (a *Agent) GetCurrentContactName() string {
	return a.agentComms.GetContactName()
}

func (a *Agent) getCurrentCommsProtocol() string {
	return a.agentComms.GetProtocol()
}

func (a *Agent) getC2Key() string {
	return a.agentComms.GetKey()
}

func getChannelIdentifier(protocol string, address string) string {
	return fmt.Sprintf("%s-%s", protocol, address)
}