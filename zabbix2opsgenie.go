/*
   	Edge encryption compatible version of zabbix2opsgenie.

		Other changes:
			- Priority Mapping
			- Config independent of marid/OEC

		@author: Robert Pratt <robert.pratt@homelab.farm>
*/
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/logutils"
)

var confPath = "/etc/opsgenie/zabbix2opsgenie.json"
var logPath = "/etc/opsgenie/zabbix2opsgenie.log" // Log here because of OEC directory permissions.

type AlertPayload struct {
	Alias       string   `json:"alias"`
	Message     string   `json:"message"`
	Source      string   `json:"source"`
	Entity      string   `json:"entity"`
	Description string   `json:"description"`
	User        string   `json:"user"`
	Note        string   `json:"note"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags"`
	Details     Details  `json:"details"`
	Responders  []*Teams `json:"responders"`
}

type Teams struct {
	TeamName string `json:"name"`
	Type     string `json:"type"`
}

type Details struct {
	EventID         string `json:"eventId"`
	TriggeredMetric string `json:"Triggered Metric Value"`
	IPAddress       string `json:"IP Address"`
	Hostname        string `json:"Hostname"`
	ItemValue       string `json:"Item Value"`
	ItemKey         string `json:"Item Key"`
	TriggerID       string `json:"Trigger ID"`
}

type ConfigParameters struct {
	ApiKey   string `json:"apiKey"`
	EdgeUrl  string `json:"edgeAppUrl"`
	OgTeams  string `json:"teams"`
	OgTags   string `json:"tags"`
	LogLevel string `json:"logLevel"`
}

// Local instance of ConfigParameters struct
var configParameters ConfigParameters

func main() {
	zabbixEventFields := parseFlags()

	configFile, err := os.Open(confPath)
	if err != nil {
		fmt.Printf("[ERROR] Unable to read config file %s, exiting...", confPath)
	}
	defer configFile.Close()

	err = parseJsonConfig(configFile, &configParameters)

	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("[ERROR] Could not open/create log file, check file/directory permissions.")
	}

	logFilter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(configParameters.LogLevel),
		Writer:   logFile,
	}
	log.SetOutput(logFilter)

	// Assign fields provided by Zabbix to Alert. Then marshal
	jsonAlertPayload, err := mapFieldsToAlert(zabbixEventFields)
	if err != nil {
		log.Fatal("[ERROR] Conversion alert API format failed, exiting... ", err)
	}

	if configParameters.LogLevel == "DEBUG" {
		log.Println("[DEBUG] Dumping event payload:")
		for i, j := range zabbixEventFields {
			log.Printf("[DEBUG] %s : %s", i, j)
		}
	}

	err = evalZabbixEvent(zabbixEventFields, jsonAlertPayload)
	if err != nil {
		log.Fatal("[ERROR] Failed to send Zabbix event to Opsgenie: ", err)
	}
}

func evalZabbixEvent(zabbixEventFields map[string]string, jsonAlertPayload []byte) (err error) {
	url := configParameters.EdgeUrl
	// Check trigger status and generate create/close URL
	if zabbixEventFields["triggerStatus"] == "PROBLEM" {
		url = url + "/v2/alerts"
		log.Print("[DEBUG] Posting event data to: ", url)
		log.Print("[WARN] Creating alert with alias ", zabbixEventFields["triggerId"]+"_"+zabbixEventFields["hostName"])

		err = postToOpsgenie("POST", url, jsonAlertPayload)
	} else if zabbixEventFields["triggerStatus"] == "OK" {
		url = url + "/v2/alerts/" + zabbixEventFields["triggerId"] + "_" + zabbixEventFields["hostName"] + "/close?identifierType=alias"
		log.Print("[DEBUG] Posting event data to ", url)
		log.Print("[WARN] Closing alert with alias ", zabbixEventFields["triggerId"]+"_"+zabbixEventFields["hostName"])

		err = postToOpsgenie("POST", url, jsonAlertPayload)
	} else {
		log.Print("[WARN] No matching actions found for event: ", zabbixEventFields["eventId"])
	}
	return
}

func mapFieldsToAlert(zabbixEventFields map[string]string) (jsonAlertPayload []byte, err error) {
	// Check for teams passed as cmd arg, fallback to config file, and default to empty
	if zabbixEventFields["teams"] == "" {
		zabbixEventFields["teams"] = configParameters.OgTeams
	}
	// Same for tags
	if zabbixEventFields["tags"] == "" {
		zabbixEventFields["tags"] = configParameters.OgTags
	}
	// Same for apiKey
	if zabbixEventFields["apiKey"] == "" {
		zabbixEventFields["apiKey"] = configParameters.ApiKey
	}

	// Split responders into a map.
	teamSlice := strings.Split(zabbixEventFields["teams"], ",")
	var responders []*Teams

	for _, team := range teamSlice {
		log.Print("[DEBUG] Team slice parsed as ", team)
		responders = append(responders, &Teams{team, "team"})
	}

	payload := AlertPayload{
		Alias:       zabbixEventFields["triggerId"] + "_" + zabbixEventFields["hostName"],
		Message:     "[Zabbix] " + zabbixEventFields["triggerName"],
		Source:      "Zabbix",
		Entity:      zabbixEventFields["hostName"],
		Tags:        strings.Split(zabbixEventFields["tags"], ","),
		Description: zabbixEventFields["triggerDescription"],
		User:        "Zabbix",
		Priority:    mapPriority(zabbixEventFields["triggerSeverity"]),
		Responders:  responders,
		Details: Details{
			EventID:         zabbixEventFields["eventId"],
			TriggeredMetric: zabbixEventFields["triggerValue"],
			IPAddress:       zabbixEventFields["ipAddress"],
			Hostname:        zabbixEventFields["hostName"],
			ItemValue:       zabbixEventFields["itemValue"],
			ItemKey:         zabbixEventFields["itemKey"],
			TriggerID:       zabbixEventFields["triggerId"],
		},
	}
	jsonAlertPayload, err = json.Marshal(payload)
	return
}

func postToOpsgenie(method string, url string, alertPayload []byte) (err error) {
	payload := bytes.NewBuffer(alertPayload)
	req, err := http.NewRequest(method, url, payload)
	req.Header.Add("Content-type", "application/json")
	req.Header.Add("Authorization", "GenieKey "+configParameters.ApiKey)
	if err != nil {
		log.Fatal("[ERROR] Failed to create HTTP request: ", err)
	}

	// Catch tests before posting.
	if url == "https://test.edge.encryption.host" {
		return nil
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("[ERROR] Failed to post data to Opsgenie: ", err)
	} else if res.StatusCode <= 200 && res.StatusCode >= 299 {
		log.Printf("[ERROR] Failed to post data to Opsgenie. HTTP Status Code %s", res.Status)
	} else {
		body, _ := ioutil.ReadAll(res.Body)
		log.Print("[WARN] Successfully posted event to Opsgenie: ", string(body))
	}
	defer res.Body.Close()
	return
}

func parseJsonConfig(file io.Reader, configParameters *ConfigParameters) (err error) {
	jsonParser := json.NewDecoder(file)
	err = jsonParser.Decode(&configParameters)
	return
}

func mapPriority(severity string) (priority string) {
	switch {
	case severity == "Information":
		priority = "P5"
	case severity == "Warning":
		priority = "P4"
	case severity == "Average":
		priority = "P3"
	case severity == "High":
		priority = "P2"
	case severity == "Disaster":
		priority = "P1"
	}
	return
}

func parseFlags() (alertFields map[string]string) {
	apiKey := flag.String("apiKey", "", " Event field from Zabbix")
	triggerName := flag.String("triggerName", "", "Event field from Zabbix")
	triggerId := flag.String("triggerId", "", " Event field from Zabbix")
	triggerStatus := flag.String("triggerStatus", "", " Event field from Zabbix")
	triggerSeverity := flag.String("triggerSeverity", "", " Event field from Zabbix")
	triggerDescription := flag.String("triggerDescription", "", " Event field from Zabbix")
	triggerUrl := flag.String("triggerUrl", "", " Event field from Zabbix")
	triggerValue := flag.String("triggerValue", "", " Event field from Zabbix")
	triggerHostGroupName := flag.String("triggerHostGroupName", "", " Event field from Zabbix")
	hostName := flag.String("hostName", "", " Event field from Zabbix")
	ipAddress := flag.String("ipAddress", "", " Event field from Zabbix")
	eventId := flag.String("eventId", "", " Event field from Zabbix")
	date := flag.String("date", "", " Event field from Zabbix")
	time := flag.String("time", "", " Event field from Zabbix")
	itemKey := flag.String("itemKey", "", " Event field from Zabbix")
	itemValue := flag.String("itemValue", "", " Event field from Zabbix")
	recoveryEventStatus := flag.String("recoveryEventStatus", "", " Event field from Zabbix")
	tags := flag.String("tags", "", "Tags for Opsgenie alert")
	recipients := flag.String("recipients", "", "Recipients of Opsgenie alert")
	teams := flag.String("teams", "", "Teams to assign the Opsgenie alert to")

	flag.Parse()

	alertFields = map[string]string{
		"apiKey":               *apiKey,
		"triggerName":          *triggerName,
		"triggerId":            *triggerId,
		"triggerStatus":        *triggerStatus,
		"triggerSeverity":      *triggerSeverity,
		"triggerDescription":   *triggerDescription,
		"triggerUrl":           *triggerUrl,
		"triggerValue":         *triggerValue,
		"triggerHostGroupName": *triggerHostGroupName,
		"hostName":             *hostName,
		"ipAddress":            *ipAddress,
		"eventId":              *eventId,
		"date":                 *date,
		"time":                 *time,
		"itemKey":              *itemKey,
		"itemValue":            *itemValue,
		"recoveryEventStatus":  *recoveryEventStatus,
		"tags":                 *tags,
		"recipients":           *recipients,
		"teams":                *teams,
	}
	return
}
