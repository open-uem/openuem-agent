//go:build darwin

package report

func (r *Report) hasRustDesk(debug bool) {
	r.HasRustDesk = false
}

func (r *Report) hasRustDeskService(debug bool) {
	r.HasRustDeskService = false
}

func (r *Report) isFlatpakRustDesk() bool {
	return false
}
