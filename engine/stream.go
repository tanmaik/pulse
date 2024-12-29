package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// FullChange represents the entire edit event structure from Wikimedia EventStream
type FullChange struct {
	Schema string `json:"$schema"`
	Meta   struct {
		URI       string `json:"uri"`
		RequestID string `json:"request_id"`
		ID        string `json:"id"`
		DT        string `json:"dt"`
		Domain    string `json:"domain"`
		Stream    string `json:"stream"`
		Topic     string `json:"topic"`
		Partition int    `json:"partition"`
		Offset    int    `json:"offset"`
	} `json:"meta"`
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Namespace int    `json:"namespace"`
	Title     string `json:"title"`
	TitleURL  string `json:"title_url"`
	Comment   string `json:"comment"`
	Timestamp int    `json:"timestamp"`
	User      string `json:"user"`
	Bot       bool   `json:"bot"`
	NotifyURL string `json:"notify_url"`
	Minor     bool   `json:"minor"`
	Length    struct {
		Old int `json:"old"`
		New int `json:"new"`
	} `json:"length"`
	Revision struct {
		Old int `json:"old"`
		New int `json:"new"`
	} `json:"revision"`
	ServerURL     string `json:"server_url"`
	ServerName    string `json:"server_name"`
	ServerPath    string `json:"server_script_path"`
	Wiki          string `json:"wiki"`
	ParsedComment string `json:"parsedcomment"`
}

// StoredEdit represents the edit structure from our API
type StoredEdit struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	TitleURL  string    `json:"titleUrl"`
	Comment   string    `json:"comment"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	Bot       bool      `json:"bot"`
	NotifyURL string    `json:"notifyUrl"`
	Minor     bool      `json:"minor"`
	LengthOld int       `json:"lengthOld"`
	LengthNew int       `json:"lengthNew"`
	ServerURL string    `json:"serverUrl"`
}

func fetchHistoricalEdits() ([]StoredEdit, error) {
	resp, err := http.Get("http://localhost:8080/edits")
	if err != nil {
		return nil, fmt.Errorf("error fetching historical edits: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}



	var edits []StoredEdit
	if err := json.Unmarshal(body, &edits); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return edits, nil
}

func main() {
	// Initialize maps for tracking edits
	editCounts := make(map[string]int)
	byteChanges := make(map[string]int)

	// Fetch and process historical edits first
	fmt.Println("Fetching historical edits...")
	historicalEdits, err := fetchHistoricalEdits()
	if err != nil {
		log.Printf("Error fetching historical edits: %v", err)
	} else {
		bar := progressbar.Default(int64(len(historicalEdits)))
		for _, edit := range historicalEdits {
			if !strings.Contains(edit.Title, ":") { // Filter out non-article edits
				editCounts[edit.Title]++
				byteDiff := abs(edit.LengthNew - edit.LengthOld)
				byteChanges[edit.Title] += byteDiff
			}
			bar.Add(1)
		}
		fmt.Printf("\nProcessed %d historical edits\n", len(historicalEdits))
	}

	// Now connect to the live stream
	fmt.Println("Connecting to live stream...")
	url := "https://stream.wikimedia.org/v2/stream/recentchange"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error connecting to stream: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		line = bytes.TrimPrefix(line, []byte("data: "))

		var change FullChange
		if err := json.Unmarshal(line, &change); err != nil {
			log.Printf("Error decoding event: %v", err)
			continue
		}

		if change.Meta.Domain == "en.wikipedia.org" && !strings.Contains(change.Title, ":") {
			editCounts[change.Title]++
			byteDiff := abs(change.Length.New - change.Length.Old)
			byteChanges[change.Title] += byteDiff
			fmt.Printf("edit #%d: %s (%d B)\n", editCounts[change.Title], change.Title, byteChanges[change.Title])
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error with stream: %v", err)
	}
}
