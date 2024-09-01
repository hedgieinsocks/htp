package cmd

import (
	"crypto/tls"
	"fmt"
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
	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
)

type options struct {
	intervalMs    int
	requestLimit  int
	tailLines     int
	httpMethod    string
	allowInsecure bool
}

type probe struct {
	id       int
	start    time.Time
	duration time.Duration
	end      time.Time
	status   int
	url      string
	err      error
}

type model struct {
	probes []probe
	width  int
	exit   bool
}

type probeMsg probe

var opts options

var rootCmd = &cobra.Command{
	Use:     "htp URL",
	Long:    "A tool to send HTTP probe requests at regular intervals",
	Version: "v0.0.3",
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
	rootCmd.Flags().IntVarP(&opts.tailLines, "tail", "t", 25, "number of requests to tail")
	rootCmd.Flags().StringVarP(&opts.httpMethod, "method", "m", "HEAD", "specify HTTP request method")
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

func renderOutput(m model, offset int) string {
	var output string
	for _, probe := range m.probes[offset:] {
		switch {
		case probe.err != nil:
			output += fmt.Sprintf("%d: start=%s, duration=%s, end=%s [%s] %v\n",
				probe.id,
				probe.start.Format("15:04:05.000"),
				probe.duration.Round(time.Millisecond),
				probe.end.Format("15:04:05.000"),
				color.RedString("ERROR"),
				probe.err,
			)
		case probe.status == 0:
			output += fmt.Sprintf("%d:\n", probe.id)
		default:
			output += fmt.Sprintf("%d: start=%s, duration=%s, end=%s [%s] %s\n",
				probe.id,
				probe.start.Format("15:04:05.000"),
				probe.duration.Round(time.Millisecond),
				probe.end.Format("15:04:05.000"),
				colorStatusCode(probe.status),
				color.BlackString(probe.url),
			)
		}
	}
	return output
}

func probeUrl(c *http.Client, id int, target *url.URL) tea.Msg {
	req, err := http.NewRequest(opts.httpMethod, target.String(), http.NoBody)
	if err != nil {
		log.Fatal(err)
	}
	start := time.Now()
	resp, err := c.Do(req)
	duration := time.Since(start)
	end := start.Add(duration)
	if err != nil {
		return probeMsg{
			id:       id,
			start:    start,
			duration: duration,
			end:      end,
			err:      err,
		}
	}
	defer resp.Body.Close()
	return probeMsg{
		id:       id,
		start:    start,
		duration: duration,
		end:      end,
		status:   resp.StatusCode,
		url:      resp.Request.URL.String(),
	}
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
	offset := len(m.probes) - opts.tailLines
	if offset < 0 {
		offset = 0
	}
	output := renderOutput(m, offset)
	return wordwrap.String(output, m.width)
}

func main(cmd *cobra.Command, args []string) {
	target, err := url.ParseRequestURI(args[0])
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
				p.Send(probeUrl(c, id, target))
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
