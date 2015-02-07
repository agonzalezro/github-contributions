package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/revel/revel"
)

type App struct {
	*revel.Controller
}

type Contributions []Contribution

type Contribution struct {
	Type    string
	Payload struct {
		Commits []struct {
			Message string
		}
	}
}

func fetchContributions(username string, cChan chan Contribution) error {
	url := fmt.Sprintf("https://api.github.com/users/%s/events", username)

	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "github-collaborations")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	cs := Contributions{}
	err = json.Unmarshal(body, &cs)
	if err != nil {
		panic(string(body))
		return err
	}

	for _, c := range cs {
		if c.Type == "PushEvent" {
			cChan <- c
		}
	}

	close(cChan)
	return nil
}

func (a App) Index() revel.Result {
	var (
		username      string
		contributions []Contribution
	)
	a.Params.Bind(&username, "username")

	if username != "" {
		cChan := make(chan Contribution, 1)
		go func() {
			err := fetchContributions(username, cChan)
			if err != nil {
				panic(err)
			}
		}()

		for c := range cChan {
			contributions = append(contributions, c)
		}
	}

	return a.Render(contributions)
}
