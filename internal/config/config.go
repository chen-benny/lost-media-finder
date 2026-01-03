package config

import "time"

type Config struct {
	// MaxVideos    int
	Workers      int
	BufferSize   int
	BaseUrl      string
	TestUrl      string
	VideoPattern string
	TitleSuffix  string
	OutputFile   string
	RateLimit    time.Duration
	CutoffDate   time.Time

	RedisAddr   string
	RedisPrefix string
	RedisTTL    time.Duration

	MongoURI string
	MongoDB  string
	MongoCol string

	LoginURL string
	Username string
	Password string

	MetricsPort string
}

func Load() *Config {
	return &Config{
		// MaxVideos:    100,
		Workers:      4,
		BufferSize:   10000,
		BaseUrl:      "https://www.vidlii.com",
		TestUrl:      "https://www.vidlii.com/user/rinkomania",
		VideoPattern: "/watch?v=",
		TitleSuffix:  " - VidLii",
		OutputFile:   "targets.json",
		RateLimit:    2 * time.Second,
		CutoffDate:   time.Date(2021, 12, 31, 23, 59, 59, 0, time.UTC),

		RedisAddr:   "localhost:6379",
		RedisPrefix: "vidlii:",
		RedisTTL:    24 * time.Hour,

		MongoURI: "mongodb://localhost:27017",
		MongoDB:  "vidlii",
		MongoCol: "videos",

		LoginURL: "https://www.vidlii.com/login",
		Username: "bennyc",
		Password: "abc123456",

		MetricsPort: "2112",
	}
}
