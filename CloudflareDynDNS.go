package main

import (
	"encoding/json"
	"fmt"
	"github.com/cloudflare/cloudflare-go"
	"github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Zone struct {
	Zone  string `json:"zone"`
	Proxy bool   `json:"proxy"`
}

type Config struct {
	ApiKey       string `json:"apikey"`
	Email        string `json:"email"`
	DefaultZones []Zone `json:"defaultzones"`
	Websites     []struct {
		Domain []string `json:"domain"`
		Zones  []Zone   `json:"zones"`
	} `json:"websites"`
}

var config Config

func main() {
	config = loadConfig("config.json")

	systray.Run(onReady, onExit)
}

func loadConfig(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return config
}

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTitle("Cloudflare Dyndns")
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	mQuit.SetIcon(icon.Data)

	var previousIP = "nil"
	var lastIPChange = time.Now()
	var lastCheck = time.Now()

	for {
		log.Println("Checking for IP address changes...")
		lastCheck = time.Now()
		newIP := getExternalIP()

		if newIP != "" && previousIP != newIP {
			log.Printf("IP Address Changed %s -> %s", previousIP, newIP)
			updateDnsRecords(newIP)

			lastIPChange = time.Now()
			previousIP = newIP
		}

		var tooltip = "Changed: " + lastIPChange.Format("01/02/2006 3:04 PM")
		tooltip += "\nChecked: " + lastCheck.Format("01/02/2006 3:04 PM")

		systray.SetTooltip(tooltip)

		log.Println("Sleeping......")
		time.Sleep(60 * time.Second)
	}
}

func onExit() {
	// clean up here
}

func updateDnsRecords(ip string) {
	api, err := cloudflare.New(config.ApiKey, config.Email)
	if err != nil {
		log.Fatal(err)
		return
	}

	for _, host := range config.Websites {
		for _, domain := range host.Domain {
			for _, z := range append(host.Zones, config.DefaultZones...) {
				zoneID, err := api.ZoneIDByName(domain)
				if err != nil {
					log.Fatal(err)
					continue
				}

				var name = z.Zone + "." + domain
				if z.Zone == "@" {
					name = domain
				}

				newRecord := cloudflare.DNSRecord{
					Type:    "A",
					Name:    name,
					Content: ip,
					Proxied: z.Proxy,
				}

				updateRecord(zoneID, api, &newRecord)
				log.Println("Updated DNSRecord:", newRecord.Name, newRecord.Content)
			}
		}
	}
}

func updateRecord(zoneID string, api *cloudflare.API, newRecord *cloudflare.DNSRecord) {
	dns := cloudflare.DNSRecord{Type: newRecord.Type, Name: newRecord.Name}

	oldRecords, err := api.DNSRecords(zoneID, dns)
	if err != nil {
		log.Fatal(err)
		return
	}

	if len(oldRecords) == 1 {
		err := api.UpdateDNSRecord(zoneID, oldRecords[0].ID, *newRecord)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(oldRecords) > 1 {
		for _, record := range oldRecords {
			err := api.DeleteDNSRecord(zoneID, record.ID)
			if err != nil {
				log.Fatal(err)
				return
			}
			log.Printf("Deleted DNSRecord: %s - %s: %s", record.Type, record.Name, record.Content)
		}
	}

	_, err = api.CreateDNSRecord(zoneID, *newRecord)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func getExternalIP() string {
	resp, err := http.Get("https://myexternalip.com/raw")
	if err == nil {
		contents, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			defer resp.Body.Close()
			return strings.TrimSpace(string(contents))
		}
	}
	return ""
}
