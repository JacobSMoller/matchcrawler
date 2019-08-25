package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// Match to be stored in the database.
type Match struct {
	ID         int        `gorm:"id" json:"id"`
	Tournament string     `gorm:"tournament" json:"tournament"`
	StartTime  *time.Time `gorm:"start_time" json:"start_time"`
	Spectators *int       `gorm:"spectators" json:"spectators"`
	State      string     `gorm:"state" json:"state"`
	Referee    string     `gorm:"referee" json:"referee"`
	HomeTeam   string     `gorm:"home_team" json:"home_team"`
	AwayTeam   string     `gorm:"away_team" json:"away_team"`
}

func replaceMonth(date string) (*time.Time, error) {
	months := map[string]string{
		"januar":    "January",
		"februar":   "February",
		"marts":     "March",
		"april":     "April",
		"maj":       "May",
		"juni":      "June",
		"juli":      "July",
		"august":    "August",
		"september": "September",
		"oktober":   "October",
		"november":  "November",
		"december":  "December",
	}
	layout := "2. January 2006 kl. 15:04"
	for da, en := range months {
		if strings.Contains(date, da) {
			date = strings.Replace(date, da, en, 1)
			break
		}
	}
	parsed, err := time.Parse(layout, date)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func createCall(client *http.Client, match *Match) {
	jsonMatch, err := json.Marshal(match)
	if err != nil {
		fmt.Println("Could not marshal match to json")
	}
	req, err := http.NewRequest("POST", "https://74ca51f0.ngrok.io/match/create", bytes.NewBuffer(jsonMatch))
	if err != nil {
		panic("Could not create http request")
	}
	_, err = client.Do(req)
	if err != nil {
		panic("Error during call to create match")
	}
}

func updateCall(client *http.Client, match *Match) {
	jsonMatch, err := json.Marshal(match)
	if err != nil {
		fmt.Println("Could not marshal match to json")
	}
	req, err := http.NewRequest("PUT", "https://74ca51f0.ngrok.io/match/update", bytes.NewBuffer(jsonMatch))
	if err != nil {
		panic("Could not create http request")
	}
	_, err = client.Do(req)
	if err != nil {
		panic("Error during call to create match")
	}
}

func main() {
	client := &http.Client{Timeout: time.Second * 5}
	// Instantiate collector
	c := colly.NewCollector(
		colly.MaxDepth(2),
	)
	err := c.Limit(&colly.LimitRule{
		DomainGlob:  "bold.dk/fodbold/*",
		Delay:       2 * time.Second,
		RandomDelay: 1 * time.Second,
	})
	if err != nil {
		fmt.Printf("call to colly Limit failed")
	}
	teamName := "fc-koebenhavn"
	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		if strings.Contains(link, teamName) && strings.Contains(link, "vs") {
			split := strings.Split(link, "/")
			teams := strings.Split(split[len(split)-3], "-")
			if teams[0]+"-"+teams[1]+"-"+teams[2] != teamName+"-"+"vs" {
				return
			}
			err = e.Request.Visit(link)
			if err != nil {
				fmt.Printf("Failed to visit link %s %s \n", link, err.Error())
			}
		}
	})
	c.OnHTML("#match_update", func(e *colly.HTMLElement) {
		splitLink := strings.Split(e.Request.URL.Path, "/")
		// -2 to offset .Path ending with /.
		matchID, err := strconv.Atoi(splitLink[len(splitLink)-2])
		if err != nil {
			fmt.Println("Failed to parse matchID to int")
		}
		match := Match{}
		match.ID = matchID
		matchTeams := make([]string, 0, 2)
		e.ForEach("table.match", func(_ int, elem *colly.HTMLElement) {
			elem.ForEach("span", func(_ int, span *colly.HTMLElement) {
				matchTeams = append(matchTeams, span.Text)
			})
		})
		match.HomeTeam = matchTeams[0]
		match.AwayTeam = matchTeams[1]
		e.ForEach("div.info_box", func(_ int, elem *colly.HTMLElement) {
			switch elem.ChildText("div.subject") {
			case "Turnering:":
				match.Tournament = elem.ChildText("div.result")
			case "Spilstart:":
				date, err := replaceMonth(elem.ChildText("div.result"))
				if err != nil {
					fmt.Printf("Failed to parse date %s", elem.ChildText("div.result"))
					date = nil
				}
				match.StartTime = date
			case "Tilskuere:":
				spectatorsNo := strings.Split(elem.ChildText("div.result"), " ")[0]
				spectators, err := strconv.Atoi(strings.Replace(spectatorsNo, ".", "", 2))
				if err != nil {
					fmt.Println("Failed to parse spectators")
					match.Spectators = nil
				} else {
					match.Spectators = &spectators
				}
			case "Dommer:":
				match.Referee = elem.ChildText("div.result")
			case "Status:":
				match.State = elem.ChildText("div.result")
			}
		})

		getMatchURL := fmt.Sprintf("https://74ca51f0.ngrok.io/match/%d", match.ID)
		resp, err := client.Get(getMatchURL)
		if err != nil {
			panic("Error during get match call")
		}
		if resp.StatusCode == 404 {
			fmt.Println("Creating match")
			createCall(client, &match)
		} else {
			b, err := ioutil.ReadAll(resp.Body)
			defer resp.Body.Close()
			if err != nil {
				panic("Failed to read response body")
			}
			var existingMatch Match
			err = json.Unmarshal(b, &existingMatch)
			if err != nil {
				panic("Failed to unmarshal existing match json")
			}
			existingState := strings.Split(existingMatch.State, " ")[0]
			currentStaste := strings.Split(match.State, " ")[0]
			if existingState != "Færdig" && currentStaste == "Færdig" {
				fmt.Println("Updating match")
				updateCall(client, &match)
			}
		}
		fmt.Println("Match done")
	})
	err = c.Visit("https://www.bold.dk/fodbold/kampe/danmark/")
	if err != nil {
		panic(err.Error())
	}
}
