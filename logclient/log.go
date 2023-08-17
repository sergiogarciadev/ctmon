package logclient

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	ctgo "github.com/google/certificate-transparency-go"
	"github.com/sergiogarciadev/ctmon/logger"
)

type Log struct {
	Name             string `json:"name,omitempty"`
	Url              string `json:"url,omitempty"`
	PageSize         int    `json:"page_size,omitempty"`
	Concurrency      int    `json:"concurrency,omitempty"`
	TreeSize         int64  `json:"tree_size,omitempty"`
	Timestamp        int64  `json:"timestamp,omitempty"`
	LastEntry        int64  `json:"last_entry,omitempty"`
	InFlightRequests int    `json:"-"`
}

func (log *Log) GetSTH() (*ctgo.GetSTHResponse, error) {
	url := fmt.Sprintf("%s/ct/v1/get-sth", log.Url)
	log.InFlightRequests += 1
	response, err := getRequest[ctgo.GetSTHResponse](url)
	log.InFlightRequests -= 1
	return response, err
}

func (log *Log) GetEntries(start int64, end int64) (*ctgo.GetEntriesResponse, error) {
	url := fmt.Sprintf("%s/ct/v1/get-entries?start=%d&end=%d", log.Url, start, end)
	log.InFlightRequests += 1
	response, err := getRequest[ctgo.GetEntriesResponse](url)
	log.InFlightRequests -= 1
	return response, err
}

func (log *Log) StreamEntries(stop chan bool) chan Entry {
	var err error

	// We are fixing a maximum of 10 req/s for all logs
	rateLimiter := time.NewTicker(time.Duration(1_000_000_000 / 10))

	pages := make(chan Page, log.Concurrency*2)

	go func() {
		start := log.LastEntry + 1

		stopRequested := false

		for !stopRequested {
			<-rateLimiter.C

			page := NewPage(start, log.PageSize)

			if page.end > log.TreeSize {
				logger.Logger.Info(fmt.Sprintf("%-15s: Downloaded the entire tree, waiting for a minute...", log.Name))
				time.Sleep(1 * time.Minute)
				continue
			}

			pages <- page

			select {
			case <-stop:
				stopRequested = true
			default:
			}

			start = page.end + 1
		}
		close(pages)
	}()

	entries := make(chan Entry, log.PageSize*log.Concurrency*4)

	var wg sync.WaitGroup
	for i := 0; i < log.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for page := range pages {
				var entriesResponse *ctgo.GetEntriesResponse
				err = nil

				if entriesResponse, err = log.GetEntries(page.start, page.end); err != nil {
					for ; err != nil; entriesResponse, err = log.GetEntries(page.start, page.end) {
						logger.Logger.Info(fmt.Sprintf("%-15s: Error   : (%s)", log.Name, err))
						delay := (rand.Int() % 10)
						time.Sleep(time.Duration(time.Duration(delay) * time.Second))
					}
				}

				for index, entry := range entriesResponse.Entries {
					rawEntry, err := ctgo.RawLogEntryFromLeaf(int64(page.start), &entry)
					PanicOnError(err)
					entries <- ParseRawLogEntry(page.start+int64(index), log, rawEntry)
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(entries)
	}()

	ordered := make(chan Entry, 100)

	go func() {
		outOfOrderEntries := make([]Entry, 0)
		for entry := range entries {
			if entry.Id != log.LastEntry+1 {
				outOfOrderEntries = append(outOfOrderEntries, entry)
				continue
			}

			ordered <- entry
			log.LastEntry = entry.Id

			if len(outOfOrderEntries) > 1 {
				sort.Sort(ById(outOfOrderEntries))

				for len(outOfOrderEntries) > 0 && log.LastEntry+1 == outOfOrderEntries[0].Id {
					entry = outOfOrderEntries[0]

					ordered <- entry
					log.LastEntry = entry.Id

					outOfOrderEntries = outOfOrderEntries[1:]
				}
			}
		}

		sort.Sort(ById(outOfOrderEntries))
		for _, entry := range outOfOrderEntries {
			if log.LastEntry+1 != entry.Id {
				panic("Got an out of order entry")
			}
			ordered <- entry
			log.LastEntry = entry.Id
		}

		close(ordered)
	}()

	return ordered
}
