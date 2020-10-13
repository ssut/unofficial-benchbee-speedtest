package benchbee

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ssut/unofficial-benchbee-speedtest/tool"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.75 Safari/537.36 Edg/86.0.622.38"
const defaultPingCount = 50
const defaultTestDuration = 6 * time.Second
const defaultSimultaneousConnections = 5

type SpeedtestIntermediateResultCallback = func(result SpeedtestIntermediateResult)

type SpeedtestWorkerType int

const (
	SpeedtestDownloadWorker SpeedtestWorkerType = iota
	SpeedtestUploadWorker
)

type SpeedtestOptions struct {
	PingCount               int
	DownloadTestDuration    time.Duration
	UploadTestDuration      time.Duration
	DownloadTestConcurrency int
	UploadTestConcurrency   int
	Header                  http.Header
	CallbackPollInterval    time.Duration
	UserAgent               string
	Dialer                  *net.Dialer
}

type SpeedtestResult struct {
	PingMillis            float64
	JitterMillis          float64
	TotalBytesDownloaded  uint64
	TotalDownloadDuration time.Duration
	TotalBytesUploaded    uint64
	TotalUploadDuration   time.Duration
}

type SpeedtestIntermediateResult struct {
	Duration time.Duration
	Bytes    uint64
}

type Speedtest struct {
	Info    *BenchBeeMetadata
	Options *SpeedtestOptions
	Result  *SpeedtestResult

	websocketDialer *websocket.Dialer
}

func NewSpeedtest(info *BenchBeeMetadata, options SpeedtestOptions) *Speedtest {
	st := &Speedtest{
		Info:    info,
		Options: &options,
		Result:  &SpeedtestResult{},
	}
	if st.Options.Header == nil {
		st.Options.Header = http.Header{}
	}
	if st.Options.Header.Get("user-agent") == "" {
		if st.Options.UserAgent == "" {
			st.Options.Header.Add("user-agent", defaultUserAgent)
		} else {
			st.Options.Header.Add("user-agent", st.Options.UserAgent)
		}
	}

	st.websocketDialer = websocket.DefaultDialer
	if options.Dialer != nil {
		st.websocketDialer.NetDial = options.Dialer.Dial
	}

	return st
}

func (st *Speedtest) TestPing() error {
	c, _, err := st.websocketDialer.Dial(st.Info.PingWS, st.Options.Header)
	if err != nil {
		return err
	}
	c.SetCompressionLevel(1)
	defer c.Close()

	done := make(chan struct{})
	resultChan := make(chan int64, st.Options.PingCount)

	go func(maxCount int) {
		defer close(done)

		var i int
		var t int64
		sendPing := func() {
			i++
			t = tool.GetTime()
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

			latency := tool.GetTime() - respondedTime
			resultChan <- latency
			if i+1 > maxCount {
				return
			}

			sendPing()
		}
	}(st.Options.PingCount)

	latencies := []float64{}
	for {
		select {
		case <-done:
			st.Result.PingMillis = tool.GetAverage(latencies)
			st.Result.JitterMillis = tool.CalculateJitter(latencies)
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
		wsURL = st.Info.DownloadWS
		break

	case SpeedtestUploadWorker:
		wsURL = st.Info.UploadWS
		break
	}

	c, _, err := st.websocketDialer.Dial(wsURL, st.Options.Header)
	c.EnableWriteCompression(false)
	c.SetCompressionLevel(1)
	if err != nil {
		panic(err)
	}
	defer c.Close()

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
			writeBuffer := make([]byte, 64500)
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
	respawnChan := make(chan context.Context, st.Options.DownloadTestConcurrency)
	readyWG := &sync.WaitGroup{}
	nChan := make(chan int64, 1024)
	var endsChan <-chan (time.Time)
	var preparedAt time.Time
	var cancels = make([]context.CancelFunc, st.Options.DownloadTestConcurrency)
	for i := 0; i < st.Options.DownloadTestConcurrency; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		readyWG.Add(1)
		go st.worker(ctx, SpeedtestDownloadWorker, readyWG, nChan, respawnChan)
	}
	readyWG.Wait()
	preparedAt = time.Now()
	endsChan = time.After(st.Options.DownloadTestDuration)

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
	}(&bytesRecv, preparedAt, st.Options.CallbackPollInterval, pollFinishChan, cb)

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

			st.Result.TotalDownloadDuration = endedAt.Sub(preparedAt)
			st.Result.TotalBytesDownloaded = uint64(bytesRecv)
			close(nChan)

			return nil
		}
	}
}

func (st *Speedtest) TestUpload(cb SpeedtestIntermediateResultCallback) error {
	respawnChan := make(chan context.Context, st.Options.UploadTestConcurrency)
	readyWG := &sync.WaitGroup{}
	nChan := make(chan int64, 1024)
	var endsChan <-chan (time.Time)
	var preparedAt time.Time
	var cancels = make([]context.CancelFunc, st.Options.UploadTestConcurrency)
	for i := 0; i < st.Options.UploadTestConcurrency; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		readyWG.Add(1)
		go st.worker(ctx, SpeedtestUploadWorker, readyWG, nChan, respawnChan)
	}
	readyWG.Wait()
	preparedAt = time.Now()
	endsChan = time.After(st.Options.UploadTestDuration)

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
	}(&bytesSent, preparedAt, st.Options.CallbackPollInterval, pollFinishChan, cb)

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

			st.Result.TotalUploadDuration = endedAt.Sub(preparedAt)
			st.Result.TotalBytesUploaded = uint64(bytesSent)
			close(nChan)

			return nil
		}
	}
}
