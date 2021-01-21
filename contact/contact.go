package contact

import (
	"errors"
	"fmt"
)

const (
	ok = 200
	created = 201
)

//Contact defines required functions for communicating with the server
type Contact interface {
	GetBeaconBytes(profile map[string]interface{}) []byte
	GetPayloadBytes(profile map[string]interface{}, payload string) ([]byte, string)
	C2RequirementsMet(profile map[string]interface{}, c2Config map[string]string) (bool, map[string]string)
	SendExecutionResults(profile map[string]interface{}, result map[string]interface{})
	GetName() string
}

// LoadedContacts contains the contact implementations
var LoadedContacts = map[string]Contact{}

func GetAvailableContactNames() []string {
	contacts := make([]string, 0, len(LoadedContacts))
	for k := range LoadedContacts {
		contacts = append(contacts, k)
	}
	return contacts
}

func GetContactByName(contactName string) (Contact, error) {
	if len(LoadedContacts) > 0 {
		if requestedContact, ok := LoadedContacts[contactName]; ok {
			return requestedContact, nil
		}
		return nil, errors.New(fmt.Sprintf("Requested contact %s not available", contactName))
	}
	return nil, errors.New("No loaded C2 contact implementations found.")
}