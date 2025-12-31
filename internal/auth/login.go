package auth

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type Client struct {
	*http.Client
}

func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		Client: &http.Client{Jar: jar},
	}
}

func (c *Client) Login(loginURL, username, password string) error {
	resp, err := c.PostForm(loginURL, url.Values{
		"username": {username},
		"password": {password},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login error: %s", resp.Status)
	}
	return nil
}
