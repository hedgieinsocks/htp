package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/itchyny/gojq"
	"github.com/muesli/reflow/wrap"
	"github.com/spf13/cobra"
)

type options struct {
	intervalMs    int
	requestLimit  int
	pagerLines    int
	httpMethod    string
	jsonFilter    string
	allowInsecure bool
}

type probe struct {
	id       int
	start    string
	duration string
	end      string
	status   string
	url      string
	json     string
	err      string
}

type model struct {
	probes []probe
	width  int
	exit   bool
}

type probeMsg probe

var opts options

var timeFmt = "15:04:05.000"

var rootCmd = &cobra.Command{
	Use:     "htp URL",
	Long:    "A tool to send HTTP probe requests at regular intervals",
	Version: "v0.0.4",
	Args:    cobra.ExactArgs(1),
	Run:     main,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("Error: ")
	rootCmd.Flags().IntVarP(&opts.intervalMs, "interval", "i", 1000, "interval between requests in milliseconds")
	rootCmd.Flags().IntVarP(&opts.requestLimit, "limit", "l", 0, "number of requests to make (default unlimited)")
	rootCmd.Flags().IntVarP(&opts.pagerLines, "pager", "p", 25, "number of requests to pager")
	rootCmd.Flags().StringVarP(&opts.httpMethod, "method", "m", "GET", "specify HTTP request method")
	rootCmd.Flags().StringVarP(&opts.jsonFilter, "json", "j", "", "jq-compatible filter for JSON response")
	rootCmd.Flags().BoolVarP(&opts.allowInsecure, "insecure", "k", false, "allow insecure connections")
	rootCmd.Flags().SortFlags = false
}

func colorStatusCode(code int) string {
	stringCode := strconv.Itoa(code)
	switch {
	case strings.HasPrefix(stringCode, "2"):
		return color.GreenString(stringCode)
	case strings.HasPrefix(stringCode, "4"):
		return color.YellowString(stringCode)
	case strings.HasPrefix(stringCode, "5"):
		return color.RedString(stringCode)
	default:
		return stringCode
	}
}

func filterJson(resp *http.Response) string {
	if opts.jsonFilter == "" {
		return ""
	}

	mime := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(mime, "application/json") {
		return color.RedString(fmt.Sprintf("Invalid content type: %s", mime))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return color.RedString(err.Error())
	}

	query, err := gojq.Parse(opts.jsonFilter)
	if err != nil {
		return color.RedString(err.Error())
	}

	var input interface{}
	if err := json.Unmarshal(body, &input); err != nil {
		return color.RedString(err.Error())
	}

	iter := query.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if err, ok := err.(*gojq.HaltError); ok && err.Value() == nil {
				break
			}
			return color.RedString(err.Error())
		}
		jsonData, err := json.Marshal(v)
		if err != nil {
			return color.RedString(err.Error())
		}
		return fmt.Sprintf("=> %s", color.CyanString(string(jsonData)))
	}
	return ""
}

func probeUrl(c *http.Client, req *http.Request, id int) tea.Msg {
	start := time.Now()
	resp, err := c.Do(req)
	duration := time.Since(start)
	end := start.Add(duration)
	if err != nil {
		return probeMsg{
			id:       id,
			start:    start.Format(timeFmt),
			duration: duration.Round(time.Millisecond).String(),
			end:      end.Format(timeFmt),
			err:      color.RedString(err.Error()),
		}
	}
	defer resp.Body.Close()
	return probeMsg{
		id:       id,
		start:    start.Format(timeFmt),
		duration: duration.Round(time.Millisecond).String(),
		end:      end.Format(timeFmt),
		status:   colorStatusCode(resp.StatusCode),
		url:      resp.Request.URL.String(),
		json:     filterJson(resp),
	}
}

func renderOutput(m model, offset int) string {
	var output string
	for _, probe := range m.probes[offset:] {
		switch {
		case probe.err != "":
			output += fmt.Sprintf("%d: start=%s, duration=%s, end=%s %s\n",
				probe.id,
				probe.start,
				probe.duration,
				probe.end,
				probe.err,
			)
		case probe.status == "":
			output += fmt.Sprintf("%d:\n", probe.id)
		default:
			output += fmt.Sprintf("%d: start=%s, duration=%s, end=%s, url=%s [%s] %s\n",
				probe.id,
				probe.start,
				probe.duration,
				probe.end,
				probe.url,
				probe.status,
				probe.json,
			)
		}
	}
	return output
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			m.exit = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case probeMsg:
		if i := slices.IndexFunc(m.probes, func(r probe) bool { return r.id == msg.id }); i >= 0 {
			m.probes[i] = probe(msg)
		} else {
			m.probes = append(m.probes, probe(msg))
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.exit {
		return ""
	}
	offset := len(m.probes) - opts.pagerLines
	if offset < 0 {
		offset = 0
	}
	output := renderOutput(m, offset)
	return wrap.String(output, m.width)
}

func main(cmd *cobra.Command, args []string) {
	target, err := url.ParseRequestURI(args[0])
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(opts.httpMethod, target.String(), http.NoBody)
	if err != nil {
		log.Fatal(err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.allowInsecure},
	}
	c := &http.Client{Transport: tr}
	p := tea.NewProgram(model{})
	t := time.NewTicker(time.Duration(opts.intervalMs) * time.Millisecond)

	fmt.Printf("%s %s [%dms]\n\n", opts.httpMethod, target, opts.intervalMs)

	go func() {
		var wg sync.WaitGroup
		defer t.Stop()
		id := 1
		for {
			if opts.requestLimit != 0 && id > opts.requestLimit {
				break
			}
			p.Send(probeMsg{id: id})
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				p.Send(probeUrl(c, req, id))
			}(id)
			id++
			<-t.C
		}
		wg.Wait()
		p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	}()

	result, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	resultModel, ok := result.(model)
	if !ok {
		log.Fatal(err)
	}

	fmt.Printf(renderOutput(resultModel, 0))
}
