package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const (
	maxVideos = 100
	rate      = 500 * time.Millisecond
)

var (
	visited    = make(map[string]bool)
	queue      = []string{"https://www.vidlii.com"] //, "https://www.vidlii.com/user/rinkomania"}
	videoCount = 0
	// pageCount  = 0
	dateRe     = regexp.MustCompile(`[A-Z][a-z]{2} \d{1,2}, \d{4}`)
	cutoffDate = time.Date(2021, 12, 31, 23, 59, 59, 0, time.UTC)
)

type Video struct {
	URL   string
	Title string
	Date  time.Time
}

func (v Video) Match() bool {
	// date before 2021-12-31
	if v.Date.IsZero() || !v.Date.Before(cutoffDate) {
		return false
	}
	// title contains Japanese characters
	for _, ch := range v.Title {
		if unicode.In(ch, unicode.Hiragana, unicode.Katakana, unicode.Han) {
			return true
		}
	}
	return false
}

func main() {
	var targets []Video

	for len(queue) > 0 && videoCount < maxVideos {
		url := queue[0]
		queue = queue[1:]

		if visited[url] {
			continue
		}
		visited[url] = true

		time.Sleep(rate)

		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		// pageCount++
		// log.Printf("[%d] %s", pageCount, url)

		// if video page
		if strings.Contains(url, "/watch?v=") {
			videoCount++
			title := strings.TrimSuffix(doc.Find("title").Text(), " - VidLii")
			dateStr := dateRe.FindString(doc.Text())
			date, _ := time.Parse("Jan 2, 2006", dateStr)

			v := Video{URL: url, Title: title, Date: date}
			log.Printf("[%d] %s | %s | %s", videoCount, title, date, url)
			if v.Match() {
				targets = append(targets, v)
				log.Printf("Found One Match: %s", url)
			}
		}

		// append new URLs
		doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
				href = "https://www.vidlii.com" + href
			}
			if strings.HasPrefix(href, "https://www.vidlii.com") && !visited[href] {
				queue = append(queue, href)
			}
		})
	}

	fmt.Printf("\nDone, visited %d videos\n", videoCount)

	for i, video := range targets {
		fmt.Println(i+1, video.URL)
	}
}
