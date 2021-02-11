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
	"os"
	"testing"

	"github.com/prometheus/common/log"
)

var ClientId = os.Getenv("ASTRA_TEST_CLIENT_ID")
var ClientSecret = os.Getenv("ASTRA_TEST_CLIENT_SECRET")
var ClientName = os.Getenv("ASTRA_TEST_CLIENT_NAME")

func TestListDb(t *testing.T) {
	client, err := Authenticate(ClientName, ClientId, ClientSecret)
	if err != nil {
		t.Fatalf("failed authentication %v", err)
	}
	dbs, err := client.ListDb("", "", "", 10)
	if err != nil {
		t.Fatalf("failed retrieving db %v", err)
	}
	for _, db := range dbs {
		log.Debug(db)
	}
}
