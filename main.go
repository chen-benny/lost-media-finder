package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	maxVideos = 100
	rate      = 500 * time.Millisecond
)

var (
	visited    = make(map[string]bool)
	queue      = []string{"https://www.vidlii.com"}
	videoCount = 0
	pageCount  = 0
	dateRe     = regexp.MustCompile(`[A-Z][a-z]{2} \d{1,2}, \d{4}`)
)

func main() {
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

		pageCount++
		log.Printf("[%d] %s", pageCount, url)

		// if video page
		if strings.Contains(url, "/watch?v=") {
			videoCount++
			title := strings.TrimSuffix(doc.Find("title").Text(), " - VidLii")
			date := dateRe.FindString(doc.Text())
			log.Printf("[%d] %s | %s | %s", videoCount, title, date, url)
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
}
