# astra-devops-sdk-go

[![.github/workflows/go.yaml](https://github.com/rsds143/astra-devops-sdk-go/actions/workflows/go.yaml/badge.svg)](https://github.com/rsds143/astra-devops-sdk-go/actions/workflows/go.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/rsds143/astra-devops-sdk-go)](https://goreportcard.com/report/github.com/rsds143/astra-devops-sdk-go)
[![Go Version](https://img.shields.io/github/go-mod/go-version/rsds143/astra-devops-sdk-go)](https://img.shields.io/github/go-mod/go-version/rsds143/astra-devops-sdk-go)
[![Latest Version](https://img.shields.io/github/v/tag/rsds143/astra-devops-sdk-go)](https://github.com/rsds143/astra-devops-sdk-go/tags)
[![Go Reference](https://pkg.go.dev/badge/github.com/rsds143/astra-devops-sdk-go.svg)](https://pkg.go.dev/github.com/rsds143/astra-devops-sdk-go)
[![Coverage Status](https://coveralls.io/repos/github/rsds143/astra-devops-sdk-go/badge.svg?branch=main)](https://coveralls.io/github/rsds143/astra-devops-sdk-go?branch=main)

Go API bindings for the Astra DevOps API with zero external dependencies. Apache 2.0 License.

## How to use

### Login Token

```go

import "github.com/rsds143/astra-devops-sdk-go/astraops"

func main() {
//using an auth token

  verbose := true
  client, err := astraopsv1.AuthenticateToken("AstraCS:scrambled:scrabmled", verbose)
}
```

### Login Legacy Service Account

```go

import "github.com/rsds143/astra-devops-sdk-go/astraops"

func main() {
//using a legacy service account
 c :=  ClientInfo {
	        ClientName: "me@example.com",
	        ClientID:   "3e241be3-2a5f-4cb1-b702-739e38975b1a",
	        ClientSecret: "33c338b0-91b5-45a6-be14-416059deb820",
  }
  verbose := true
  client, err := astraopsv1.Authenticate(c, verbose)
}
```

#### Create Database

Will block until creation

```go
createDb := CreateDb{
		Name:          "testerdblist",
		Keyspace:      "mykeyspace",
		Region:        "europe-west1",
		CloudProvider: "GCP",
		CapacityUnits: 1,
		Tier:          "serverless",
		User:          "myuser",
		Password:      "mypass",
	}
//id is a uuid
//db is the following type
//type DataBase struct {
//	ID               string   `json:"id"`
//	OrgID            string   `json:"orgId"`
//	OwnerID          string   `json:"ownerId"`
//	Info             Info     `json:"info"`
//	CreationTime     string   `json:"creationTime"`
//	TerminationTime  string   `json:"terminationTime"`
//	Status           string   `json:"status"`
//	Storage          Storage  `json:"storage"`
//	AvailableActions []string `json:"availableActions"`
//	Message          string   `json:"message"`
//	StudioURL        string   `json:"studioUrl"`
//	GrafanaURL       string   `json:"grafanaUrl"`
//	CqlshURL         string   `json:"cqlshUrl"`
//	GraphqlURL       string   `json:"graphUrl"`
//	DataEndpointURL  string   `json:"dataEndpointUrl"`
//}
id, db, err := client.CreateDb(createDb)
```

### Delete Database

Will block until terminating status or terminated status is returned

```go

//id is database ID one gets when using ListDb or CreateDb
//preparedStateOnly is an internal field, do not use unless directed to by support
preparedStateOnly := false
err := client.Terminate(id, preparedStateOnly)
```

### List All Databases With A Limit Of 10

```go
dbs, err := client.ListDb("", "", "", 10)
//returns an array of DataBases
```

### Get Database by ID

```go
db, err := client.FindDb(id)

//type DataBase struct {
//	ID               string   `json:"id"`
//	OrgID            string   `json:"orgId"`
//	OwnerID          string   `json:"ownerId"`
//	Info             Info     `json:"info"`
//	CreationTime     string   `json:"creationTime"`
//	TerminationTime  string   `json:"terminationTime"`
//	Status           string   `json:"status"`
//	Storage          Storage  `json:"storage"`
//	AvailableActions []string `json:"availableActions"`
//	Message          string   `json:"message"`
//	StudioURL        string   `json:"studioUrl"`
//	GrafanaURL       string   `json:"grafanaUrl"`
//	CqlshURL         string   `json:"cqlshUrl"`
//	GraphqlURL       string   `json:"graphUrl"`
//	DataEndpointURL  string   `json:"dataEndpointUrl"`
}
*/
```

### Get Secure Connect Bundle by ID

```go
secureBundle, err := client.GetSecureBundle(id)

/*
type SecureBundle struct {
	DownloadURL               string `json:"downloadURL"`
	DownloadURLInternal       string `json:"downloadURLInternal"`
	DownloadURLMigrationProxy string `json:"downloadURLMigrationProxy"`
}
*/
```

### Park legacy tier db (non serverless)

Will block until parking complete

```go
err := client.Park(id)
```

### Unpark legacy tier db (non serverless)

Will block until unparking complete, this can take very long

```go
err := client.UnPark(id)
```

### Resize 

```go
var int32 capacityUnits = 5
err := Resize(id, capacityUnits)
```

### List all tiers available

```go
tiers, err := GetTierInfo()
//returns a list of TierInfo
//type TierInfo struct {
//	Tier                            string   `json:"tier"`
//	Description                     string   `json:"description"`
//	CloudProvider                   string   `json:"cloudProvider"`
//	Region                          string   `json:"region"`
//	RegionDisplay                   string   `json:"regionDisplay"`
//	RegionContinent                 string   `json:"regionContinent"`
//	Cost                            TierCost `json:"cost"`
//	DatabaseCountUsed               int      `json:"databaseCountUsed"`
//	DatabaseCountLimit              int      `json:"databaseCountLimit"`
//	CapacityUnitsUsed               int      `json:"capacityUnitsUsed"`
//	CapacityUnitsLimit              int      `json:"capacityUnitsLimit"`
//	DefaultStoragePerCapacityUnitGb int      `json:"defaultStoragePerCapacityUnitGb"`
//}
//
//tier cost has the following fields 
//type TierCost struct {
//	CostPerMinCents         float64 `json:"costPerMinCents"`
//	CostPerHourCents        float64 `json:"costPerHourCents"`
//	CostPerDayCents         float64 `json:"costPerDayCents"`
//	CostPerMonthCents       float64 `json:"costPerMonthCents"`
//	CostPerMinMRCents       float64 `json:"costPerMinMRCents"`
//	CostPerHourMRCents      float64 `json:"costPerHourMRCents"`
//	CostPerDayMRCents       float64 `json:"costPerDayMRCents"`
//	CostPerMonthMRCents     float64 `json:"costPerMonthMRCents"`
//	CostPerMinParkedCents   float64 `json:"costPerMinParkedCents"`
//	CostPerHourParkedCents  float64 `json:"costPerHourParkedCents"`
//	CostPerDayParkedCents   float64 `json:"costPerDayParkedCents"`
//	CostPerMonthParkedCents float64 `json:"costPerMonthParkedCents"`
//	CostPerNetworkGbCents   float64 `json:"costPerNetworkGbCents"`
//	CostPerWrittenGbCents   float64 `json:"costPerWrittenGbCents"`
//	CostPerReadGbCents      float64 `json:"costPerReadGbCents"`
//}
```




