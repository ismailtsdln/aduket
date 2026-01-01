package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ismailtsdln/aduket"
)

var (
	purple = lipgloss.Color("#7D56F4")
	white  = lipgloss.Color("#FAFAFA")
	dark   = lipgloss.Color("#1A1B26")
	gray   = lipgloss.Color("#3C3C3C")
	accent = lipgloss.Color("#00D7FF")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(purple).
			Padding(0, 1).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true).
			Padding(0, 1)

	docStyle = lipgloss.NewStyle().Margin(1, 2)

	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(purple).
				Border(lipgloss.ThickBorder(), false, false, true, false).
				MarginBottom(1)

	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A9B1D6")).
			Padding(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7AA2F7")).
			Italic(true)
)

type Config struct {
	Expectations []struct {
		Method   string            `json:"method"`
		Path     string            `json:"path"`
		Status   int               `json:"status"`
		Response string            `json:"response"`
		Headers  map[string]string `json:"headers"`
	} `json:"expectations"`
}

type item struct {
	method       string
	path         string
	status       int
	headers      http.Header
	requestBody  string
	responseBody string
}

func (i item) Title() string {
	methodColor := "#FAFAFA"
	switch i.method {
	case "GET":
		methodColor = "#9ECE6A"
	case "POST":
		methodColor = "#7AA2F7"
	case "PUT":
		methodColor = "#E0AF68"
	case "DELETE":
		methodColor = "#F7768E"
	}
	methodStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(methodColor)).Bold(true).Width(7)
	return fmt.Sprintf("%s %s", methodStyle.Render(i.method), i.path)
}

func (i item) Description() string {
	statusColor := "#00FF00"
	if i.status >= 400 {
		statusColor = "#FF0000"
	} else if i.status >= 300 {
		statusColor = "#FFFF00"
	}
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)
	return fmt.Sprintf("Status: %s | Headers: %d", statusStyle.Render(fmt.Sprintf("%d", i.status)), len(i.headers))
}
func (i item) FilterValue() string { return i.path }

type model struct {
	list         list.Model
	viewport     viewport.Model
	server       *aduket.Server
	selectedItem *item
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.server.Close()
			return m, tea.Quit
		case "enter", " ":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.selectedItem = &i
				detail := fmt.Sprintf("Path: %s\nStatus: %d\n\nHeaders:\n", i.path, i.status)
				for k, v := range i.headers {
					detail += fmt.Sprintf("  %s: %s\n", k, strings.Join(v, ", "))
				}
				detail += "\nRequest Body:\n"
				if i.requestBody != "" {
					detail += i.requestBody
				} else {
					detail += "[empty]"
				}
				detail += "\n\nResponse Body:\n"
				if i.responseBody != "" {
					detail += i.responseBody
				} else {
					detail += "[empty]"
				}
				m.viewport.SetContent(bodyStyle.Render(detail))
			}
		}
	case *aduket.CapturedRequest:
		i := item{
			method:       msg.Method,
			path:         msg.URL.Path,
			status:       msg.StatusCode,
			headers:      msg.Header,
			requestBody:  string(msg.BodyContent),
			responseBody: string(msg.ResponseBody),
		}
		return m, m.list.InsertItem(0, i)
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width/2-h, msg.Height-v-6)
		m.viewport.Width = msg.Width/2 - h
		m.viewport.Height = msg.Height - v - 6
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	sideBar := m.list.View()
	detailView := ""
	if m.selectedItem != nil {
		detailView = lipgloss.JoinVertical(lipgloss.Left,
			detailTitleStyle.Width(m.viewport.Width).Render(fmt.Sprintf("%s %s", m.selectedItem.method, m.selectedItem.path)),
			m.viewport.View(),
		)
	} else {
		detailView = lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(gray).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Select a request to see details")
	}

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top,
		sideBar,
		lipgloss.NewStyle().PaddingLeft(2).Render(detailView),
	)

	banner := titleStyle.Render(" ADUKET ")
	urlInfo := headerStyle.Render(fmt.Sprintf("Mock Server: %s", m.server.URL))
	helpInfo := statusStyle.Render(" [q: quit] [enter: inspect] [/: search] ")

	return docStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Center, banner, urlInfo),
			"",
			mainContent,
			"",
			helpInfo,
		),
	)
}

func main() {
	port := flag.Int("port", 8080, "port to run the mock server on")
	configFile := flag.String("config", "", "path to json config file")
	flag.Parse()

	s := aduket.NewUnstartedServer()
	addr := fmt.Sprintf(":%d", *port)
	if err := s.Listen(addr); err != nil {
		fmt.Printf("Error starting server on %s: %v\n", addr, err)
		os.Exit(1)
	}

	if *configFile != "" {
		data, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Printf("Error reading config: %v\n", err)
			os.Exit(1)
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("Error parsing config: %v\n", err)
			os.Exit(1)
		}
		for _, exp := range cfg.Expectations {
			s.Expect(exp.Method, exp.Path).
				Response(exp.Status, exp.Response).
				Headers(exp.Headers)
		}
	} else {
		s.Expect("GET", "/").Response(200, "{\"message\": \"Aduket CLI is running!\"}")
	}

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Traffic"
	l.SetShowHelp(false)
	l.Styles.Title = lipgloss.NewStyle().Foreground(purple).Bold(true)

	vp := viewport.New(0, 0)

	m := model{
		list:     l,
		viewport: vp,
		server:   s,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	s.OnRequest = func(req *aduket.CapturedRequest) {
		p.Send(req)
	}

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
