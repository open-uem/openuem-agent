//go:build darwin

package report

func (r *Report) getAntivirusInfo() error {
	r.Antivirus.Name = "Unknown"
	r.Antivirus.IsActive = false
	r.Antivirus.IsUpdated = false
	return nil
}
