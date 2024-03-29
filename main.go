package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/accesscontrol"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
)

const (
	host         = ""
	port         = 2222
	bufHeight    = 14
	defaultFile  = "./data/short_intro.txt"
	starWarsFile = "./data/sw1.txt"
)

type tickerMsg struct{}

type model struct {
	file     *os.File
	scanner  *bufio.Scanner
	timer    *time.Timer
	viewport viewport.Model
}

var (
	intre = regexp.MustCompile(`^([\d]+)`)
	file  string
)

func main() {
	if len(os.Args) > 1 {
		file = os.Args[1]
	} else {
		file = defaultFile
	}

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
		wish.WithHostKeyPath(".ssh/ascii-ssh-movie_ed25519"),
		wish.WithMiddleware(
			bm.Middleware(teaHandler),
			lm.Middleware(),
			accesscontrol.Middleware(),
			activeterm.Middleware(),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Starting SSH server on %s:%d", host, port)
	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	<-done
	log.Println("Stopping SSH server")
	if err := s.Close(); err != nil {
		log.Fatalln(err)
	}
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, active := s.Pty()
	if !active {
		fmt.Println("no active terminal, skipping")
		return nil, nil
	}

	currentFile := file
	if s.User() == "starwars" {
		currentFile = starWarsFile
	}

	f, err := os.Open(currentFile)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(f)
	firstRawFrame, ok := getNextFrame(scanner)
	if !ok {
		log.Fatal("could not read initial frame")
	}
	firstFrame, timeout, err := getAndReplaceTimeFrame(firstRawFrame)
	if err != nil {
		log.Fatal(err)
	}

	m := model{
		viewport: viewport.Model{
			Width:  pty.Window.Width,
			Height: pty.Window.Height,
		},
		file:    f,
		scanner: scanner,
		timer:   time.NewTimer(time.Duration(timeout) * time.Second / 15),
	}
	m.viewport.SetContent(firstFrame)
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

func (m model) Init() tea.Cmd {
	return listenTimer(m.timer.C)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Height = msg.Height
		m.viewport.Width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.file.Close()
			return m, tea.Quit
		}
	case tickerMsg:
		nextRaw, ok := getNextFrame(m.scanner)
		if !ok {
			m.file.Close()
			return m, tea.Quit
		}
		nextFrame, timeout, _ := getAndReplaceTimeFrame(nextRaw)
		m.viewport.SetContent(nextFrame)
		m.timer.Reset(time.Duration(timeout) * time.Second / 15)
		return m, listenTimer(m.timer.C)

	}
	return m, nil
}

func (m model) View() string {
	return m.viewport.View()
}

func listenTimer(c <-chan time.Time) tea.Cmd {
	return func() tea.Msg {
		<-c
		return tickerMsg{}
	}
}

func getNextFrame(scanner *bufio.Scanner) (string, bool) {
	var s strings.Builder
	for i := 0; i < bufHeight; i++ {
		if !scanner.Scan() {
			return "", false
		}
		s.WriteString(scanner.Text())
		if i < bufHeight-1 {
			s.WriteString("\n")
		}
	}
	return s.String(), true
}

func getAndReplaceTimeFrame(x string) (string, int, error) {
	i, err := strconv.Atoi(intre.FindString(x))
	if err != nil {
		return "", 0, err
	}
	return intre.ReplaceAllString(x, " "), i, nil
}
