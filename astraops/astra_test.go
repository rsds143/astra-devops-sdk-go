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
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os/user"
	"path"
	"strings"
	"testing"
)

func TestTokenLogin(t *testing.T) {
	t.Parallel()
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	tokenFile := path.Join(u.HomeDir, ".config", "astra", "token")
	b, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		log.Fatal(err)
	}

	client := AuthenticateToken(strings.Trim(string(b), "\n"), true)
	_, err = client.ListDb("", "", "", 10)
	if err != nil {
		t.Fatalf("failed authentication '%v'", err)
	}
}

func TestListDb(t *testing.T) {
	t.Parallel()
	client, id := generateDB(t, "testerdblist", "serverless")
	defer func() {
		terminateDB(t, client, id)
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
	t.Parallel()
	client, id := generateDB(t, "testingdbparkworks", "free")
	defer func() {
		terminateDB(t, client, id)
	}()
	err := client.Park(id)
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

func TestGetConnectionBundle(t *testing.T) {
	t.Parallel()
	client, id := generateDB(t, "testgetconnection", "serverless")
	defer func() {
		terminateDB(t, client, id)
	}()
	secureBundle, err := client.GetSecureBundle(id)
	if err != nil {
		t.Fatalf("failed getting secured bundle %v", err)
	}
	if secureBundle.DownloadURL == "" {
		t.Errorf("no download url for bundle")
	}

	if secureBundle.DownloadURLInternal == "" {
		t.Errorf("no internal download url for bundle")
	}

	if secureBundle.DownloadURLMigrationProxy == "" {
		t.Errorf("no migration proxy url for bundle")
	}
}

func TestTerminateDB(t *testing.T) {
	t.Parallel()
	client, id := generateDB(t, "testterminate", "serverless")
	// yes this will create a log that it cannot delete the already terminated db this is fine
	defer func() {
		terminateDB(t, client, id)
	}()
	err := client.Terminate(id, false)
	if err != nil {
		t.Fatalf("failed to delete %v", err)
	}
	dbs, err := client.ListDb("", "", "", 10)
	if err != nil {
		t.Fatalf("failed retrieving db %v", err)
	}
	for _, db := range dbs {
		log.Printf("id: '%v'", db.ID)
		if db.ID == id {
			log.Print("found newly deleted db")
			if db.Status == TERMINATING || db.Status == TERMINATED {
				log.Printf("database %v successfully deleted", db.ID)
				break
			}
			t.Fatalf("expected database to terminated but it was %v", db.Status)
		}
	}
}

func generateString() (string, error) {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateDB(t *testing.T, name string, tier string) (*AuthenticatedClient, string) {
	c := getClientInfo()
	client, err := Authenticate(c, true)
	if err != nil {
		t.Fatalf("failed authentication %v", err)
	}
	pass, err := generateString()
	if err != nil {
		t.Fatalf("failed random gen %v", err)
	}
	createDb := CreateDb{
		Name:          name,
		Keyspace:      "mykeyspace",
		Region:        "europe-west1",
		CloudProvider: "GCP",
		CapacityUnits: 1,
		Tier:          tier,
		User:          "myuser",
		Password:      pass,
	}
	db, err := client.CreateDb(createDb)
	if err != nil {
		t.Fatalf("failed creating db %v", err)
	}
	id := db.ID
	t.Logf("id is '%s'", id)
	return client, id
}

func terminateDB(t *testing.T, client *AuthenticatedClient, id string) {
	if id == "" {
		t.Logf("no database to delete in test %v", t.Name())
		return
	}
	if err := client.TerminateAsync(id, false); err != nil {
		t.Logf("warning error deleting created db %s due to %s in test %v", id, err, t.Name())
		return
	}
	t.Logf("database %v deleted for test %v", id, t.Name())
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

func TestFormatErrors(t *testing.T) {
	str := FormatErrors([]Error{
		{ID: 1, Message: "hello error"},
		{ID: 2, Message: "goodbye error"},
	})
	expected := "ID: 1 Text: 'hello error', ID: 2 Text: 'goodbye error'"
	if str != expected {
		t.Errorf("expected '%v' but was '%v'", expected, str)
	}
}

func TestReadErrorFromResponse(t *testing.T) {
	stringReader := strings.NewReader(`
		{ "errors": [
			{"id":1, "message": "bad error"},
			{"id":2, "message": "worse error"}
		]
	}
	`)
	body := ioutil.NopCloser(stringReader)
	err := readErrorFromResponse(&http.Response{
		StatusCode: 400,
		Body:       body,
	}, 201, 202)
	expected := "expected status codes 201, 202 but had: 400 error with errors - ID: 1 Text: 'bad error', ID: 2 Text: 'worse error'"
	if err.Error() != expected {
		t.Errorf("expected '%v' but was '%v'", expected, err.Error())
	}
}

func TestReadErrorFromResponseBadJSON(t *testing.T) {
	stringReader := strings.NewReader(`wrongerror`)
	body := ioutil.NopCloser(stringReader)
	err := readErrorFromResponse(&http.Response{
		StatusCode: 400,
		Body:       body,
	}, 201, 202)
	expected := "unable to decode error response with error: 'invalid character 'w' looking for beginning of value'. status code was 400"
	if err.Error() != expected {
		t.Errorf("expected '%v' but was '%v'", expected, err.Error())
	}
}
