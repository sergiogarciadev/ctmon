package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/sergiogarciadev/ctmon/db"
	"github.com/sergiogarciadev/ctmon/logclient"
	"github.com/sergiogarciadev/ctmon/logger"
)

func saveState() {
	data, err := json.MarshalIndent(logclient.Logs, "", "  ")
	logclient.PanicOnError(err)
	logclient.PanicOnError(os.WriteFile("state.json", data, 0600))
}

func loadState() {
	data, err := os.ReadFile("state.json")
	logclient.PanicOnError(err)
	json.Unmarshal(data, &logclient.Logs)
}

func getHeads() {
	var wg sync.WaitGroup

	for _, log := range logclient.Logs {
		wg.Add(1)

		go func(log *logclient.Log) {
			sth, err := log.GetSTH()
			logclient.PanicOnError(err)
			if sth.Timestamp > uint64(log.Timestamp) {
				log.Timestamp = int64(sth.Timestamp)
				log.TreeSize = int64(sth.TreeSize)
			}
			wg.Done()
		}(log)
	}

	wg.Wait()
}

func printHeads() {
	for {
		fmt.Print("\033[H\033[2J")
		logNames := make([]string, 0, len(logclient.Logs))

		for logName := range logclient.Logs {
			logNames = append(logNames, logName)
		}

		sort.Strings(logNames)

		fmt.Println("Log              Head Timestamp            Tree Size      Downloaded    Remaining")

		for _, logName := range logNames {
			log := logclient.Logs[logName]
			timestamp := time.UnixMilli(int64(log.Timestamp))
			fmt.Printf("%-15s: %s %15d %15d %12d\n", log.Name, timestamp.Format(time.DateTime), log.TreeSize, log.LastEntry, log.TreeSize-log.LastEntry)
		}

		println("\n=================================================================================")

		time.Sleep(10 * time.Second)
	}
}

type DownloadCmd struct {
	Save     bool     `help:"Save certificates to database."`
	SaveBulk bool     `help:"Save certificates to database using BulkInsert"`
	IPs      []string `name:"ip" help:"IPs to user." type:"string"`
	Regex    string   `help:"Print certificates matching this regex to stdout"`
}

func (cmd *DownloadCmd) Run(cli *CliContext) error {
	if cmd.Save || cmd.SaveBulk {
		db.Open()
		defer db.Close()
	}
	defer logger.Close()

	for _, ip := range cmd.IPs {
		localAddr, err := net.ResolveIPAddr("ip", ip)
		if err != nil {
			panic(err)
		}
		localTCPAddr := net.TCPAddr{
			IP: localAddr.IP,
		}
		client := http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					LocalAddr: &localTCPAddr,
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
		logclient.AddHttpClient(client)
	}

	loadState()

	if cli.ShowStats {
		go printHeads()
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for {
			getHeads()
			saveState()

			time.Sleep(1 * time.Minute)
		}
	}()

	entries := make(chan logclient.Entry, 8)

	go func() {
		for _, log := range logclient.Logs {
			wg.Add(1)
			go func(log *logclient.Log) {
				stop := make(chan bool, 1)
				logEntries := log.StreamEntries(stop)
				for entry := range logEntries {
					entries <- entry
				}
				wg.Done()
			}(log)
		}
	}()

	bulkEntries := make([]*logclient.Entry, 10_000)
	bulkIndex := 0

	var re *regexp.Regexp

	if cmd.Regex != "" {
		var err error
		re, err = regexp.Compile(cmd.Regex)
		if err != nil {
			return err
		}
	}

	for entry := range entries {
		if cmd.SaveBulk {
			bulkEntries[bulkIndex] = &entry
			bulkIndex++

			if bulkIndex == 10_000 {
				db.BulkInsert(bulkEntries)
				bulkIndex = 0
			}
		} else if cmd.Save {
			err := db.Insert(&entry)
			if err != nil {
				println(err.Error())
			}
		}

		if entry.Certificate != nil && re != nil {
			for _, name := range entry.Certificate.DNSNames {
				if re.MatchString(name) {
					println(name)
				}
			}
		}
	}

	wg.Wait()

	return nil
}

type CliContext struct {
	ShowStats bool
}

var cli struct {
	ShowStats bool `help:"Show application status."`

	Download DownloadCmd `cmd:"" help:"Download certicates."`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run(&CliContext{ShowStats: cli.ShowStats})
	ctx.FatalIfErrorf(err)
}
