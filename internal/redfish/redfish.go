package redfish

import (
	"fmt"
	"github.com/metal-stack/go-hal"
	"log"

	"github.com/metal-stack/go-hal/pkg/api"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

const defaultUUID = "00000000-0000-0000-0000-000000000000"

type APIClient struct {
	*gofish.APIClient
}

func New(url, user, password string, insecure bool) (*APIClient, error) {
	// Create a new instance of gofish client, ignoring self-signed certs
	config := gofish.ClientConfig{
		Endpoint: url,
		Username: user,
		Password: password,
		Insecure: insecure,
	}
	c, err := gofish.Connect(config)
	if err != nil {
		return nil, err
	}
	return &APIClient{
		APIClient: c,
	}, nil
}

func (c *APIClient) BoardInfo() (*api.Board, error) {
	// Query the chassis data using the session token
	chassis, err := c.Service.Chassis()
	if err != nil {
		return nil, err
	}

	for _, chass := range chassis {
		log.Printf("cass:%v\n", chass)
		log.Printf("Model:" + chass.Model + " Name:" + chass.Name + " Part:" + chass.PartNumber + " Serial:" + chass.SerialNumber + " Version:" + chass.Version + " SKU:" + chass.SKU + "\n")
		if chass.ChassisType == redfish.RackMountChassisType {
			return &api.Board{
				VendorString: chass.Manufacturer,
				Model:        chass.Model,
				PartNumber:   chass.PartNumber,
				SerialNumber: chass.SerialNumber,
			}, nil
		}
	}
	return nil, fmt.Errorf("no board detected")
}

// MachineUUID retrieves a unique uuid for this (hardware) machine
func (c *APIClient) MachineUUID() (string, error) {
	systems, err := c.Service.Systems()
	if err != nil {
		return defaultUUID, err
	}
	for _, system := range systems {
		log.Printf("system:%v\n", system)
		if system.UUID != "" {
			return system.UUID, nil
		}
	}
	return defaultUUID, err
}

func (c *APIClient) PowerState() (hal.PowerState, error) {
	systems, err := c.Service.Systems()
	if err != nil {
		return hal.PowerUnknownState, err
	}
	for _, system := range systems {
		if system.PowerState != "" {
			return hal.GuessPowerState(string(system.PowerState)), nil
		}
	}
	return hal.PowerUnknownState, nil
}
