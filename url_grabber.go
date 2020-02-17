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

type Task string
type Result struct {
	URL   string
	Count int
	Error error
}
type TaskManager func(Task)
type Worker func(<-chan Task, *sync.WaitGroup)


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
	pattern *regexp.Regexp,
	results chan<- *Result,
) Worker {
	return func(
		input <-chan Task,
		wg *sync.WaitGroup,
	) {
		defer wg.Done()
		for task := range input {
			url := string(task)
			count, err := downloadAndCount(url, pattern)
			results <- &Result{url, count, err}
		}
	}
}

func concurrentTaskManager(
	workersLimit int,
	startWorker Worker,
	wg *sync.WaitGroup,
) TaskManager {
	input := make(chan Task, workersLimit)
	workersLeft := workersLimit

	return func(x Task) {
		if len(x) == 0 {
			close(input)
			return
		}
		input <- x
		if workersLeft > 0 {
			workersLeft--
			wg.Add(1)
			go startWorker(input, wg)
		}
	}
}

func handleTasksFrom(
	stream io.Reader,
	handleTask TaskManager,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	defer handleTask("")

	scanner := bufio.NewScanner(stream)
	for {
		scanner.Scan()
		text := scanner.Text()
		if len(text) == 0 {
			break
		}
		handleTask(Task(text))
	}
}

func logResultCounts(
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

	wg1 := new(sync.WaitGroup)
	workersLimit := 5
	handleTask := concurrentTaskManager(workersLimit, startWorker, wg1)

	wg1.Add(1)
	go handleTasksFrom(os.Stdin, handleTask, wg1)

	wg2 := new(sync.WaitGroup)
	wg2.Add(1)
	go logResultCounts(results, wg2)

	wg1.Wait()
	close(results)
	wg2.Wait()
}
