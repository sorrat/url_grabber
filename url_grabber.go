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

type Worker func(<-chan string, *sync.WaitGroup)

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

func downloadAndCountWorker(pattern *regexp.Regexp, results chan<- *Result) Worker {
	return func(
		input <-chan string,
		wg *sync.WaitGroup,
	) {
		defer wg.Done()
		for url := range input {
			count, err := downloadAndCount(url, pattern)
			results <- &Result{url, count, err}
		}
	}
}

func enqueueTasks(
	stream io.Reader,
	q chan<- string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	defer close(q)

	scanner := bufio.NewScanner(stream)
	for {
		scanner.Scan()
		text := scanner.Text()
		if len(text) == 0 {
			break
		}
		q <- text
	}
}

func processTasks(
	input <-chan string,
	workersLimit int,
	startWorker Worker,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	workersLeft := workersLimit
	inputBuffer := make(chan string, workersLimit)
	defer close(inputBuffer)

	for item := range input {
		inputBuffer <- item

		if workersLeft > 0 {
			wg.Add(1)
			go startWorker(inputBuffer, wg)
			workersLeft--
		}
	}
}

func processResultCounts(
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
	pattern := regexp.MustCompile(`\bGo\b`)
	results := make(chan *Result)
	startWorker := downloadAndCountWorker(pattern, results)

	workersLimit := 5
	input := make(chan string)

	wg1 := new(sync.WaitGroup)
	wg1.Add(2)
	go enqueueTasks(os.Stdin, input, wg1)
	go processTasks(input, workersLimit, startWorker, wg1)

	wg2 := new(sync.WaitGroup)
	wg2.Add(1)
	go processResultCounts(results, wg2)

	wg1.Wait()
	close(results)
	wg2.Wait()
}
