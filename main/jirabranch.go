package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/fatih/color"
)

var branchMatcher = regexp.MustCompile("\\s*\\*?\\s*(\\w+-\\d+).*")

type jiraSettings struct {
	rootUrl string
	user    string
	pw      string
}

var (
	yellow  = color.New(color.FgYellow).SprintfFunc()
	red     = color.New(color.FgRed).SprintfFunc()
	green   = color.New(color.FgGreen).SprintfFunc()
	blue    = color.New(color.FgBlue).SprintfFunc()
	magenta = color.New(color.FgMagenta).SprintfFunc()
	cyan    = color.New(color.FgCyan).SprintfFunc()
	white   = color.New(color.FgWhite).SprintfFunc()
)

func main() {

	js := jiraSettings{}
	noColor, noLinks := false, false
	flag.StringVar(&(js.rootUrl), "url", "https://jira.mongodb.org/", "root URL of the jira server") // where issues are found via JSON rest api")
	flag.BoolVar(&noColor, "no-color", false, "disable colors in output")
	flag.BoolVar(&noColor, "no-links", false, "hide URLs in output")
	flag.Parse()

	if noColor {
		color.NoColor = true
	}

	js.rootUrl = strings.TrimRight(js.rootUrl, "/")

	in := bufio.NewReader(os.Stdin)
	wg := sync.WaitGroup{}
	for {
		line, err := in.ReadString(byte('\n'))

		if err != nil && err != io.EOF {
			fmt.Println("error reading input: ", err)
			os.Exit(1)
		}

		wg.Add(1)
		go func() {
			ticketInfo := getBranchDescription(line, js, !noLinks)
			fmt.Println(ticketInfo)
			wg.Done()
		}()

		if err == io.EOF {
			break
		}
	}
	wg.Wait()
}

type TicketInfo struct {
	URL    string
	Fields struct {
		Created    string `json:"created"`
		Resolution struct {
			Name string `json:"name"`
		} `json:"resolution"`
		Resolutiondate string `json:"resolutiondate"`
		Status         struct {
			Name string `json:"name"`
		} `json:"status"`
		Summary string `json:"summary"`
		Updated string `json:"updated"`
		Watches struct {
			IsWatching bool   `json:"isWatching"`
			Self       string `json:"self"`
			WatchCount int    `json:"watchCount"`
		} `json:"watches"`
		Workratio int `json:"workratio"`
	} `json:"fields"`
	Key string `json:"key"`
}

func (ti TicketInfo) Description(includeUrl bool) string {
	state := ti.Fields.Status.Name
	stateColor := red
	if len(ti.Fields.Resolutiondate) > 0 {
		stateColor = green
		state += ": " + ti.Fields.Resolution.Name
	}

	if !includeUrl {
		ti.URL = ""
	}

	out := fmt.Sprintf("%s (%s) %s %s", ti.Key, stateColor(state), ti.Fields.Summary, cyan(ti.URL))
	return strings.TrimSpace(out)
}

func getBranchDescription(line string, js jiraSettings, includeUrls bool) string {
	line = strings.TrimSpace(line)
	matches := branchMatcher.FindStringSubmatch(line)
	if len(matches) < 2 {
		return line
	}

	// It looks like a ticket name, so let's find it

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/rest/api/latest/issue/%s", js.rootUrl, matches[1]), nil)
	if err != nil {
		fmt.Println(err)
		return line
	}
	if len(js.user) > 0 && len(js.pw) > 0 {
		req.SetBasicAuth(js.user, js.pw)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return line
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return line
	}
	ticket := TicketInfo{}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return line
	}
	err = json.Unmarshal(body, &ticket)
	if err != nil {
		fmt.Println(err)
		return line
	}

	ticket.URL = fmt.Sprintf("%s/browse/%s", js.rootUrl, ticket.Key)

	return ticket.Description(includeUrls)
}
