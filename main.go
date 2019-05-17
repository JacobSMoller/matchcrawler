package main

import (
	"fmt"
	"strings"

	"github.com/gocolly/colly"
)

func main() {
	// Instantiate default collector
	c := colly.NewCollector(
		// MaxDepth is 1, so only the links on the scraped page
		// is visited, and no further links are followed
		colly.MaxDepth(1),
	)
	teamName := "broendby-if"

	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// fmt.Println(link)
		if strings.Contains(link, teamName) {
			split := strings.Split(link, "/")
			teams := strings.Split(split[len(split)-3], "-")
			fmt.Println(teams[0]+"-"+teams[1] == teamName && teams[2] != "vs")
			if teams[0]+"-"+teams[1] != teamName && teams[2] != "vs" {
				return
			}
			fmt.Println("FOFOFO")
			fmt.Println(link)
			e.Request.Visit(link)
		}
	})

	// Start scraping on https://en.wikipedia.org
	c.Visit("https://www.bold.dk/fodbold/kampe/danmark/")
}
