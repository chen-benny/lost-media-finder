package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const (
	maxVideos    = 100
	workers      = 4
	bufferSize   = 10000
	baseUrl      = "https://www.vidlii.com"
	testUrl      = "https://www.vidlii.com/user/rinkomania"
	videoPattern = "/watch?v="
	titleSuffix  = " - VidLii"
)

// the reported date is before 2022
var (
	cutoffDate = time.Date(2021, 12, 31, 23, 59, 59, 0, time.UTC)
)

type Video struct {
	URL   string
	Title string
	Date  time.Time
}

// Match selects video that has Japanese chars in title and date before cutoff
func (v Video) Match() bool {
	if v.Date.IsZero() || !v.Date.Before(cutoffDate) {
		return false
	}
	for _, c := range v.Title {
		if unicode.In(c, unicode.Hiragana, unicode.Katakana, unicode.Han) {
			return true
		}
	}
	return false
}

type Crawler struct {
	visited sync.Map
	targets []Video
	mu      sync.Mutex
	count   int
	queue   chan string
	ticker  *time.Ticker
}

func NewCrawler() *Crawler {
	return &Crawler{
		queue:  make(chan string, bufferSize),
		ticker: time.NewTicker(time.Millisecond * 500),
	}
}

func (c *Crawler) done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count > maxVideos
}

func (c *Crawler) worker() {
	for url := range c.queue {
		c.process(url)
	}
}

func (c *Crawler) process(url string) {
	if c.done() {
		return
	}

	if _, seen := c.visited.LoadOrStore(url, true); seen {
		return
	}

	<-c.ticker.C

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching %s: %s", url, err)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Printf("Error parsing %s: %s", url, err)
		return
	}

	log.Printf("parsed %s: %s", url, resp.Status)

	if strings.Contains(url, videoPattern) {
		// transform e.g. https://www.vidlii.com/user/rinkomania/watch?v=K5u6i67Cg3e into baseUrl + videoPattern + vId
		idx := strings.Index(url, videoPattern)
		url = baseUrl + url[idx:]

		c.mu.Lock()
		c.count++
		n := c.count
		c.mu.Unlock()

		title := strings.TrimSuffix(doc.Find("title").Text(), titleSuffix)
		dateStr := strings.TrimSpace(doc.Find("date").First().Text())
		date, _ := time.Parse("Jan 2, 2006", dateStr)

		v := Video{URL: url, Title: title, Date: date}
		log.Printf("[%d] %s | %s", n, title, dateStr)

		if v.Match() {
			c.mu.Lock()
			c.targets = append(c.targets, v)
			c.mu.Unlock()
			log.Printf("Found one target: %s", v.Title)
		}
	}

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
			href = baseUrl + href
		}
		if strings.HasPrefix(href, baseUrl) {
			select {
			case c.queue <- href:
			default:
			}
		}
	})
}

func (c *Crawler) Run() {
	c.queue <- testUrl

	for i := 0; i < workers; i++ {
		go c.worker()
	}

	for !c.done() {
		time.Sleep(time.Millisecond * 500)
	}
}

func main() {
	c := NewCrawler()
	defer c.ticker.Stop()
	c.Run()
	fmt.Printf("targets found: %d\n", len(c.targets))
	for _, v := range c.targets {
		fmt.Printf("%s | %s | %s\n", v.Title, v.Date.Format("Jan 2, 2006"), v.URL)
	}
}
