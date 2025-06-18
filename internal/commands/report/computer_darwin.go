//go:build darwin

package report

import (
	"encoding/json"
	"errors"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func (r *Report) getComputerInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: computer system info has been requested")
	}
	if err := r.getComputerSystemInfo1(); err != nil {
		if err := r.getComputerSystemInfo2(); err != nil {
			log.Printf("[ERROR]: could not get information from System Profiler: %v", err)
			return err
		}
	}

	return nil
}

func (r *Report) getComputerSystemInfo1() error {
	var data SPHardwareDataType1
	out, err := exec.Command("system_profiler", "-json", "SPHardwareDataType").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &data); err != nil {
		return err
	}

	r.Computer.Manufacturer = "Apple"

	if len(data.SPHardwareDataType) == 0 {
		return errors.New("could not get info from SPHardwareDataType")
	}

	hw := data.SPHardwareDataType[0]

	r.Computer.Model = hw.MachineModel
	if r.Computer.Model == "iMacPro1,1" {
		r.Computer.Model = "MacBookPro15,1"
	}
	r.Computer.Memory = getMacOSMemory(hw.PhysicalMemory)
	r.Computer.Processor = hw.CPUType
	r.Computer.ProcessorArch = getMacOSArch()
	r.Computer.ProcessorCores = int64(hw.NumProcessors)

	r.Computer.Serial = hw.SerialNumber
	return nil
}

func (r *Report) getComputerSystemInfo2() error {
	var data SPHardwareDataType2
	out, err := exec.Command("system_profiler", "-json", "SPHardwareDataType").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &data); err != nil {
		return err
	}

	r.Computer.Manufacturer = "Apple"

	if len(data.SPHardwareDataType) == 0 {
		return errors.New("could not get info from SPHardwareDataType")
	}

	hw := data.SPHardwareDataType[0]

	r.Computer.Model = hw.MachineModel
	if r.Computer.Model == "iMacPro1,1" {
		r.Computer.Model = "MacBookPro15,1"
	}
	r.Computer.Memory = getMacOSMemory(hw.PhysicalMemory)
	r.Computer.Processor = hw.CPUType
	if hw.CPUType == "" {
		r.Computer.Processor = hw.ChipType
	}
	r.Computer.ProcessorArch = getMacOSArch()
	numProcessors, err := strconv.Atoi(hw.NumProcessors)
	if err != nil {
		r.Computer.ProcessorCores = int64(numProcessors)
	}

	r.Computer.Serial = hw.SerialNumber
	return nil
}

func getMacOSMemory(memory string) uint64 {
	if strings.Contains(memory, "GB") {
		quantity := strings.TrimSuffix(memory, " GB")
		val, err := strconv.Atoi(quantity)
		if err == nil {
			return uint64(val * 1024)
		}
	}

	if strings.Contains(memory, "MB") {
		quantity := strings.TrimSuffix(memory, " MB")
		val, err := strconv.Atoi(quantity)
		if err == nil {
			return uint64(val)
		}
	}

	return 0
}

type SPHardwareDataType1 struct {
	SPHardwareDataType []HardwareDataType1 `json:"SPHardwareDataType"`
}

type SPHardwareDataType2 struct {
	SPHardwareDataType []HardwareDataType2 `json:"SPHardwareDataType"`
}

type HardwareDataType1 struct {
	Name                  string `json:"_name"`
	BootROMVersion        string `json:"boot_rom_version"`
	CPUType               string `json:"cpu_type"`
	ChipType              string `json:"chip_type"`
	CurrentProcessorSpeed string `json:"current_processor_speed"`
	L2CacheCore           string `json:"l2_cache_core"`
	L3Cache               string `json:"l3_cache"`
	MachineModel          string `json:"machine_model"`
	MachineName           string `json:"machine_name"`
	NumProcessors         int    `json:"number_processors"`
	OSLoaderVersion       string `json:"os_loader_version"`
	Packages              int    `json:"packages"`
	PhysicalMemory        string `json:"physical_memory"`
	PlatformCPUHTT        string `json:"platform_cpu_http"`
	PlatformUUID          string `json:"platform_UUID"`
	ProvisioningUDID      string `json:"provisioning_UDID"`
	SerialNumber          string `json:"serial_number"`
}
type HardwareDataType2 struct {
	Name                  string `json:"_name"`
	BootROMVersion        string `json:"boot_rom_version"`
	CPUType               string `json:"cpu_type"`
	ChipType              string `json:"chip_type"`
	CurrentProcessorSpeed string `json:"current_processor_speed"`
	L2CacheCore           string `json:"l2_cache_core"`
	L3Cache               string `json:"l3_cache"`
	MachineModel          string `json:"machine_model"`
	MachineName           string `json:"machine_name"`
	NumProcessors         string `json:"number_processors"`
	OSLoaderVersion       string `json:"os_loader_version"`
	Packages              int    `json:"packages"`
	PhysicalMemory        string `json:"physical_memory"`
	PlatformCPUHTT        string `json:"platform_cpu_http"`
	PlatformUUID          string `json:"platform_UUID"`
	ProvisioningUDID      string `json:"provisioning_UDID"`
	SerialNumber          string `json:"serial_number"`
}
