package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type SpeedtestIntermediateResultCallback = func(result SpeedtestIntermediateResult)

type SpeedtestWorkerType int

const (
	SpeedtestDownloadWorker SpeedtestWorkerType = iota
	SpeedtestUploadWorker
)

type SpeedtestOptions struct {
	pingCount               int
	downloadTestDuration    time.Duration
	uploadTestDuration      time.Duration
	downloadTestConcurrency int
	uploadTestConcurrency   int
	header                  http.Header
	callbackPollInterval    time.Duration
}

type SpeedtestResult struct {
	pingMillis   float64
	jitterMillis float64
	downloadMbps float64
	uploadMbps   float64

	totalBytesDownloaded  uint64
	totalDownloadDuration time.Duration
	totalBytesUploaded    uint64
	totalUploadDuration   time.Duration
}

type SpeedtestIntermediateResult struct {
	Duration time.Duration
	Bytes    uint64
}

type Speedtest struct {
	info    *BenchBeeMetadata
	options *SpeedtestOptions

	result *SpeedtestResult
}

func NewSpeedtest(info *BenchBeeMetadata, options SpeedtestOptions) *Speedtest {
	st := &Speedtest{
		info:    info,
		options: &options,
		result:  &SpeedtestResult{},
	}
	if st.options.header == nil {
		st.options.header = http.Header{}
	}
	if st.options.header.Get("user-agent") == "" {
		st.options.header.Add("user-agent", userAgent)
	}

	return st
}

func (st *Speedtest) TestPing() error {
	c, _, err := websocket.DefaultDialer.Dial(st.info.PingWS, st.options.header)
	if err != nil {
		return err
	}
	c.SetCompressionLevel(1)
	defer c.Close()

	done := make(chan struct{})
	resultChan := make(chan int64, st.options.pingCount)

	go func(maxCount int) {
		defer close(done)

		var i int
		var t int64
		sendPing := func() {
			i++
			t = GetTime()
			c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("rtt:%d", t)))
		}

		sendPing()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}

			received := strings.SplitN(string(message), ":", 2)
			respondedTime, err := strconv.ParseInt(received[1], 10, 0)
			if err != nil {
				continue
			}

			latency := GetTime() - respondedTime
			resultChan <- latency
			if i+1 > maxCount {
				return
			}

			sendPing()
		}
	}(st.options.pingCount)

	latencies := []float64{}
	for {
		select {
		case <-done:
			st.result.pingMillis = GetAverage(latencies)
			st.result.jitterMillis = CalculateJitter(latencies)
			return nil

		case latency := <-resultChan:
			latencies = append(latencies, float64(latency))
			break
		}
	}
}

func (st *Speedtest) worker(ctx context.Context, workerType SpeedtestWorkerType, readyWG *sync.WaitGroup, nChan chan<- int64, respawnChan chan<- context.Context) {
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	var wsURL string
	switch workerType {
	case SpeedtestDownloadWorker:
		wsURL = st.info.DownloadWS
		break

	case SpeedtestUploadWorker:
		wsURL = st.info.UploadWS
		break
	}

	c, _, err := websocket.DefaultDialer.Dial(wsURL, st.options.header)
	c.EnableWriteCompression(false)
	c.SetCompressionLevel(1)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	writeBuffer := make([]byte, 64500)
	var fn func()
	switch workerType {
	case SpeedtestDownloadWorker:
		{
			fn = func() {
				_, r, err := c.NextReader()
				if err != nil {
					respawnChan <- ctx
					return
				}

				n, err := io.Copy(ioutil.Discard, r)
				if err != nil {
					respawnChan <- ctx
					return
				}
				nChan <- n
			}
		}
		break

	case SpeedtestUploadWorker:
		{
			fn = func() {
				wc, err := c.NextWriter(websocket.BinaryMessage)
				if err != nil {
					respawnChan <- ctx
					return
				}

				n, err := wc.Write(writeBuffer)
				if err != nil {
					respawnChan <- ctx
					return
				}
				nChan <- int64(n)
			}
		}
		break
	}

	readyWG.Done()
	for {
		select {
		case <-ctx.Done():
			return

		default:
			fn()
		}
	}
}

func (st *Speedtest) TestDownload(cb SpeedtestIntermediateResultCallback) error {
	respawnChan := make(chan context.Context, st.options.downloadTestConcurrency)
	readyWG := &sync.WaitGroup{}
	nChan := make(chan int64, 1024)
	var endsChan <-chan (time.Time)
	var preparedAt time.Time
	var cancels = make([]context.CancelFunc, st.options.downloadTestConcurrency)
	for i := 0; i < st.options.downloadTestConcurrency; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		readyWG.Add(1)
		go st.worker(ctx, SpeedtestDownloadWorker, readyWG, nChan, respawnChan)
	}
	readyWG.Wait()
	preparedAt = time.Now()
	endsChan = time.After(st.options.downloadTestDuration)

	var bytesRecv uint64
	pollFinishChan := make(chan struct{})
	go func(bytesRecvPointer *uint64, preparedAt time.Time, interval time.Duration, finish <-chan struct{}, cb SpeedtestIntermediateResultCallback) {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				n := atomic.LoadUint64(bytesRecvPointer)
				cb(SpeedtestIntermediateResult{
					Duration: time.Since(preparedAt),
					Bytes:    n,
				})
				break

			case <-finish:
				return
			}
		}
	}(&bytesRecv, preparedAt, st.options.callbackPollInterval, pollFinishChan, cb)

	for {
		select {
		case n := <-nChan:
			atomic.AddUint64(&bytesRecv, uint64(n))
			break

		case ctx := <-respawnChan:
			go st.worker(ctx, SpeedtestDownloadWorker, readyWG, nChan, respawnChan)
			break

		case endedAt := <-endsChan:
			pollFinishChan <- struct{}{}
			for _, cancel := range cancels {
				cancel()
			}

			st.result.totalDownloadDuration = endedAt.Sub(preparedAt)
			st.result.totalBytesDownloaded = uint64(bytesRecv)
			close(nChan)

			return nil
		}
	}
}

func (st *Speedtest) TestUpload(cb SpeedtestIntermediateResultCallback) error {
	respawnChan := make(chan context.Context, st.options.uploadTestConcurrency)
	readyWG := &sync.WaitGroup{}
	nChan := make(chan int64, 1024)
	var endsChan <-chan (time.Time)
	var preparedAt time.Time
	var cancels = make([]context.CancelFunc, st.options.uploadTestConcurrency)
	for i := 0; i < st.options.uploadTestConcurrency; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		readyWG.Add(1)
		go st.worker(ctx, SpeedtestUploadWorker, readyWG, nChan, respawnChan)
	}
	readyWG.Wait()
	preparedAt = time.Now()
	endsChan = time.After(st.options.uploadTestDuration)

	var bytesSent uint64
	pollFinishChan := make(chan struct{})
	go func(bytesSentPointer *uint64, preparedAt time.Time, interval time.Duration, finish <-chan struct{}, cb SpeedtestIntermediateResultCallback) {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				n := atomic.LoadUint64(bytesSentPointer)
				cb(SpeedtestIntermediateResult{
					Duration: time.Since(preparedAt),
					Bytes:    n,
				})
				break

			case <-finish:
				return
			}
		}
	}(&bytesSent, preparedAt, st.options.callbackPollInterval, pollFinishChan, cb)

	for {
		select {
		case n := <-nChan:
			atomic.AddUint64(&bytesSent, uint64(n))
			break

		case ctx := <-respawnChan:
			go st.worker(ctx, SpeedtestUploadWorker, readyWG, nChan, respawnChan)
			break

		case endedAt := <-endsChan:
			pollFinishChan <- struct{}{}
			for _, cancel := range cancels {
				cancel()
			}

			st.result.totalUploadDuration = endedAt.Sub(preparedAt)
			st.result.totalBytesUploaded = uint64(bytesSent)
			close(nChan)

			return nil
		}
	}
}
