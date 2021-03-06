package api

import (
	"fmt"
	"github.com/metal-stack/go-hal/internal/kernel"
	"strings"
)

// Board raw dmi board information
type Board struct {
	VendorString string
	Vendor       Vendor
	Model        string
	PartNumber   string
	SerialNumber string
	BiosVersion  string
	BMC          *BMC
	BIOS         *BIOS
	Firmware     kernel.FirmwareMode
}

// BMC Base Management Controller details
type BMC struct {
	IP                  string
	MAC                 string
	ChassisPartNumber   string
	ChassisPartSerial   string
	BoardMfg            string
	BoardMfgSerial      string
	BoardPartNumber     string
	ProductManufacturer string
	ProductPartNumber   string
	ProductSerial       string
	FirmwareRevision    string `ipmitool:"Firmware Revision"`
}

// BIOS information of this machine
type BIOS struct {
	Version string
	Vendor  string
	Date    string
}

var (
	VagrantBoard = &Board{
		VendorString: "vagrant",
		Vendor:       0,
		Model:        "vagrant",
		PartNumber:   "vagrant",
		SerialNumber: "vagrant",
		BiosVersion:  "0",
		BMC: &BMC{
			IP:                  "1.1.1.1",
			MAC:                 "aa:bb:cc:dd:ee:ff",
			ChassisPartNumber:   "vagrant",
			ChassisPartSerial:   "vagrant",
			BoardMfg:            "vagrant",
			BoardMfgSerial:      "vagrant",
			BoardPartNumber:     "vagrant",
			ProductManufacturer: "vagrant",
			ProductPartNumber:   "vagrant",
			ProductSerial:       "vagrant",
			FirmwareRevision:    "vagrant",
		},
		BIOS: &BIOS{
			Version: "0",
			Vendor:  "vagrant",
			Date:    "01/01/2020",
		},
		Firmware: 0,
	}
)

type Compliance int

const (
	IPMI2Compliance Compliance = iota
	SMCIPMIToolCompliance
)

type (
	// Vendor identifies different server vendors
	Vendor int
)

const (
	// VendorUnknown is a unknown Vendor
	VendorUnknown Vendor = iota
	// VendorSupermicro identifies all Supermicro servers
	VendorSupermicro
	// VendorLenovo identifies all Lenovo servers
	VendorLenovo
	// VendorDell identifies all Dell servers
	VendorDell
	// VendorVagrant is a virtual machine.
	VendorVagrant
)

var (
	vendors = [...]string{
		VendorSupermicro: "Supermicro",
		VendorLenovo:     "Lenovo",
		VendorDell:       "Dell",
		VendorVagrant:    "Vagrant",
		VendorUnknown:    "UNKNOWN",
	}
	allVendors = [...]Vendor{VendorSupermicro, VendorLenovo, VendorDell, VendorVagrant, VendorUnknown}
)

func (v Vendor) String() string { return vendors[v] }

// GuessVendor will try to guess from vendor string
func GuessVendor(vendor string) Vendor {
	for _, v := range allVendors {
		if strings.Contains(strings.ToLower(v.String()), strings.ToLower(vendor)) {
			return v
		}
	}
	return VendorUnknown
}

func (b *Board) String() string {
	return fmt.Sprintf("Vendor:%s Name:%s", b.Vendor, b.Model)
}

func (b *BIOS) String() string {
	return "version:" + b.Version + " vendor:" + b.Vendor + " date:" + b.Date
}
