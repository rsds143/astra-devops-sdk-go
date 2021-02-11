/**
Copyright 2021 Ryan Svihla

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package astraops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/common/log"
)

// Authenticate returns a token from the service account
func Authenticate(clientName, clientId, clientSecret string) (*AuthenticatedClient, error) {
	url := "https://api.astra.datastax.com/v2/authenticateServiceAccount"
	c := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxConnsPerHost:     10,
			MaxIdleConnsPerHost: 10,
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	payload := map[string]interface{}{
		"clientName":   clientName,
		"clientId":     clientId,
		"clientSecret": clientSecret,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("unable to marshal JSON object with: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("failed creating request with: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("failed listing databases with: %w", err)
	}
	var tokenResponse map[string]interface{}
	if res.StatusCode != 200 {
		err = json.NewDecoder(res.Body).Decode(&tokenResponse)
		if err != nil {
			return &AuthenticatedClient{}, fmt.Errorf("unable to decode error response with error: %w", err)
		}
		return &AuthenticatedClient{}, fmt.Errorf("expected status code 200 but had: %v error was %v", res.StatusCode, tokenResponse["errors"])
	}
	err = json.NewDecoder(res.Body).Decode(&tokenResponse)
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("unable to decode response with error: %w", err)
	}
	if token, ok := tokenResponse["token"]; !ok {
		return &AuthenticatedClient{}, fmt.Errorf("unable to find token in json: %s", payload)
	} else {
		log.Infof("response is %v", tokenResponse)
		log.Infof("token is %s", token)
		return &AuthenticatedClient{
			client: c,
			token:  fmt.Sprintf("Bearer %s", token),
		}, nil
	}
}

// AuthenticatedClient has a token and the methods to query the Astra DevOps API
type AuthenticatedClient struct {
	token  string
	client *http.Client
}

func (a *AuthenticatedClient) ListDb(include string, provider string, startingAfter string, limit int32) ([]DataBase, error) {
	var dbs []DataBase
	url := "https://api.astra.datastax.com/v2/databases"
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return dbs, fmt.Errorf("failed creating request with: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", a.token)
	q := req.URL.Query()
	if len(include) > 0 {
		q.Add("include", include)
	}
	if len(provider) > 0 {
		q.Add("provider", provider)
	}
	if len(startingAfter) > 0 {
		q.Add("starting_after", startingAfter)
	}
	if limit > 0 {
		q.Add("limit", strconv.FormatInt(int64(limit), 10))
	}
	req.URL.RawQuery = q.Encode()
	res, err := a.client.Do(req)
	if err != nil {
		return dbs, fmt.Errorf("failed listing databases with: %w", err)
	}
	if res.StatusCode != 200 {
		var resObj map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&resObj)
		if err != nil {
			return []DataBase{}, fmt.Errorf("unable to decode error response with error: %w", err)
		}
		return []DataBase{}, fmt.Errorf("expected status code 200 but had: %v error was %v", res.StatusCode, resObj["errors"])
	}
	err = json.NewDecoder(res.Body).Decode(&dbs)
	if err != nil {
		return []DataBase{}, fmt.Errorf("unable to decode response with error: %w", err)
	}
	return dbs, nil
}

// Info is some database meta data info
type Info struct {
	Name                string         `json:"name"`
	Keyspace            string         `json:"keyspace"`
	CloudProvider       string         `json:"cloudProvider"`
	Tier                string         `json:"tier"`
	CapacityUnits       int            `json:"capacityUnits"`
	Region              string         `json:"region"`
	User                string         `json:"user"`
	Password            string         `json:"password"`
	AdditionalKeyspaces []string       `json:"additionalKeyspaces"`
	Cost                map[string]int `json:"cost"`
}

// Storage is the storage information for the cluster
type Storage struct {
	NodeCount         int `json:"nodeCount"`
	ReplicationFactor int `json:"replicationFactor"`
	TotalStorage      int `json:"totalStorage"`
	UsedStorage       int `json:"usedStorage"`
}

// DataBase is the returned data from the Astra DevOps API
type DataBase struct {
	Id               string   `json:"id"`
	OrgId            string   `json:"orgId"`
	OwnerId          string   `json:"ownerId"`
	Info             Info     `json:"info"`
	CreationTime     string   `json:"creationTime"`
	TerminationTime  string   `json:"terminationTime"`
	Status           string   `json:"status"`
	Storage          Storage  `json:"storage"`
	AvailableActions []string `json:"availableActions"`
	Message          string   `json:"message"`
	StudioUrl        string   `json:"studioUrl"`
	GrafanaUrl       string   `json:"grafanaUrl"`
	CqlshUrl         string   `json:"cqlshUrl"`
	GraphqlUrl       string   `json:"graphUrl"`
	DataEndpointUrl  string   `json:"dataEndpointUrl"`
}
