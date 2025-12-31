package main

import (
	"fmt"
	"log"

	"lost-media-finder/internal/config"
	"lost-media-finder/internal/crawler"
	"lost-media-finder/internal/metrics"
	"lost-media-finder/internal/storage"
)

func main() {
	cfg := config.Load()

	redis, err := storage.NewRedis(cfg.RedisAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer redis.Close()

	mongo, err := storage.NewMongo(cfg.MongoURI, cfg.MongoDB, cfg.MongoCol)
	if err != nil {
		log.Fatal(err)
	}
	defer mongo.Close()

	go metrics.Serve(cfg.MetricsPort)

	c := crawler.New(cfg, redis, mongo)
	c.Clear()
	c.Resume()

	fmt.Println("Crawler started...")
	c.Run(cfg.TestUrl)

	if err := c.Save(); err != nil {
		log.Printf("Error saving: %s", err)
	}

	fmt.Printf("Visited %d videos, targets %d\n", c.Count(), c.TargetCount())
}
