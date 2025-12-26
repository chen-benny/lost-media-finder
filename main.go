package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	maxVideos    = 100
	workers      = 4
	bufferSize   = 10000
	baseUrl      = "https://www.vidlii.com"
	testUrl      = "https://www.vidlii.com/user/rinkomania"
	videoPattern = "/watch?v="
	titleSuffix  = " - VidLii"
	outputFile   = "targets.json"

	redisAddr   = "localhost:6379"
	redisPrefix = "vidlii:"
	redisTTL    = 24 * time.Hour

	mongoURI = "mongodb://localhost:27017"
	mongoDB  = "vidlii"
	mongoCol = "videos"
)

// the reported date is before 2022
var (
	cutoffDate = time.Date(2021, 12, 31, 23, 59, 59, 0, time.UTC)
)

type Video struct {
	URL      string `json:"url" bson:"_id"`
	Title    string `json:"title" bson:"title"`
	Date     string `json:"date" bson:"date"`
	IsTarget bool   `json:"is_target" bson:"is_target"`
}

// Match selects video that has Japanese chars in title and date before cutoff
func (v Video) Match() bool {
	date, _ := time.Parse("Jan 2, 2006", v.Date)
	if date.IsZero() || !date.Before(cutoffDate) {
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
	targets []Video
	mu      sync.Mutex
	count   int
	queue   chan string
	ticker  *time.Ticker

	redis *redis.Client
	mongo *mongo.Collection
}

func NewCrawler() *Crawler {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("MongoDB connection failed:", err)
	}

	col := client.Database(mongoDB).Collection(mongoCol)
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "date", Value: 1}},
	})

	return &Crawler{
		redis:  rdb,
		mongo:  col,
		queue:  make(chan string, bufferSize),
		ticker: time.NewTicker(time.Millisecond * 500),
	}
}

func (c *Crawler) done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count >= maxVideos
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

	ctx := context.Background()
	added, err := c.redis.SetNX(ctx, redisPrefix+url, 1, redisTTL).Result() // Redis SETNX with 24h TTL
	if err != nil || !added {
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

		v := Video{URL: url, Title: title, Date: dateStr, IsTarget: false}

		if v.Match() {
			v.IsTarget = true
			c.mu.Lock()
			c.targets = append(c.targets, v)
			c.mu.Unlock()
			log.Printf("Found one target: %s", v.Title)
		}

		ctx := context.Background()
		c.mongo.ReplaceOne(ctx, bson.M{"_id": v.URL}, v, options.Replace().SetUpsert(true))

		log.Printf("[%d] %s | %s | target=%v", n, title, dateStr, v.IsTarget)
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

func (c *Crawler) Resume() {
	ctx := context.Background()
	cursor, err := c.mongo.Find(ctx, bson.M{})
	if err != nil {
		log.Printf("Resume failed: %s", err)
		return
	}
	defer cursor.Close(ctx)

	var videos []Video
	if err := cursor.All(ctx, &videos); err != nil {
		log.Printf("Resume failed: %s", err)
		return
	}

	for _, v := range videos {
		c.redis.SetNX(ctx, redisPrefix+v.URL, 1, redisTTL)
		if v.Match() {
			c.targets = append(c.targets, v)
		}
	}
	c.count = len(videos)
	log.Printf("Resumed %d videos, %d targets", c.count, len(c.targets))
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

func (c *Crawler) Close() {
	c.redis.Close()
	c.mongo.Database().Client().Disconnect(context.Background())
}

func (c *Crawler) Save() error {
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(c.targets)
}

func (c *Crawler) Clear() {
	ctx := context.Background()
	c.mongo.Drop(ctx)
	c.redis.FlushDB(ctx)
	log.Println("Cleared MongoDB and Redis")
}

func main() {
	c := NewCrawler()
	defer c.ticker.Stop()
	defer c.Close()

	c.Clear() // uncomment in production
	c.Resume()
	c.Run()

	if err := c.Save(); err != nil {
		log.Printf("Error saving: %s", err)
	}

	fmt.Printf("visited %d videos targets found: %d\n", c.count, len(c.targets))
	fmt.Printf("Save to %s\n", outputFile)
}
