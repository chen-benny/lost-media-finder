package model

import (
	"time"
	"unicode"
)

type Video struct {
	URL      string `json:"url" bson:"_id"`
	Title    string `json:"title" bson:"title"`
	Date     string `json:"date" bson:"date"`
	IsTarget bool   `json:"is_target" bson:"is_target"`
}

func (v Video) Match(cutoff time.Time) bool {
	date, _ := time.Parse("Jan 2, 2006", v.Date)
	if date.IsZero() || !date.Before(cutoff) {
		return false
	}

	for _, c := range v.Title {
		if unicode.In(c, unicode.Hiragana, unicode.Katakana, unicode.Han) {
			return true
		}
	}
	return false
}
