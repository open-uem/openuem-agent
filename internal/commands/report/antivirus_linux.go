//go:build linux

package report

func (r *Report) getAntivirusInfo(debug bool) error {
	r.Antivirus.Name = ""
	r.Antivirus.IsActive = false
	r.Antivirus.IsUpdated = false
	return nil
}
