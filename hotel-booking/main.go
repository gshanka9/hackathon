package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/google/go-github/v41/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Booking struct {
	ID       int    `json:"id"`
	Customer string `json:"customer"`
	Room     int    `json:"room"`
}

var (
	bookings []Booking
	log      = logrus.New()
)

func main() {
	// Set up logging to file and console
	log.SetFormatter(&logrus.JSONFormatter{})
	file, err := os.OpenFile("runtime_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()
	log.SetOutput(file)

	http.HandleFunc("/book", bookingHandler)
	http.HandleFunc("/author", authorHandler)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	log.Info("Starting server on :8081")
	err = http.ListenAndServe(":8081", nil)
	if err != nil {
		logError(err)
	}
}

func bookingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method hello", http.StatusMethodNotAllowed)
		return
	}

	var b Booking
	err := json.NewDecoder(r.Body).Decode(&b)
	logError(err)
	if err != nil {
		logError(err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	b.ID = len(bookings) + 1
	bookings = append(bookings, b)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

func authorHandler(w http.ResponseWriter, r *http.Request) {
	author, err := getAuthorFromGitHub("your-repo-owner", "your-repo-name", "main.go", 42)
	if err != nil {
		logError(err)
		http.Error(w, "Failed to get author", http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, author)
}

func logError(err error) {
	stack := debug.Stack()
	_, file, line, _ := runtime.Caller(1)
	log.WithFields(logrus.Fields{
		"error": err,
		"stack": string(stack),
		"file":  file,
		"line":  line,
	}).Error("An error occurred")
}

func getAuthorFromGitHub(repoOwner, repoName, filename string, line int) (string, error) {
	ctx := context.Background()
	token := os.Getenv("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")

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	opts := github.ListOptions{PerPage: 100}
	commits, _, err := client.Repositories.ListCommits(ctx, repoOwner, repoName, &github.CommitsListOptions{
		Path:        filename,
		ListOptions: opts,
	})
	if err != nil {
		return "", err
	}

	for _, commit := range commits {
		files, _, err := client.Repositories.GetCommit(ctx, repoOwner, repoName, *commit.SHA, &opts)
		if err != nil {
			return "", err
		}
		for _, file := range files.Files {
			if *file.Filename == filename && isLineInRange(line, file.Patch) {
				return *commit.Author.Login, nil
			}
		}
	}

	return "", fmt.Errorf("No commit found for file %s at line %d", filename, line)
}

func isLineInRange(line int, patch *string) bool {
	if patch == nil {
		return false
	}
	lines := strings.Split(*patch, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "@@") {
			var start, count int
			fmt.Sscanf(l, "@@ -%d,%d +%d,%d @@", &start, &count, &start, &count)
			if line >= start && line <= start+count {
				return true
			}
		}
	}
	return false
}
