package crawler

import (
	"context"
	"encoding/json"
	"log"
	"lost-media-finder/internal/auth"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"lost-media-finder/internal/config"
	"lost-media-finder/internal/metrics"
	"lost-media-finder/internal/model"
	"lost-media-finder/internal/storage"

	"github.com/PuerkitoBio/goquery"
)

type Crawler struct {
	cfg     *config.Config
	redis   *storage.Redis
	mongo   *storage.Mongo
	client  *auth.Client
	targets []model.Video
	mu      sync.Mutex
	count   int
	queue   chan string
}

func New(cfg *config.Config, redis *storage.Redis, mongo *storage.Mongo) *Crawler {
	return &Crawler{
		cfg:    cfg,
		redis:  redis,
		mongo:  mongo,
		client: auth.NewClient(),
		queue:  make(chan string, cfg.BufferSize),
	}
}

func (c *Crawler) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func (c *Crawler) TargetCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.targets)
}

func (c *Crawler) worker() {
	for url := range c.queue {
		c.process(url)
	}
}

func (c *Crawler) process(url string) {
	ctx := context.Background()
	added, err := c.redis.TryAdd(ctx, c.cfg.RedisPrefix, url, c.cfg.RedisTTL)
	if err != nil || !added {
		return
	}

	time.Sleep(c.cfg.RateLimit)

	var resp *http.Response
	var doc *goquery.Document
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		start := time.Now()
		resp, err = c.client.Get(url)
		metrics.FetchDuration.Observe(time.Since(start).Seconds())

		if err == nil {
			doc, err = goquery.NewDocumentFromReader(resp.Body)
			resp.Body.Close()
			if err == nil {
				break // success
			}
		}

		if attempt < maxRetries {
			log.Printf("[WARN] Retry %d/%d for %s: %v", attempt, maxRetries, url, err)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if err != nil {
		metrics.Errors.Inc()
		log.Printf("[ERROR] Failed to fetch %s: %v", url, err)
		return
	}

	metrics.PagesProcessed.Inc()

	if strings.Contains(url, c.cfg.VideoPattern) {
		idx := strings.Index(url, c.cfg.VideoPattern)
		url = c.cfg.BaseUrl + url[idx:]

		c.mu.Lock()
		c.count++
		videoCount := c.count
		targetCount := len(c.targets)
		c.mu.Unlock()

		title := strings.TrimSuffix(doc.Find("title").Text(), c.cfg.TitleSuffix)
		dateStr := strings.TrimSpace(doc.Find("date").First().Text())

		v := model.Video{URL: url, Title: title, Date: dateStr}
		v.IsTarget = v.Match(c.cfg.CutoffDate)
		metrics.VideoFound.Inc()

		if v.IsTarget {
			c.mu.Lock()
			c.targets = append(c.targets, v)
			log.Printf("[FOUND] %d, %s - %s", len(c.targets), v.Title, v.URL)
			c.mu.Unlock()
			metrics.TargetsFound.Inc()
		}

		c.mongo.Upsert(ctx, v)
		// log.Printf("[%d] %s | %s | target=%v", n, title, dateStr, v.IsTarget)

		if videoCount%100 == 0 {
			log.Printf("[PROG] Processing video: %d videos, %d targets", videoCount, targetCount)
		}
	}

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
			href = c.cfg.BaseUrl + href
		}
		if strings.HasPrefix(href, c.cfg.BaseUrl) {
			select {
			case c.queue <- href:
			default:
			}
		}
	})
}

func (c *Crawler) Resume() {
	ctx := context.Background()
	videos, err := c.mongo.FindAll(ctx)
	if err != nil {
		log.Printf("[ERROR] Resume failed: %s", err)
		return
	}

	for _, v := range videos {
		c.redis.TryAdd(ctx, c.cfg.RedisPrefix, v.URL, c.cfg.RedisTTL)
		if v.IsTarget {
			c.targets = append(c.targets, v)
		}
	}

	c.count = len(videos)
	log.Printf("[INFO] Resume %d videos, %d targets", c.count, len(c.targets))
}

func (c *Crawler) Run(url string) {
	for i := 0; i < c.cfg.Workers; i++ {
		go c.worker()
	}

	c.queue <- url

	for {
		metrics.QueueSize.Set(float64(len(c.queue)))
		time.Sleep(c.cfg.RateLimit * 2)

		if len(c.queue) == 0 {
			time.Sleep(c.cfg.RateLimit * 2)
			if len(c.queue) == 0 {
				log.Println("[INFO] Crawler completed")
				close(c.queue)
				return
			}
		}

	}
}

func (c *Crawler) RunTest(url string) {
	for i := 0; i < c.cfg.Workers; i++ {
		go c.worker()
	}

	c.queue <- url

	for {
		time.Sleep(c.cfg.RateLimit)

		c.mu.Lock()
		count := c.count
		c.mu.Unlock()

		if count >= 10 {
			log.Println("[INFO] Test limit reached")
			close(c.queue)
			return
		}
	}
}

func (c *Crawler) Save() error {
	ctx := context.Background()
	targets, err := c.mongo.FindTargets(ctx)
	if err != nil {
		return err
	}

	f, err := os.Create(c.cfg.OutputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(targets)
}

func (c *Crawler) Clear() {
	ctx := context.Background()
	c.mongo.Drop(ctx)
	c.redis.FlushDB(ctx)
	log.Println("[INFO] Crawler cleared MongoDB and Redis")
}
