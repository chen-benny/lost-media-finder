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
	"golang.org/x/time/rate"
)

type Crawler struct {
	cfg      *config.Config
	redis    *storage.Redis
	mongo    *storage.Mongo
	client   *auth.Client
	targets  []model.Video
	mu       sync.Mutex
	count    int
	queue    chan string
	wg       sync.WaitGroup
	maxCount int
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
	limiter := rate.NewLimiter(rate.Every(c.cfg.RateLimit), 1)
	for url := range c.queue {
		if c.maxCount <= 0 || c.Count() < c.maxCount {
			limiter.Wait(context.Background())
			c.process(url)
		}
		c.wg.Done()
	}
}

func (c *Crawler) enqueue(url string) {
	c.wg.Add(1)
	select {
	case c.queue <- url:
	default:
		if err := c.redis.PushOverflow(context.Background(), url); err != nil {
			c.wg.Done()
		}
	}
}

func (c *Crawler) drainOverFlow(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		url, err := c.redis.PopOverflow(ctx)
		if err != nil || url == "" {
			time.Sleep(time.Second)
			continue
		}

		select {
		case c.queue <- url:
		case <-ctx.Done():
			return
		}
	}
}

func (c *Crawler) process(url string) {
	if c.maxCount > 0 { // for RunTest
		c.mu.Lock()
		if c.count >= c.maxCount {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
	ctx := context.Background()
	added, err := c.redis.TryAdd(ctx, c.cfg.RedisPrefix, url, c.cfg.RedisTTL)
	if err != nil || !added {
		return
	}

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

		if videoCount%1000 == 0 {
			log.Printf("[PROG] Processing video: %d videos, %d targets", videoCount, targetCount)
		}
	}

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
			href = c.cfg.BaseUrl + href
		}
		if strings.HasPrefix(href, c.cfg.BaseUrl) {
			c.enqueue(href)
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
	ctx, cancel := context.WithCancel(context.Background())

	go c.drainOverFlow(ctx)

	for i := 0; i < c.cfg.Workers; i++ {
		go c.worker()
	}
	c.enqueue(url)
	c.wg.Wait()
	cancel() // stop drain first
	close(c.queue)
	log.Printf("[INFO] Crawler Finished")
}

func (c *Crawler) RunTest(url string) {
	ctx, cancel := context.WithCancel(context.Background())

	c.maxCount = c.cfg.MaxVideos
	go c.drainOverFlow(ctx)

	for i := 0; i < c.cfg.Workers; i++ {
		go c.worker()
	}

	c.enqueue(url)
	c.wg.Wait()
	cancel()
	close(c.queue)
	log.Printf("[INFO] Crawler Finished Test")
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
