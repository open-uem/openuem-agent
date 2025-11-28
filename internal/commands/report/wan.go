package report

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type IPConfigAPIResponse struct {
	IPAddress string `json:"ip"`
}

func (r *Report) getWANAddress() error {
	url := "https://ipconfig.io/json"

	response, err := http.Get(url)
	if err != nil {
		log.Printf("[ERROR]: request to ipconfig.io found an error: %v", err)
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("[ERROR]: could not read ipconfig.io response, reason: %v", err)
		return err
	}

	data := IPConfigAPIResponse{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("[ERROR]: could not unmarshall ipconfig.io response, reason: %v", err)
		return err
	}

	r.WAN = data.IPAddress
	return nil
}
