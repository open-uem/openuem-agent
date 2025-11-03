package report

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

type FreeAPIResponse struct {
	IPAddress string `json:"ipAddress"`
}

func (r *Report) getWANAddress() error {
	url := "https://free.freeipapi.com/api/json"

	response, err := http.Get(url)
	if err != nil {
		log.Printf("[ERROR]: request to freeipapi.com found an error: %v", err)
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("[ERROR]: could not read freeipapi.com response, reason: %v", err)
		return err
	}

	data := FreeAPIResponse{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("[ERROR]: could not unmarshall freeipapi.com response, reason: %v", err)
		return err
	}

	r.WAN = data.IPAddress
	return nil
}
