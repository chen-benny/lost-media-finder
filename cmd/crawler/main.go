package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"lost-media-finder/internal/config"
	"lost-media-finder/internal/crawler"
	"lost-media-finder/internal/metrics"
	"lost-media-finder/internal/storage"
)

func main() {
	testMode := flag.Bool("test", false, "run tests with test url and max video limit")
	flag.Parse()

	cfg := config.Load()

	logFile, err := os.OpenFile("crawler.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

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
	if *testMode {
		log.Println("[INFO] Running in test mode")
		c.Clear()
		c.RunTest(cfg.TestUrl)
	} else {
		log.Println("[INFO] Running in production mode")
		c.Resume()
		c.Run(cfg.BaseUrl)
	}

	if err := c.Save(); err != nil {
		log.Printf("Error saving: %s", err)
	}

	result := fmt.Sprintf("Visited %d videos, targets %d\n", c.Count(), c.TargetCount())
	log.Println(result)
	fmt.Printf(result)
}
