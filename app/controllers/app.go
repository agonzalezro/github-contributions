package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/revel/revel"
	"github.com/russross/blackfriday"
)

type App struct {
	*revel.Controller
}

type SearchResults struct {
	TotalCount int `json:"total_count"`

	Items []SearchResultsItem
}

type SearchResultsItem struct {
	Title     string
	Body      string
	State     string
	CreatedAt string `json:"created_at"`

	PullRequest struct {
		HTMLURL string `json:"html_url"`
	} `json:"pull_request"`
}

type Contributions []Contribution

type Contribution struct {
	URL     string `json:"url"`
	Project string `json:"project"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Date    string `json:"date"`
	Closed  bool   `json:"closed"`
}

func NewContributionFromSearchResponse(i SearchResultsItem) Contribution {
	url := i.PullRequest.HTMLURL

	projectName := func(url string) string {
		re := regexp.MustCompile("github.com/(.+)/pull/")

		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}(url)

	body := string(
		blackfriday.MarkdownBasic([]byte(i.Body)))

	return Contribution{
		URL:     url,
		Project: projectName,
		Title:   i.Title,
		Body:    body,
		Date:    i.CreatedAt,
		Closed:  i.State == "closed",
	}
}

func (cs Contributions) Len() int {
	return len(cs)
}

func (cs Contributions) Less(i, j int) bool {
	layout := "2006-01-02T15:04:05Z"

	iDate, err := time.Parse(layout, cs[i].Date)
	if err != nil {
		iDate = time.Now()
	}

	jDate, err := time.Parse(layout, cs[j].Date)
	if err != nil {
		jDate = time.Now()
	}

	return iDate.After(jDate)
}

func (cs Contributions) Swap(i, j int) {
	cs[i], cs[j] = cs[j], cs[i]
}

func getUrl(username string, page int) string {
	values := url.Values{}
	for k, v := range map[string]string{
		"q":        fmt.Sprintf("author:%s type:pr", username),
		"per_page": "100",
		"page":     string(page),
	} {
		values.Set(k, v)
	}

	return fmt.Sprintf("https://api.github.com/search/issues?%s", values.Encode())
}

func getNumberOfPages(username string) (int, error) {
	url := getUrl(username, 1)

	client := &http.Client{}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return -1, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	link := resp.Header.Get("Link")
	if link != "" {
		re := regexp.MustCompile(`page=(\d+).*>; rel="last"`)

		matches := re.FindStringSubmatch(link)
		if len(matches) > 1 {
			pages, err := strconv.ParseInt(matches[1], 0, 0)
			return int(pages), err
		}
	}
	return 1, nil // No error, but no pages either
}

func fetchContributions(username string, cChan chan Contribution) error {
	var wg sync.WaitGroup

	numberOfPages, err := getNumberOfPages(username)
	if err != nil {
		return err
	}
	// TODO: for testing purposes, and perhaps for later we are going to limit this
	if numberOfPages > 2 {
		numberOfPages = 2
	}

	wg.Add(numberOfPages)

	for page := 1; page <= numberOfPages; page++ {
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

			sr := SearchResults{}
			err = json.Unmarshal(body, &sr)
			if err != nil {
				return err
			}

			for _, i := range sr.Items {
				cChan <- NewContributionFromSearchResponse(i)
			}

			wg.Done()
			return nil
		}(page)
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
