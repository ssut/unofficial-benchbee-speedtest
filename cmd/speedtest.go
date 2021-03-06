package cmd

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ssut/unofficial-benchbee-speedtest/benchbee"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.75 Safari/537.36 Edg/86.0.622.38"
const defaultPingCount = 50
const defaultTestDuration = 6 * time.Second
const defaultSimultaneousConnections = 5

func testSpeed(options benchbee.SpeedtestOptions) {
	info, err := benchbee.FetchBasicInfo(&http.Client{
		Transport: &http.Transport{
			Dial:    options.Dialer.Dial,
			DialTLS: options.Dialer.Dial,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n\tBENCHBEE BETA SPEEDTEST (Unofficial)")
	fmt.Println()
	fmt.Printf("\t  Server: %s - %s, %s, %s\n", info.ServerIPInfo.Isp, info.ServerIPInfo.City, info.ServerIPInfo.RegionName, info.ServerIPInfo.Country)
	fmt.Printf("\t     ISP: %s\n", info.ISP)

	st := benchbee.NewSpeedtest(info, options)

	// latency (ping)
	if err := st.TestPing(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\t Latency: %7.2f ms   (%.2f ms jitter)\n", st.Result.PingMillis, st.Result.JitterMillis)

	// download
	p := mpb.New(mpb.WithWidth(24))
	baseDownloadName := fmt.Sprintf("\tDownload: %7.2f Mbps", 0.0)
	downloadBar := p.AddBar(
		st.Options.DownloadTestDuration.Milliseconds(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return baseDownloadName
			}, decor.WC{W: len(baseDownloadName) - 1, C: decor.DidentRight}),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
		),
	)
	if err := st.TestSpeed(benchbee.SpeedtestDownloadWorker, func(res benchbee.SpeedtestIntermediateResult) {
		downloadBar.SetCurrent(res.Duration.Milliseconds())
		seconds := float64(res.Duration.Milliseconds()) / 1000.0
		speed := float64(res.Bytes) / seconds / 125000
		baseDownloadName = fmt.Sprintf("\tDownload: %7.2f Mbps", speed)
	}); err != nil {
		log.Fatal(err)
	}
	downloadBar.Abort(true)
	p.Wait()
	totalDownloadSeconds := float64(st.Result.TotalDownloadDuration.Milliseconds()) / 1000.0
	averageDownloadSpeed := float64(st.Result.TotalBytesDownloaded) / totalDownloadSeconds / 125000
	fmt.Printf("\tDownload: %7.2f Mbps (data used: %s)\n", averageDownloadSpeed, humanize.Bytes(st.Result.TotalBytesDownloaded))

	// upload
	p = mpb.New(mpb.WithWidth(24))
	baseUploadName := fmt.Sprintf("\t  Upload: %7.2f Mbps", 0.0)
	uploadBar := p.AddBar(
		st.Options.DownloadTestDuration.Milliseconds(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return baseUploadName
			}, decor.WC{W: len(baseUploadName) - 1, C: decor.DidentRight}),
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WCSyncSpace),
		),
	)
	if err := st.TestSpeed(benchbee.SpeedtestUploadWorker, func(res benchbee.SpeedtestIntermediateResult) {
		uploadBar.SetCurrent(res.Duration.Milliseconds())
		seconds := float64(res.Duration.Milliseconds()) / 1000.0
		speed := float64(res.Bytes) / seconds / 125000
		baseUploadName = fmt.Sprintf("\t  Upload: %7.2f Mbps", speed)
	}); err != nil {
		log.Fatal(err)
	}
	uploadBar.Abort(true)
	p.Wait()

	totalUploadSeconds := float64(st.Result.TotalUploadDuration.Milliseconds()) / 1000.0
	averageUploadSpeed := float64(st.Result.TotalBytesUploaded) / totalUploadSeconds / 125000
	fmt.Printf("\t  Upload: %7.2f Mbps (data used: %s)\n", averageUploadSpeed, humanize.Bytes(st.Result.TotalBytesUploaded))
	fmt.Println()
}
