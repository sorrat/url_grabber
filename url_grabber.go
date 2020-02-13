package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

type Result struct {
	URL   string
	Count int
	Error error
}

func downloadPage(url string) (string, error) {
	httpClient := http.Client{
		Timeout: 20 * time.Second,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP status = %v", resp.StatusCode)
		return "", err
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func countOccurrences(text string, pattern *regexp.Regexp) int {
	matches := pattern.FindAllStringIndex(text, -1)
	return len(matches)
}

func downloadAndCount(url string, pattern *regexp.Regexp) (int, error) {
	log.Printf("Working on %v\n", url)
	page, err := downloadPage(url)
	if err != nil {
		return 0, err
	}
	count := countOccurrences(page, pattern)
	return count, nil
}

func downloadAndCountWorker(
	urls <-chan string,
	pattern *regexp.Regexp,
	results chan<- *Result,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for url := range urls {
		count, err := downloadAndCount(url, pattern)
		results <- &Result{url, count, err}
	}
}

func enqueueTasksFrom(
	stream io.Reader,
	pattern *regexp.Regexp,
	workersLimit int,
	results chan<- *Result,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	urls := make(chan string, workersLimit)
	defer close(urls)

	workersNum := workersLimit
	scanner := bufio.NewScanner(stream)
	for {
		scanner.Scan()
		text := scanner.Text()
		if len(text) == 0 {
			break
		}
		if workersNum > 0 {
			wg.Add(1)
			go downloadAndCountWorker(urls, pattern, results, wg)
			workersNum--
		}
		urls <- text
	}
}

func processResults(
	results <-chan *Result,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	total := 0
	for res := range results {
		if res.Error != nil {
			log.Printf("Error for %v: %v\n", res.URL, res.Error)
		} else {
			log.Printf("Count for %v: %v\n", res.URL, res.Count)
			total += res.Count
		}
	}
	log.Printf("Total: %v\n", total)
}

func main() {
	const workersLimit = 5
	pattern := regexp.MustCompile(`\bGo\b`)
	results := make(chan *Result)

	wg1 := new(sync.WaitGroup)
	wg1.Add(1)
	go enqueueTasksFrom(os.Stdin, pattern, workersLimit, results, wg1)

	wg2 := new(sync.WaitGroup)
	wg2.Add(1)
	go processResults(results, wg2)

	wg1.Wait()
	close(results)
	wg2.Wait()
}
