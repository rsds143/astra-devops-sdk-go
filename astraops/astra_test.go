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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os/user"
	"path"
	"testing"
)

type ClientInfo struct {
	ClientName   string `json:"clientName"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

func getClientInfo() ClientInfo {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	saFile := path.Join(u.HomeDir, ".config", "astra", "sa.json")
	b, err := ioutil.ReadFile(saFile)
	if err != nil {
		log.Fatal(err)
	}
	var clientInfo ClientInfo
	if err = json.Unmarshal(b, &clientInfo); err != nil {
		log.Fatalf("unable to convert %s to json object with error %v", saFile, err)
	}
	return clientInfo
}

func TestListDb(t *testing.T) {
	c := getClientInfo()
	client, err := Authenticate(c.ClientName, c.ClientID, c.ClientSecret)
	if err != nil {
		t.Fatalf("failed authentication %v", err)
	}
	createDb := CreateDb{
		Name:          "mydb",
		Keyspace:      "mykeyspace",
		Region:        "europe-west1",
		CloudProvider: "GCP",
		CapacityUnits: 1,
		Tier:          "free",
		User:          fmt.Sprintf("a%v", rand.Int63()),
		Password:      fmt.Sprintf("b%v", rand.Int63()),
	}
	id, _, err := client.CreateDb(createDb)
	if err != nil {
		t.Fatalf("failed creating db %v", err)
	}
	t.Logf("id is '%s'", id)
	defer func() {
		if id != "" {
			if err := client.Terminate(id, false); err != nil {
				t.Logf("warning error deleting created db %s up %s", createDb.Name, err)
			}
		}
	}()
	dbs, err := client.ListDb("", "", "", 10)
	if err != nil {
		t.Fatalf("failed retrieving db %v", err)
	}
	found := false
	for _, db := range dbs {
		log.Printf("id: '%v'", db.ID)
		if db.ID == id {
			log.Print("found newly created db")
			found = true
			break
		}
	}
	if !found {
		t.Errorf("did not find newly created db in %v", dbs)
	}
}

func TestParkDb(t *testing.T) {
	c := getClientInfo()
	client, err := Authenticate(c.ClientName, c.ClientID, c.ClientSecret)
	if err != nil {
		t.Fatalf("failed authentication %v", err)
	}
	createDb := CreateDb{
		Name:          "mydb",
		Keyspace:      "mykeyspace",
		Region:        "europe-west1",
		CloudProvider: "GCP",
		CapacityUnits: 1,
		Tier:          "free",
		User:          fmt.Sprintf("a%v", rand.Int63()),
		Password:      fmt.Sprintf("b%v", rand.Int63()),
	}
	id, _, err := client.CreateDb(createDb)
	if err != nil {
		t.Fatalf("failed creating db %v", err)
	}
	t.Logf("id is '%s'", id)
	defer func() {
		if id != "" {
			if err := client.Terminate(id, false); err != nil {
				t.Logf("warning error deleting created db %s up %s", createDb.Name, err)
			}
		}
	}()
	err = client.Park(id)
	if err != nil {
		t.Fatalf("park failed with error %v", err)
	}
	db, err := client.FindDb(id)
	if err != nil {
		t.Fatalf("unable to find parked db with error %v", err)
	}
	if db.Status != "PARKED" {
		t.Fatalf("expected db to be parked but was %v", db.Status)
	}
}
