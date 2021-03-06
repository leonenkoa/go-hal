package ipmi

// IPMI Wiki
// https://www.thomas-krenn.com/de/wiki/IPMI_Konfiguration_unter_Linux_mittels_ipmitool
//
// Oder:
// https://wiki.hetzner.de/index.php/IPMI

import (
	"bufio"
	"fmt"
	"github.com/metal-stack/go-hal"
	"github.com/vmware/goipmi"
	"log"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/metal-stack/go-hal/internal/password"
	"github.com/metal-stack/go-hal/pkg/api"
	"github.com/pkg/errors"
)

// Privilege of an ipmitool user
type Privilege int

const (
	// Callback ipmi privilege
	Callback = Privilege(1)
	// User ipmi privilege
	User = Privilege(2)
	// Operator ipmi privilege
	Operator = Privilege(3)
	// Administrator ipmi privilege
	Administrator = Privilege(4)
	// OEM ipmi privilege
	OEM = Privilege(5)
	// NoAccess ipmi privilege
	NoAccess = Privilege(15)
)

// Ipmi defines methods to interact with ipmi
type Ipmi interface {
	DevicePresent() bool
	run(arg ...string) (string, error)
	CreateUser(username, uid string, privilege Privilege) (string, error)
	GetLanConfig() (LanConfig, error)
	SendBootOrderRaw(target hal.BootTarget) error
	SendChassisControlRaw(chassisControl ipmi.ChassisControl) error
	SendChassisIdentifyRaw(intervalOrOff, forceOn string) error
	GetFru() (Fru, error)
	GetSession() (Session, error)
	BMC() (*api.BMC, error)
}

// Ipmitool is used to query and modify the IPMI based BMC from the host os.
type Ipmitool struct {
	command    string
	compliance api.Compliance
}

// LanConfig contains the config of IPMI.
// tag must contain first column name of ipmitool lan print command output
// to get the second column value be parsed into the field.
type LanConfig struct {
	IP  string `ipmitool:"IP Address"`
	Mac string `ipmitool:"MAC Address"`
}

func (l *LanConfig) String() string {
	return fmt.Sprintf("ip: %s mac:%s", l.IP, l.Mac)
}

// Session holds information about the current IPMI session
type Session struct {
	UserID    string `ipmitool:"user id"`
	Privilege string `ipmitool:"privilege level"`
}

// Fru contains Field Replacable Unit information, retrieved with ipmitool fru
type Fru struct {
	// 	FRU Device Description : Builtin FRU Device (ID 0)
	//  Chassis Type          : Other
	//  Chassis Part Number   : CSE-217BHQ+-R2K22BP2
	//  Chassis Serial        : C217BAH31AG0535
	//  Board Mfg Date        : Mon Jan  1 01:00:00 1996
	//  Board Mfg             : Supermicro
	//  Board Product         : NONE
	//  Board Serial          : HM187S003231
	//  Board Part Number     : X11DPT-B
	//  Product Manufacturer  : Supermicro
	//  Product Name          : NONE
	//  Product Part Number   : SYS-2029BT-HNTR
	//  Product Version       : NONE
	//  Product Serial        : A328789X9108135

	ChassisPartNumber   string `ipmitool:"Chassis Part Number"`
	ChassisPartSerial   string `ipmitool:"Chassis Serial"`
	BoardMfg            string `ipmitool:"Board Mfg"`
	BoardMfgSerial      string `ipmitool:"Board Mfg Serial"`
	BoardPartNumber     string `ipmitool:"Board Part Number"`
	ProductManufacturer string `ipmitool:"Product Manufacturer"`
	ProductPartNumber   string `ipmitool:"Product Part Number"`
	ProductSerial       string `ipmitool:"Product Serial"`
}

// BMCInfo contains the parsed output of ipmitool bmc info
type BMCInfo struct {
	// # ipmitool bmc info
	// Device ID                 : 32
	// Device Revision           : 1
	// Firmware Revision         : 1.64
	// IPMI Version              : 2.0
	// Manufacturer ID           : 10876
	// Manufacturer Name         : Supermicro
	// Product ID                : 2402 (0x0962)
	// Product Name              : Unknown (0x962)
	// Device Available          : yes
	// Provides Device SDRs      : no
	// Additional Device Support :
	FirmwareRevision string `ipmitool:"Firmware Revision"`
}

// New creates a new Ipmitool with the default command
func New(ipmitoolBin string, compliance api.Compliance) (Ipmi, error) {
	_, err := exec.LookPath(ipmitoolBin)
	if err != nil {
		return nil, fmt.Errorf("ipmitool binary not present at:%s err:%w", ipmitoolBin, err)
	}
	return &Ipmitool{
		command:    ipmitoolBin,
		compliance: compliance,
	}, nil
}

// BMC returns the BMC struct
func (i *Ipmitool) BMC() (*api.BMC, error) {
	lan, err := i.GetLanConfig()
	if err != nil {
		return nil, err
	}
	fru, err := i.GetFru()
	if err != nil {
		return nil, err
	}
	info, err := i.GetBMCInfo()
	if err != nil {
		return nil, err
	}
	bmc := &api.BMC{
		IP:                  lan.IP,
		MAC:                 lan.Mac,
		BoardMfg:            fru.BoardMfg,
		BoardMfgSerial:      fru.BoardMfgSerial,
		BoardPartNumber:     fru.BoardPartNumber,
		ChassisPartNumber:   fru.ChassisPartNumber,
		ChassisPartSerial:   fru.ChassisPartSerial,
		ProductManufacturer: fru.ProductManufacturer,
		ProductPartNumber:   fru.ProductPartNumber,
		ProductSerial:       fru.ProductSerial,
		FirmwareRevision:    info.FirmwareRevision,
	}
	return bmc, nil
}

// DevicePresent returns true if the ipmi device is present, which is required to talk to the BMC.
func (i *Ipmitool) DevicePresent() bool {
	const ipmiDevicePrefix = "/dev/ipmi*"
	matches, err := filepath.Glob(ipmiDevicePrefix)
	if err != nil {
		return false
	}
	return len(matches) > 0
}

// Run execute ipmitool
func (i *Ipmitool) run(args ...string) (string, error) {
	path, err := exec.LookPath(i.command)
	if err != nil {
		return "", errors.Wrapf(err, "unable to locate program:%s in path", i.command)
	}
	cmd := exec.Command(path, args...)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("run ipmitool with args: %v output:%v error:%v", args, string(output), err)
	}
	return string(output), err
}

// GetFru returns the Field Replacable Unit information
func (i *Ipmitool) GetFru() (Fru, error) {
	config := &Fru{}
	cmdOutput, err := i.run("fru")
	if err != nil {
		return *config, errors.Wrapf(err, "unable to execute ipmitool 'fru':%v", cmdOutput)
	}
	fruMap := output2Map(cmdOutput)
	from(config, fruMap)
	return *config, nil
}

// GetBMCInfo returns the bmc info
func (i *Ipmitool) GetBMCInfo() (BMCInfo, error) {
	bmc := &BMCInfo{}
	cmdOutput, err := i.run("bmc", "info")
	if err != nil {
		return *bmc, errors.Wrapf(err, "unable to execute ipmitool 'bmc info':%v", cmdOutput)
	}
	bmcMap := output2Map(cmdOutput)
	from(bmc, bmcMap)
	return *bmc, nil
}

// GetLanConfig returns the LanConfig
func (i *Ipmitool) GetLanConfig() (LanConfig, error) {
	config := &LanConfig{}
	cmdOutput, err := i.run("lan", "print")
	if err != nil {
		return *config, errors.Wrapf(err, "unable to execute ipmitool 'lan print':%v", cmdOutput)
	}
	lanConfigMap := output2Map(cmdOutput)
	from(config, lanConfigMap)
	return *config, nil
}

// GetSession returns the Session info
func (i *Ipmitool) GetSession() (Session, error) {
	session := &Session{}
	cmdOutput, err := i.run("session", "info", "all")
	if err != nil {
		return *session, errors.Wrapf(err, "unable to execute ipmitool 'session info all':%v", cmdOutput)
	}
	sessionMap := output2Map(cmdOutput)
	from(session, sessionMap)
	return *session, nil
}

// CreateUser create a ipmi user with password and privilege level
func (i *Ipmitool) CreateUser(username, uid string, privilege Privilege) (string, error) {
	out, err := i.run("user", "set", "name", uid, username)
	if err != nil {
		return "", errors.Wrapf(err, "unable to create user %s: %v", username, out)
	}
	// This happens from time to time for unknown reason
	// retry password creation max 30 times with 1 second delay
	pw := ""
	err = retry.Do(
		func() error {
			pw = password.Generate(10)
			out, err = i.run("user", "set", "password", uid, pw)
			if err != nil {
				log.Printf("ipmi password creation failed for user:%s output:%v", username, out)
			}
			return err
		},
		retry.OnRetry(func(n uint, err error) {
			log.Printf("retry ipmi password creation for user:%s id:%s retry:%d", username, uid, n)
		}),
		retry.Delay(1*time.Second),
		retry.Attempts(30),
	)
	if err != nil {
		return pw, errors.Wrapf(err, "unable to set password for user %s: %v", username, out)
	}

	channelnumber := "1"
	out, err = i.run("channel", "setaccess", channelnumber, uid, "link=on", "ipmi=on", "callin=on", fmt.Sprintf("privilege=%d", int(privilege)))
	if err != nil {
		return pw, errors.Wrapf(err, "unable to set privilege for user %s: %v", username, out)
	}
	out, err = i.run("user", "enable", uid)
	if err != nil {
		return pw, errors.Wrapf(err, "unable to enable user %s: %v", username, out)
	}
	out, err = i.run("sol", "payload", "enable", channelnumber, uid)
	if err != nil {
		return pw, errors.Wrapf(err, "unable to enable user %s for sol access: %v", username, out)
	}

	return pw, nil
}

// SendBootOrderRaw persistently sets the boot order to given bootTarget as raw bytes.
func (i *Ipmitool) SendBootOrderRaw(bootTarget hal.BootTarget) error {
	uefiQualifier, bootDevQualifier := GetBootOrderQualifiers(bootTarget, i.compliance)
	out, err := i.run("raw", "0x00", "0x08", "0x05", uefiQualifier, bootDevQualifier, "0x00", "0x00", "0x00")
	if err != nil {
		return errors.Wrapf(err, "unable to persistently set boot order:%s out:%v", bootTarget, out)
	}
	return nil
}

// SendChassisControlRaw sends the given chassisControl as raw bytes.
func (i *Ipmitool) SendChassisControlRaw(chassisControl ipmi.ChassisControl) error {
	var rawByte string
	switch chassisControl {
	case ipmi.ControlPowerDown:
		rawByte = "0x00"
	case ipmi.ControlPowerUp:
		rawByte = "0x01"
	case ipmi.ControlPowerCycle:
		rawByte = "0x02"
	case ipmi.ControlPowerHardReset:
		rawByte = "0x03"
	default:
		return fmt.Errorf("unsupported chassis control:%d", chassisControl)
	}
	out, err := i.run("raw", "0x00", "0x02", rawByte)
	if err != nil {
		return errors.Wrapf(err, "unable to send chassis control:%v out:%v", chassisControl, out)
	}
	return nil
}

// SendChassisIdentifyRaw sends the given chassis identify as raw bytes.
func (i *Ipmitool) SendChassisIdentifyRaw(intervalOrOff, forceOn string) error {
	out, err := i.run("raw", "0x00", "0x04", intervalOrOff, forceOn)
	if err != nil {
		return errors.Wrapf(err, "unable to send chassis identify raw: intervalOrOff:%s forceOn:%s out:%v", intervalOrOff, forceOn, out)
	}
	return nil
}

func output2Map(cmdOutput string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(cmdOutput))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(strings.Join(parts[1:], ""))
		result[key] = value
	}
	for k, v := range result {
		log.Printf("output key:%s value:%s", k, v)
	}
	return result
}

// from uses reflection to fill a struct based on the tags on it.
func from(target interface{}, input map[string]string) {
	log.Printf("from target:%s input:%s", target, input)
	val := reflect.ValueOf(target).Elem()
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		ipmitoolKey := tag.Get("ipmitool")
		valueField.SetString(input[ipmitoolKey])
	}
}

const (
	persistentLegacyQualifier = "0xff"
	persistentUEFIQualifier   = "0xe0"

	pxeQualifier = "0x04"

	ipmi2HDQualifier       = "0x08"
	smcipmitoolHDQualifier = "0x24"

	biosQualifier = "0x18"
)

// GetBootOrderQualifiers returns the qualifiers needed to persistently set the the given bootOrder according to the given compliance.
func GetBootOrderQualifiers(bootTarget hal.BootTarget, compliance api.Compliance) (uefiQualifier, bootDevQualifier string) {
	/*
	   Set boot order to UEFI PXE persistently:
	   raw 0x00 0x08 0x05 0xe0 0x04 0x00 0x00 0x00  (conforms to IPMI 2.0 as well as SMCIPMITool)

	   Set boot order to UEFI HD persistently:
	   raw 0x00 0x08 0x05 0xe0 0x08 0x00 0x00 0x00  (IPMI 2.0)
	   raw 0x00 0x08 0x05 0xe0 0x24 0x00 0x00 0x00  (SMCIPMITool)

	   Set boot order to (UEFI) BIOS persistently:
	   raw 0x00 0x08 0x05 0xe0 0x18 0x00 0x00 0x00  (IPMI 2.0   , UEFI BIOS)
	   raw 0x00 0x08 0x05 0xff 0x18 0x00 0x00 0x00  (SMCIPMITool, legacy BIOS)

	   See https://github.com/metal-stack/metal/issues/73#note_151375

	   For reference: https://www.intel.com/content/dam/www/public/us/en/documents/product-briefs/ipmi-second-gen-interface-spec-v2-rev1-1.pdf (page 422)
	*/

	switch bootTarget {
	case hal.BootTargetPXE:
		uefiQualifier = persistentUEFIQualifier
		bootDevQualifier = pxeQualifier
	case hal.BootTargetDisk:
		uefiQualifier = persistentUEFIQualifier
		switch compliance {
		case api.IPMI2Compliance:
			bootDevQualifier = ipmi2HDQualifier
		case api.SMCIPMIToolCompliance:
			bootDevQualifier = smcipmitoolHDQualifier
		}
	case hal.BootTargetBIOS:
		switch compliance {
		case api.IPMI2Compliance:
			uefiQualifier = persistentUEFIQualifier
		case api.SMCIPMIToolCompliance:
			uefiQualifier = persistentLegacyQualifier
		}
		bootDevQualifier = biosQualifier
	}

	return
}

func Uint8(qualifier string) (uint8, error) {
	i, err := strconv.Atoi(qualifier)
	if err != nil {
		return 0, err
	}
	return uint8(i), nil
}
