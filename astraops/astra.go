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

// Package astraops provides access to the Astra DevOps api
package astraops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ClientInfo is a handy type for consumers but not used internally in the library other than for testing
type ClientInfo struct {
	ClientName   string `json:"clientName"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// StatusEnum has all the available statuses for a database
type StatusEnum string

// List of StatusEnum
const (
	ACTIVE       StatusEnum = "ACTIVE"
	PENDING      StatusEnum = "PENDING"
	PREPARING    StatusEnum = "PREPARING"
	PREPARED     StatusEnum = "PREPARED"
	INITIALIZING StatusEnum = "INITIALIZING"
	PARKED       StatusEnum = "PARKED"
	PARKING      StatusEnum = "PARKING"
	UNPARKING    StatusEnum = "UNPARKING"
	TERMINATED   StatusEnum = "TERMINATED"
	TERMINATING  StatusEnum = "TERMINATING"
	RESIZING     StatusEnum = "RESIZING"
	ERROR        StatusEnum = "ERROR"
	MAINTENANCE  StatusEnum = "MAINTENANCE"
	UNKNOWN      StatusEnum = "UNKNOWN"
)

func newHTTPClient() *http.Client {
	return &http.Client{
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
}

// AuthenticateToken returns a client
// * @param token string - token generated for login in the astra UI
// * @param verbose bool - if true the logging is much more verbose
// @returns (*AuthenticatedClient , error)
func AuthenticateToken(token string, verbose bool) *AuthenticatedClient {
	return &AuthenticatedClient{
		client:  newHTTPClient(),
		token:   fmt.Sprintf("Bearer %s", token),
		verbose: verbose,
	}
}

// Authenticate returns a client using legacy Service Account. This is not deprecated but one should move to AuthenticateToken
// * @param clientInfo - classic service account from legacy Astra
// * @param verbose bool - if true the logging is much more verbose
// @returns (*AuthenticatedClient , error)
func Authenticate(clientInfo ClientInfo, verbose bool) (*AuthenticatedClient, error) {
	url := "https://api.astra.datastax.com/v2/authenticateServiceAccount"
	body, err := json.Marshal(clientInfo)
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("unable to marshal JSON object with: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("failed creating request with: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c := newHTTPClient()
	res, err := c.Do(req)
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("failed listing databases with: %w", err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return &AuthenticatedClient{}, readErrorFromResponse(res, 200)
	}
	var tokenResponse TokenResponse
	err = json.NewDecoder(res.Body).Decode(&tokenResponse)
	if err != nil {
		return &AuthenticatedClient{}, fmt.Errorf("unable to decode response with error: %w", err)
	}
	if tokenResponse.Token == "" {
		return &AuthenticatedClient{}, errors.New("empty token in token response")
	}
	return &AuthenticatedClient{
		client:  c,
		token:   fmt.Sprintf("Bearer %s", tokenResponse.Token),
		verbose: verbose,
	}, nil
}

// AuthenticatedClient has a token and the methods to query the Astra DevOps API
type AuthenticatedClient struct {
	token   string
	client  *http.Client
	verbose bool
}

const serviceURL = "https://api.astra.datastax.com/v2/databases"

func (a *AuthenticatedClient) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", a.token)
	req.Header.Set("Content-Type", "application/json")
}

// WaitUntil will keep checking the database for the requested status until it is available. Eventually it will timeout if the operation is not
// yet complete.
// * @param id string - the database id to find
// * @param tries int - number of attempts
// * @param intervalSeconds int - seconds to wait between tries
// * @param status StatusEnum - status to wait for
// @returns (Database, error)
func (a *AuthenticatedClient) WaitUntil(id string, tries int, intervalSeconds int, status StatusEnum) (Database, error) {
	for i := 0; i < tries; i++ {
		time.Sleep(time.Duration(intervalSeconds) * time.Second)
		db, err := a.FindDb(id)
		if err != nil {
			if a.verbose {
				log.Printf("db %s not able to be found with error '%v' trying again %v more times", id, err, tries-i-1)
			} else {
				log.Printf("waiting")
			}
			continue
		}
		if db.Status == status {
			return db, nil
		}
		if a.verbose {
			log.Printf("db %s in state %v but expected %v trying again %v more times", id, db.Status, status, tries-i-1)
		} else {
			log.Printf("waiting")
		}
	}
	return Database{}, fmt.Errorf("unable to find db id %s with status %s after %v seconds", id, status, intervalSeconds*tries)
}

// ListDb find all databases that match the parameters
// * @param "include" (optional.string) -  Allows filtering so that databases in listed states are returned
// * @param "provider" (optional.string) -  Allows filtering so that databases from a given provider are returned
// * @param "startingAfter" (optional.string) -  Optional parameter for pagination purposes. Used as this value for starting retrieving a specific page of results
// * @param "limit" (optional.int32) -  Optional parameter for pagination purposes. Specify the number of items for one page of data
// @return ([]Database, error)
func (a *AuthenticatedClient) ListDb(include string, provider string, startingAfter string, limit int32) ([]Database, error) {
	var dbs []Database
	req, err := http.NewRequest("GET", serviceURL, http.NoBody)
	if err != nil {
		return dbs, fmt.Errorf("failed creating request with: %v", err)
	}
	a.setHeaders(req)
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
		return dbs, fmt.Errorf("failed listing databases with: %v", err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return dbs, readErrorFromResponse(res, 200)
	}
	err = json.NewDecoder(res.Body).Decode(&dbs)
	if err != nil {
		return []Database{}, fmt.Errorf("unable to decode response with error: %v", err)
	}
	return dbs, nil
}

// CreateDb creates a database in Astra, username and password fields are required only on legacy tiers and waits until it is in a created state
// * @param createDb Definition of new database
// @return (Database, error)
func (a *AuthenticatedClient) CreateDb(createDb CreateDb) (Database, error) {
	id, err := a.CreateDbAsync(createDb)
	if err != nil {
		return Database{}, err
	}
	db, err := a.WaitUntil(id, 30, 30, ACTIVE)
	if err != nil {
		return db, fmt.Errorf("create db failed because '%v'", err)
	}
	return db, nil
}

// CreateDbAsync creates a database in Astra, username and password fields are required only on legacy tiers and returns immediately as soon as the request succeeds
// * @param createDb Definition of new database
// @return (Database, error)
func (a *AuthenticatedClient) CreateDbAsync(createDb CreateDb) (string, error) {
	body, err := json.Marshal(&createDb)
	if err != nil {
		return "", fmt.Errorf("unable to marshall create db json with: %w", err)
	}
	req, err := http.NewRequest("POST", serviceURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed creating request with: %w", err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed creating database with: %w", err)
	}
	defer closeBody(res)
	if res.StatusCode != 201 {
		return "", readErrorFromResponse(res, 201)
	}
	return res.Header.Get("location"), nil
}

func readErrorFromResponse(res *http.Response, expectedCodes ...int) error {
	var resObj ErrorResponse
	err := json.NewDecoder(res.Body).Decode(&resObj)
	if err != nil {
		return fmt.Errorf("unable to decode error response with error: '%v'. status code was %v", err, res.StatusCode)
	}
	var statusSuffix string
	if len(expectedCodes) > 0 {
		statusSuffix = "s"
	}
	var errorSuffix string
	if len(resObj.Errors) > 0 {
		errorSuffix = "s"
	}
	var codeString []string
	for _, c := range expectedCodes {
		codeString = append(codeString, fmt.Sprintf("%v", c))
	}
	formattedCodes := strings.Join(codeString, ", ")
	return fmt.Errorf("expected status code%v %v but had: %v error with error%v - %v", statusSuffix, formattedCodes, res.StatusCode, errorSuffix, FormatErrors(resObj.Errors))
}

// FindDb Returns specified database
// * @param databaseID string representation of the database ID
// @return (Database, error)
func (a *AuthenticatedClient) FindDb(databaseID string) (Database, error) {
	var dbs Database
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", serviceURL, databaseID), http.NoBody)
	if err != nil {
		return dbs, fmt.Errorf("failed creating request to find db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return dbs, fmt.Errorf("failed get database id %s with: %w", databaseID, err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return dbs, readErrorFromResponse(res, 200)
	}
	err = json.NewDecoder(res.Body).Decode(&dbs)
	if err != nil {
		return Database{}, fmt.Errorf("unable to decode response with error: %w", err)
	}
	return dbs, nil
}

// AddKeyspaceToDb Adds keyspace into database
// * @param databaseID string representation of the database ID
// * @param keyspaceName Name of database keyspace
// @return error
func (a *AuthenticatedClient) AddKeyspaceToDb(databaseID string, keyspaceName string) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/keyspaces/%s", serviceURL, databaseID, keyspaceName), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed creating request to add keyspace to db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add keyspace to db id %s with: %w", databaseID, err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return readErrorFromResponse(res, 200)
	}
	return nil
}

// GetSecureBundle Returns a temporary URL to download a zip file with certificates for connecting to the database.
// The URL expires after five minutes.&lt;p&gt;There are two types of the secure bundle URL: &lt;ul&gt
// * @param databaseID string representation of the database ID
// @return (SecureBundle, error)
func (a *AuthenticatedClient) GetSecureBundle(databaseID string) (SecureBundle, error) {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/secureBundleURL", serviceURL, databaseID), http.NoBody)
	if err != nil {
		return SecureBundle{}, fmt.Errorf("failed creating request to get secure bundle for db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return SecureBundle{}, fmt.Errorf("failed get secure bundle for database id %s with: %w", databaseID, err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return SecureBundle{}, readErrorFromResponse(res, 200)
	}
	var sb SecureBundle
	err = json.NewDecoder(res.Body).Decode(&sb)
	if err != nil {
		return SecureBundle{}, fmt.Errorf("unable to decode response with error: %w", err)
	}
	return sb, nil
}

// TerminateAsync deletes the database at the specified id, preparedStateOnly can be left to false in almost all cases
// * @param databaseID string representation of the database ID
// * @param "PreparedStateOnly" -  For internal use only.  Used to safely terminate prepared databases
// @return error
func (a *AuthenticatedClient) TerminateAsync(id string, preparedStateOnly bool) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/terminate", serviceURL, id), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed creating request to terminate db with id %s with: %w", id, err)
	}
	a.setHeaders(req)
	q := req.URL.Query()
	q.Add("preparedStateOnly", strconv.FormatBool(preparedStateOnly))
	req.URL.RawQuery = q.Encode()
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to terminate database id %s with: %w", id, err)
	}
	defer closeBody(res)
	if res.StatusCode != 202 {
		return readErrorFromResponse(res, 202)
	}
	return nil
}

// Terminate deletes the database at the specified id and will block until it shows up as deleted or is removed from the system
// * @param databaseID string representation of the database ID
// * @param "PreparedStateOnly" -  For internal use only.  Used to safely terminate prepared databases
// @return error
func (a *AuthenticatedClient) Terminate(id string, preparedStateOnly bool) error {
	err := a.TerminateAsync(id, preparedStateOnly)
	if err != nil {
		return err
	}
	tries := 30
	intervalSeconds := 10
	var lastResponse string
	var lastStatusCode int
	for i := 0; i < tries; i++ {
		time.Sleep(time.Duration(intervalSeconds) * time.Second)
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", serviceURL, id), http.NoBody)
		if err != nil {
			return fmt.Errorf("failed creating request to find db with id %s with: %w", id, err)
		}
		a.setHeaders(req)
		res, err := a.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed get database id %s with: %w", id, err)
		}
		defer closeBody(res)
		lastStatusCode = res.StatusCode
		if res.StatusCode == 401 {
			return nil
		}
		if res.StatusCode == 200 {
			var db Database
			err = json.NewDecoder(res.Body).Decode(&db)
			if err != nil {
				return fmt.Errorf("critical error trying to get status of database not deleted, unable to decode response with error: %v", err)
			}
			if db.Status == TERMINATED || db.Status == TERMINATING {
				if a.verbose {
					log.Printf("delete status is %v for db %v and is therefore successful, we are going to exit now", db.Status, id)
				}
				return nil
			}
			if a.verbose {
				log.Printf("db %s not deleted yet expected status code 401 or a 200 with a db Status of %v or %v but was 200 with a db status of %v. trying again", id, TERMINATED, TERMINATING, db.Status)
			} else {
				log.Printf("waiting")
			}
			continue
		}
		lastResponse = fmt.Sprintf("%v", readErrorFromResponse(res, 200, 401))
		if a.verbose {
			log.Printf("db %s not deleted yet expected status code 401 or a 200 with a db Status of %v or %v but was: %v and error was '%v'. trying again", id, TERMINATED, TERMINATING, res.StatusCode, lastResponse)
		} else {
			log.Printf("waiting")
		}
	}
	return fmt.Errorf("delete of db %s not complete. Last response from finding db was '%v' and last status code was %v", id, lastResponse, lastStatusCode)
}

// ParkAsync parks the database at the specified id. Note you cannot park a serverless database
// * @param databaseID string representation of the database ID
// @return error
func (a *AuthenticatedClient) ParkAsync(databaseID string) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/park", serviceURL, databaseID), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed creating request to park db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to park database id %s with: %w", databaseID, err)
	}
	defer closeBody(res)
	if res.StatusCode != 202 {
		return readErrorFromResponse(res, 202)
	}
	return nil
}

// Park parks the database at the specified id and will block until the database is parked
// * @param databaseID string representation of the database ID
// @return error
func (a *AuthenticatedClient) Park(databaseID string) error {
	err := a.ParkAsync(databaseID)
	if err != nil {
		return fmt.Errorf("park db failed because '%v'", err)
	}
	_, err = a.WaitUntil(databaseID, 30, 30, PARKED)
	if err != nil {
		return fmt.Errorf("unable to check status for park db because of error '%v'", err)
	}
	return nil
}

// UnparkAsync unparks the database at the specified id. NOTE you cannot unpark a serverless database
// * @param databaseID String representation of the database ID
// @return error
func (a *AuthenticatedClient) UnparkAsync(databaseID string) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/unpark", serviceURL, databaseID), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed creating request to unpark db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to unpark database id %s with: %w", databaseID, err)
	}
	defer closeBody(res)
	if res.StatusCode != 202 {
		return readErrorFromResponse(res, 202)
	}
	return nil
}

// Unpark unparks the database at the specified id and will block until the database is unparked
// * @param databaseID String representation of the database ID
// @return error
func (a *AuthenticatedClient) Unpark(databaseID string) error {
	err := a.UnparkAsync(databaseID)
	if err != nil {
		return fmt.Errorf("unpark db failed because '%v'", err)
	}
	_, err = a.WaitUntil(databaseID, 60, 30, ACTIVE)
	if err != nil {
		return fmt.Errorf("unable to check status for unpark db because of error '%v'", err)
	}
	return nil
}

// Resize a database. Total number of capacity units desired should be specified. Reducing a size of a database is not supported at this time. Note you cannot resize a serverless database
// * @param databaseID string representation of the database ID
// * @param capacityUnits int32 containing capacityUnits key with a value greater than the current number of capacity units (max increment of 3 additional capacity units)
// @return error
func (a *AuthenticatedClient) Resize(databaseID string, capacityUnits int32) error {
	body := fmt.Sprintf("{\"capacityUnits\":%d}", capacityUnits)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/resize", serviceURL, databaseID), bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("failed creating request to unpark db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to unpark database id %s with: %w", databaseID, err)
	}
	defer res.Body.Close()
	if res.StatusCode > 299 {
		var resObj ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&resObj)
		if err != nil {
			return fmt.Errorf("unable to decode error response with error: %w", err)
		}
		return fmt.Errorf("expected status code 2xx but had: %v with error(s) - %v", res.StatusCode, FormatErrors(resObj.Errors))
	}
	return nil
}

// ResetPassword changes the password for the database at the specified id
// * @param databaseID string representation of the database ID
// * @param username string containing username
// * @param password string containing password. The specified password will be updated for the specified database user
// @return error
func (a *AuthenticatedClient) ResetPassword(databaseID, username, password string) error {
	body := fmt.Sprintf("{\"username\":\"%s\",\"password\":\"%s\"}", username, password)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s/resetPassword", serviceURL, databaseID), bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("failed creating request to reset password for db with id %s with: %w", databaseID, err)
	}
	a.setHeaders(req)
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reset password for database id %s with: %w", databaseID, err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return readErrorFromResponse(res, 200)
	}
	return nil
}

// GetTierInfo Returns all supported tier, cloud, region, count, and capacitity combinations
// @return ([]TierInfo, error)
func (a *AuthenticatedClient) GetTierInfo() ([]TierInfo, error) {
	var ti []TierInfo
	req, err := http.NewRequest("GET", "https://api.astra.datastax.com/v2/availableRegions", http.NoBody)
	if err != nil {
		return []TierInfo{}, fmt.Errorf("failed creating request for tier info with: %w", err)
	}
	a.setHeaders(req)

	res, err := a.client.Do(req)
	if err != nil {
		return []TierInfo{}, fmt.Errorf("failed listing tier info with: %w", err)
	}
	defer closeBody(res)
	if res.StatusCode != 200 {
		return []TierInfo{}, readErrorFromResponse(res, 200)
	}
	err = json.NewDecoder(res.Body).Decode(&ti)
	if err != nil {
		return []TierInfo{}, fmt.Errorf("unable to decode response with error: %w", err)
	}
	return ti, nil
}

// DatabaseInfo is some database meta data info
type DatabaseInfo struct {
	// Name of the database--user friendly identifier
	Name string `json:"name,omitempty"`
	// Keyspace name in database
	Keyspace string `json:"keyspace,omitempty"`
	// CloudProvider where the database lives
	CloudProvider string `json:"cloudProvider,omitempty"`
	// Tier defines the compute power (vertical scaling) for the database
	Tier string `json:"tier,omitempty"`
	// CapacityUnits is the amount of space available (horizontal scaling) for the database. For free tier the max CU's is 1, and 12 for C10 the max is 12 on startup.
	CapacityUnits int32 `json:"capacityUnits,omitempty"`
	// Region refers to the cloud region.
	Region string `json:"region,omitempty"`
	// User is the user to access the database
	User string `json:"user,omitempty"`
	// Password for the user to access the database
	Password string `json:"password,omitempty"`
	// Additional keyspaces names in database
	AdditionalKeyspaces []string `json:"additionalKeyspaces,omitempty"`
}

// MigrationProxyConfiguration of the migration proxy and mappings of astra node to a customer node currently in use
type MigrationProxyConfiguration struct {
	// origin cassandra username
	OriginUsername string `json:"originUsername"`
	// origin cassandra password
	OriginPassword string                  `json:"originPassword"`
	Mappings       []MigrationProxyMapping `json:"mappings"`
}

// MigrationProxyMapping is a mapping of astra node to a customer node currently in use
type MigrationProxyMapping struct {
	// ip on which the node currently in use is accessible
	OriginIP string `json:"originIP"`
	// port on which the node currently in use is accessible
	OriginPort int32 `json:"originPort"`
	// the number of the rack, usually 0, 1, or 2
	Rack int32 `json:"rack"`
	// The number of the node in a given rack, starting with 0
	RackNodeOrdinal int32 `json:"rackNodeOrdinal"`
}

// RegionCombination defines a Tier, cloud provider, region combination
type RegionCombination struct {
	Tier          string `json:"tier"`
	CloudProvider string `json:"cloudProvider"`
	Region        string `json:"region"`
	Cost          *Costs `json:"cost"`
}

// TierInfo defines a Tier, cloud provider, region combination
type TierInfo struct {
	Tier                            string `json:"tier"`
	CloudProvider                   string `json:"cloudProvider"`
	Region                          string `json:"region"`
	Cost                            *Costs `json:"cost"`
	DatabaseCountUsed               int32  `json:"databaseCountUsed"`
	DatabaseCountLimit              int32  `json:"databaseCountLimit"`
	CapacityUnitsUsed               int32  `json:"capacityUnitsUsed"`
	CapacityUnitsLimit              int32  `json:"capacityUnitsLimit"`
	DefaultStoragePerCapacityUnitGb int32  `json:"defaultStoragePerCapacityUnitGb"`
}

// Costs are the total costs skus for the given tier
type Costs struct {
	CostPerMinCents         float64 `json:"costPerMinCents,omitempty"`
	CostPerHourCents        float64 `json:"costPerHourCents,omitempty"`
	CostPerDayCents         float64 `json:"costPerDayCents,omitempty"`
	CostPerMonthCents       float64 `json:"costPerMonthCents,omitempty"`
	CostPerMinParkedCents   float64 `json:"costPerMinParkedCents,omitempty"`
	CostPerHourParkedCents  float64 `json:"costPerHourParkedCents,omitempty"`
	CostPerDayParkedCents   float64 `json:"costPerDayParkedCents,omitempty"`
	CostPerMonthParkedCents float64 `json:"costPerMonthParkedCents,omitempty"`
}

// Database is the returned data from the Astra DevOps API
type Database struct {
	ID      string       `json:"id"`
	OrgID   string       `json:"orgId"`
	OwnerID string       `json:"ownerId"`
	Info    DatabaseInfo `json:"info"`
	// CreationTime in ISO RFC3339 format
	CreationTime string `json:"creationTime,omitempty"`
	// TerminationTime in ISO RFC3339 format
	TerminationTime  string     `json:"terminationTime,omitempty"`
	Status           StatusEnum `json:"status"`
	Storage          Storage    `json:"storage,omitempty"`
	AvailableActions []string   `json:"availableActions,omitempty"`
	// Message to the customer about the cluster
	Message         string `json:"message,omitempty"`
	StudioURL       string `json:"studioUrl,omitempty"`
	GrafanaURL      string `json:"grafanaUrl,omitempty"`
	CqlshURL        string `json:"cqlshUrl,omitempty"`
	GraphqlURL      string `json:"graphqlUrl,omitempty"`
	DataEndpointURL string `json:"dataEndpointUrl,omitempty"`
}

// SecureBundle from which the creds zip may be downloaded
type SecureBundle struct {
	// DownloadURL is only valid for about 5 minutes
	DownloadURL string `json:"downloadURL"`
	// Internal DownloadURL is only valid for about 5 minutes
	DownloadURLInternal string `json:"downloadURLInternal,omitempty"`
	// Migration Proxy DownloadURL is only valid for about 5 minutes
	DownloadURLMigrationProxy string `json:"downloadURLMigrationProxy,omitempty"`
	// Internal Migration Proxy DownloadURL is only valid for about 5 minutes
	DownloadURLMigrationProxyInternal string `json:"downloadURLMigrationProxyInternal,omitempty"`
}

// CreateDb object for submitting a new database
type CreateDb struct {
	// Name of the database--user friendly identifier
	Name string `json:"name"`
	// Keyspace name in database
	Keyspace string `json:"keyspace"`
	// CloudProvider where the database lives
	CloudProvider string `json:"cloudProvider"`
	// Tier defines the compute power (vertical scaling) for the database, developer gcp is the free tier.
	Tier string `json:"tier"`
	// CapacityUnits is the amount of space available (horizontal scaling) for the database. For free tier the max CU's is 1, and 100 for CXX/DXX the max is 12 on startup.
	CapacityUnits int32 `json:"capacityUnits"`
	// Region refers to the cloud region.
	Region string `json:"region"`
	// User is the user to access the database
	User string `json:"user"`
	// Password for the user to access the database
	Password string `json:"password"`
}

// TokenResponse comes from the classic service account auth
type TokenResponse struct {
	Token  string  `json:"token"`
	Errors []Error `json:"errors"`
}

// ErrorResponse when the API has an error
type ErrorResponse struct {
	Errors []Error `json:"errors"`
}

// Error when the api has an error this is the structure
type Error struct {
	// API specific error code
	ID int32 `json:"ID,omitempty"`
	// User-friendly description of error
	Message string `json:"message"`
}

// Storage contains the information about how much storage space a cluster has available
type Storage struct {
	// NodeCount for the cluster
	NodeCount int32 `json:"nodeCount"`
	// ReplicationFactor is the number of nodes storing a piece of data
	ReplicationFactor int32 `json:"replicationFactor"`
	// TotalStorage of the cluster in GB
	TotalStorage int32 `json:"totalStorage"`
	// UsedStorage in GB
	UsedStorage int32 `json:"usedStorage,omitempty"`
}

// FormatErrors puts the API errors into a well formatted text output
func FormatErrors(es []Error) string {
	var formatted []string
	for _, e := range es {
		formatted = append(formatted, fmt.Sprintf("ID: %v Text: '%v'", e.ID, e.Message))
	}
	return strings.Join(formatted, ", ")
}

func closeBody(res *http.Response) {
	if err := res.Body.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to close request body '%v'", err)
	}
}
