package main

import (
	"fmt"
	"strconv"
	"github.com/gocolly/colly"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"log"
	"strings"
	"time"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DbHost string `required:"true" split_words:"true"`
	DbName string `required:"true" split_words:"true"`
	DbUser string `required:"true" split_words:"true"`
	DbPw   string `required:"true" split_words:"true"`
}

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

var cfg Config

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
	err := envconfig.Process("attendance", &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	dbConnectString := fmt.Sprintf("host=%s port=5432 user=%s dbname=%s password=%s sslmode=disable",
		cfg.DbHost, cfg.DbUser, cfg.DbName, cfg.DbPw)
	//conect to db
	db, err := gorm.Open(
		"postgres",
		dbConnectString,
	)
	// Connect to database
	defer db.Close()
	db.SingularTable(true)
	if err != nil {
		panic(err.Error())
	}
	// Instantiate collector
	c := colly.NewCollector(
		colly.MaxDepth(2),
	)
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
		dayStart := bod(*match.StartTime)
		dayEnd := dayStart.Add(24 * time.Hour)
		var oldMatch Match
		result := db.Where("start_time >= ? AND start_time < ? AND home_team = ?", dayStart, dayEnd, match.HomeTeam).Assign(&match).FirstOrCreate(&oldMatch)
		if result.Error != nil {
			fmt.Printf("Failed to store %+v", match)
		}
		fmt.Println("Match done")
	})
	c.Visit("https://www.bold.dk/fodbold/kampe/danmark/")
}
