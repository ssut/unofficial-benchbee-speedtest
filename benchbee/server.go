package benchbee

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ssut/unofficial-benchbee-speedtest/tool"
)

type BenchBeeMetadata struct {
	ISP string
	IP  string

	Download   string
	Upload     string
	PingWS     string
	DownloadWS string
	UploadWS   string

	ServerIPInfo *tool.IPInfo
}

func verifyServerURL(url string) error {
	if !strings.HasPrefix(url, "ws") && !strings.HasPrefix(url, "http") && !strings.Contains(url, "://") {
		return fmt.Errorf("Possibly not a valid server URL: %s", url)
	}

	return nil
}

func FetchBasicInfo(client *http.Client) (*BenchBeeMetadata, error) {
	req, err := http.NewRequest("GET", "http://beta.benchbee.co.kr/home.asp", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	body, err := doc.Html()
	if err != nil {
		return nil, err
	}

	info := &BenchBeeMetadata{
		ISP: doc.Find("#isp_name").Text(),
		IP:  doc.Find(".ipprd .ipadrs").Text(),
	}

	regex, err := regexp.Compile(`(settings\.url_[a-z_]+)\s?=\s?['|"](.+)['|"];`)
	if err != nil {
		return nil, err
	}
	matches := regex.FindAllStringSubmatch(string(body), -1)
	var representativeURL *url.URL
	for i := 0; i < len(matches); i++ {
		if len(matches[i]) < 3 {
			return nil, fmt.Errorf("Unexpected matched group length: %+v", matches)
		} else if err := verifyServerURL(matches[i][2]); err != nil {
			return nil, err
		}

		if representativeURL == nil || representativeURL.Hostname() == "" {
			representativeURL, _ = url.Parse(matches[i][2])
		}

		switch matches[i][1] {
		case "settings.url_dn":
			info.Download = matches[i][2]
			break

		case "settings.url_up":
			info.Upload = matches[i][2]
			break

		case "settings.url_dn_ws":
			info.DownloadWS = matches[i][2]
			break

		case "settings.url_up_ws":
			info.UploadWS = matches[i][2]
			break

		case "settings.url_ping":
			info.PingWS = matches[i][2]
			break
		}
	}

	if representativeURL == nil || representativeURL.Host == "" {
		return nil, errors.New("Invalid Representative Server URL")
	}
	serverIPInfo, err := tool.GetIPInfo(representativeURL.Hostname(), client)
	if err != nil {
		return nil, err
	}
	info.ServerIPInfo = serverIPInfo

	return info, nil
}
