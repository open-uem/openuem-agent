package deploy

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/open-uem/nats"
)

type brewOutdatedPackage struct {
	Name              string       `json:"name"`
	InstalledVersions brewVersions `json:"installed_versions"`
	CurrentVersion    string       `json:"current_version"`
	Pinned            bool         `json:"pinned"`
}

// brewVersions accepts both a list of versions and a single version, as
// older Homebrew releases reported a string for casks
type brewVersions []string

func (v *brewVersions) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*v = list
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*v = []string{single}
	return nil
}

// ParseBrewOutdated parses the output of `brew outdated --json=v2`. Pinned
// formulae are skipped as Homebrew refuses to upgrade them.
func ParseBrewOutdated(output []byte) ([]nats.AvailableUpdate, error) {
	outdated := struct {
		Formulae []brewOutdatedPackage `json:"formulae"`
		Casks    []brewOutdatedPackage `json:"casks"`
	}{}

	// Homebrew may print warnings around the JSON document
	start, end := bytes.IndexByte(output, '{'), bytes.LastIndexByte(output, '}')
	if start == -1 || end < start {
		return nil, errors.New("brew outdated didn't return a JSON document")
	}

	if err := json.Unmarshal(output[start:end+1], &outdated); err != nil {
		return nil, err
	}

	updates := []nats.AvailableUpdate{}
	add := func(packages []brewOutdatedPackage, brewType string) {
		for _, p := range packages {
			if p.Pinned || !IsValidPackageID(p.Name) {
				continue
			}
			updates = append(updates, nats.AvailableUpdate{
				PackageId:        p.Name,
				PackageName:      p.Name,
				PackageBrewType:  brewType,
				InstalledVersion: strings.Join(p.InstalledVersions, ", "),
				AvailableVersion: p.CurrentVersion,
				Source:           "brew",
			})
		}
	}
	add(outdated.Formulae, "formula")
	add(outdated.Casks, "cask")

	return updates, nil
}
