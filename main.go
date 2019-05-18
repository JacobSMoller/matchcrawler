package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// Match to be stored in the database.
type Match struct {
	Tournament string     `gorm:"tournament"`
	StartTime  *time.Time `gorm:"start_time"`
	Spectators *int       `gorm:"spectators"`
	State      string     `gorm:"state"`
	Referee    string     `gorm:"referee"`
	HomeTeam   string     `gorm:"home_team"`
	AwayTeam   string     `gorm:"away_team"`
}

// returns date at beginning of day.
func bod(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// MatchExists checks if a match already exists based on date and hometeam.
func (m Match) MatchExists(db *gorm.DB) bool {
	var match Match
	dayStart := bod(*m.StartTime)
	dayEnd := dayStart.Add(24 * time.Hour)
	result := db.Table("match").Select("*").Where("start_time >= ? AND start_time < ? AND home_team = ?", dayStart, dayEnd, m.HomeTeam).Scan(&match)
	return !result.RecordNotFound()
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

func main() {
	// Connect to database
	db, err := gorm.Open(
		"postgres",
		"host=localhost port=5432 user=postgres dbname=attendance password=docker sslmode=disable",
	)
	defer db.Close()
	if err != nil {
		panic(err.Error())
	}
	// Instantiate collector
	c := colly.NewCollector()
	c.Limit(&colly.LimitRule{
		DomainGlob:  "bold.dk/fodbold/*",
		Delay:       2 * time.Second,
		RandomDelay: 1 * time.Second,
	})
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
			e.Request.Visit(link)
		}
	})
	c.OnHTML("#match_update", func(e *colly.HTMLElement) {
		match := Match{}
		matchTeams := strings.Split(e.ChildAttr(`meta[itemprop="name"]`, "content"), " - ")
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
		// Only add match to DB if it isn't already there.
		if match.MatchExists(db) {
			return
		}
		result := db.Table("match").Create(&match)
		if result.Error != nil {
			fmt.Printf("Failed to store %+v", match)
		}
		fmt.Println("Match done")
	})
	c.Visit("https://www.bold.dk/fodbold/kampe/danmark/")
}
