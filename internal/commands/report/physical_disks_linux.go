//go:build linux

package report

func (r *Report) getPhysicalDisksInfo(debug bool) error {
	// lsblk --json --nodeps --bytes -o name,serial,model,size
	return nil
}
