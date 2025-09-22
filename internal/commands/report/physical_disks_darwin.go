//go:build darwin

package report

func (r *Report) getPhysicalDisksInfo(debug bool) error {
	// Get SATA disk info
	// system_profiler -json SPSerialATADataType

	return nil
}
