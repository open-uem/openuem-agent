//go:build darwin

package report

import (
	"log"
	"os/exec"
	"regexp"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getSharesInfo() error {
	r.getShares()

	return nil
}

func (r *Report) getShares() {
	listExported := `sharing -l`
	out, err := exec.Command("bash", "-c", listExported).Output()
	if err != nil {
		log.Printf("[ERROR]: could not run the sharing command, reason: %v", err)
		return
	}

	reg := regexp.MustCompile(`name:\s*(.*)`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if i%2 != 0 {
			continue
		}
		myShare := openuem_nats.Share{}
		myShare.Description = v[1]
		r.Shares = append(r.Shares, myShare)
	}

	reg = regexp.MustCompile(`path:\s*(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Shares) > i {
			r.Shares[i].Name = v[1]
			r.Shares[i].Path = v[1]
			break
		}
	}

	log.Println("[INFO]: shares information has been retrieved")
}
