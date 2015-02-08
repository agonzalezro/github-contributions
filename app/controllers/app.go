package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/revel/revel"
)

type App struct {
	*revel.Controller
}

type Contributions []Contribution

type Contribution struct {
	Type string `json:"type"`
	Repo struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repo"`
	Payload struct {
		Action      string `json:"action"`
		PullRequest struct {
			HTMLURL   string `json:"html_url"`
			Title     string `json:"title"`
			Body      string `json:"body"`
			CreatedAt string `json:"created_at"`
		} `json:"pull_request"`
	} `json:"payload"`
}

func (cs Contributions) Len() int {
	return len(cs)
}

func (cs Contributions) Less(i, j int) bool {
	layout := "2006-01-02T15:04:05Z"

	iDate, err := time.Parse(layout, cs[i].Payload.PullRequest.CreatedAt)
	if err != nil {
		iDate = time.Now()
	}

	jDate, err := time.Parse(layout, cs[j].Payload.PullRequest.CreatedAt)
	if err != nil {
		jDate = time.Now()
	}

	return iDate.After(jDate)
}

func (cs Contributions) Swap(i, j int) {
	cs[i], cs[j] = cs[j], cs[i]
}

func getUrl(username string, page int) string {
	url := fmt.Sprintf("https://api.github.com/users/%s/events", username)
	if page > 1 {
		url += fmt.Sprintf("?page=%d", page)
	}
	return url
}

func fetchContributions(username string, cChan chan Contribution) error {
	var wg sync.WaitGroup
	wg.Add(5)

	// We need to default to something if we don't want to do an initial
	// request to get the number of available pages (in the Link Header).
	//
	// Anyway, after several tests, it looks like it's always 5.
	numberOfPages := 5

	for i := 1; i <= numberOfPages; i++ {
		go func(page int) error {
			url := getUrl(username, page)

			resp, err := http.Get(url)
			if err != nil {
				return err
			}
			if resp.StatusCode != 200 {
				return nil
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			cs := Contributions{}
			err = json.Unmarshal(body, &cs)
			if err != nil {
				return err
			}

			isNewPR := func(c Contribution) bool {
				return c.Type == "PullRequestEvent" && c.Payload.Action == "opened"
			}
			for _, c := range cs {
				if isNewPR(c) {
					log.Println(c)
					cChan <- c
				}
			}

			wg.Done()
			return nil
		}(i)
	}

	wg.Wait()

	close(cChan)
	return nil
}

func (a App) Show(username string) revel.Result {
	var contributions Contributions
	a.Params.Bind(&username, "username")

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

	sort.Sort(contributions)
	return a.RenderJson(contributions)
}

func (a App) Index() revel.Result {
	return a.Render()
}
