package main

import (
	"fmt"
	"log"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.75 Safari/537.36 Edg/86.0.622.38"
const defaultPingCount = 50
const defaultTestDuration = 6 * time.Second
const defaultSimultaneousConnections = 5

func main() {
	info, err := fetchBasicInfo()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n\tBENCHBEE BETA SPEEDTEST (Unofficial)")
	fmt.Println()
	fmt.Printf("\t  Server: %s - %s, %s, %s\n", info.ServerIPInfo.Isp, info.ServerIPInfo.City, info.ServerIPInfo.RegionName, info.ServerIPInfo.Country)
	fmt.Printf("\t     ISP: %s\n", info.ISP)

	st := NewSpeedtest(info, SpeedtestOptions{
		pingCount:               defaultPingCount,
		downloadTestDuration:    defaultTestDuration,
		uploadTestDuration:      defaultTestDuration,
		downloadTestConcurrency: defaultSimultaneousConnections,
		uploadTestConcurrency:   defaultSimultaneousConnections,
		callbackPollInterval:    100 * time.Millisecond,
	})
	if err := st.TestPing(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\t Latency: %7.2f ms   (%.2f ms jitter)\n", st.result.pingMillis, st.result.jitterMillis)
	p := mpb.New(mpb.WithWidth(24))
	baseDownloadName := fmt.Sprintf("\tDownload: %7.2f Mbps", 0.0)
	downloadBar := p.AddBar(
		st.options.downloadTestDuration.Milliseconds(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return baseDownloadName
			}, decor.WC{W: len(baseDownloadName) - 1, C: decor.DidentRight}),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
		),
	)
	st.TestDownload(func(res SpeedtestIntermediateResult) {
		downloadBar.SetCurrent(res.Duration.Milliseconds())
		seconds := float64(res.Duration.Milliseconds()) / 1000.0
		speed := float64(res.Bytes) / seconds / 125000
		baseDownloadName = fmt.Sprintf("\tDownload: %7.2f Mbps", speed)
	})
	downloadBar.Abort(true)
	p.Wait()
	totalDownloadSeconds := float64(st.result.totalDownloadDuration.Milliseconds()) / 1000.0
	averageDownloadSpeed := float64(st.result.totalBytesDownloaded) / totalDownloadSeconds / 125000
	fmt.Printf("\tDownload: %7.2f Mbps (data used: %s)\n", averageDownloadSpeed, humanize.Bytes(st.result.totalBytesDownloaded))

	p = mpb.New(mpb.WithWidth(24))
	baseUploadName := fmt.Sprintf("\t  Upload: %7.2f Mbps", 0.0)
	uploadBar := p.AddBar(
		st.options.downloadTestDuration.Milliseconds(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return baseUploadName
			}, decor.WC{W: len(baseUploadName) - 1, C: decor.DidentRight}),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
		),
	)
	st.TestUpload(func(res SpeedtestIntermediateResult) {
		uploadBar.SetCurrent(res.Duration.Milliseconds())
		seconds := float64(res.Duration.Milliseconds()) / 1000.0
		speed := float64(res.Bytes) / seconds / 125000
		baseUploadName = fmt.Sprintf("\t  Upload: %7.2f Mbps", speed)
	})
	uploadBar.Abort(true)
	p.Wait()

	totalUploadSeconds := float64(st.result.totalUploadDuration.Milliseconds()) / 1000.0
	averageUploadSpeed := float64(st.result.totalBytesUploaded) / totalUploadSeconds / 125000
	fmt.Printf("\t  Upload: %7.2f Mbps (data used: %s)\n", averageUploadSpeed, humanize.Bytes(st.result.totalBytesUploaded))
}
