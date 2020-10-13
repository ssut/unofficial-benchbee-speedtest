package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type IPInfo struct {
	Query       string  `json:"query"`
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	Isp         string  `json:"isp"`
	Org         string  `json:"org"`
	As          string  `json:"as"`
}

func getIPInfo(ipAddress string) (*IPInfo, error) {
	resp, err := http.Get(fmt.Sprintf("http://ip-api.com/json/%s", ipAddress))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info := &IPInfo{}
	if err := json.Unmarshal(body, info); err != nil {
		return nil, err
	}

	return info, nil
}
