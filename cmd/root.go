package cmd

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ssut/unofficial-benchbee-speedtest/benchbee"
	"github.com/ssut/unofficial-benchbee-speedtest/tool"
)

var (
	rootCmd = &cobra.Command{
		Use:   "benchbee",
		Short: "An unofficial BenchBee Speeedtest CLI.",
		Run: func(cmd *cobra.Command, args []string) {
			concurrency := defaultSimultaneousConnections
			concurrencyStr := os.Getenv("BENCHBEE_CONCURRENCY")
			if concurrencyStr != "" {
				var err error
				concurrency, err = strconv.Atoi(concurrencyStr)
				if err != nil {
					log.Fatal(err)
				}
			}
			userAgentStr := os.Getenv("BENCHBEE_USER_AGENT")

			ifname, err := cmd.PersistentFlags().GetString("interface")
			if err != nil {
				log.Fatal(err)
			}
			ipAddr, err := cmd.PersistentFlags().GetString("ip")
			if err != nil {
				log.Fatal(err)
			}

			dialer := &net.Dialer{}
			if ifname != "" {
				dialer, err = tool.CreateDialerUsingInterfaceName(ifname)
				if err != nil {
					log.Fatal(err)
				}
			} else if ipAddr != "" {
				dialer, err = tool.CreateDialerUsingIPAddress(ipAddr)
				if err != nil {
					log.Fatal(err)
				}
			}

			options := benchbee.SpeedtestOptions{
				Dialer:                  dialer,
				UserAgent:               userAgentStr,
				PingCount:               defaultPingCount,
				DownloadTestDuration:    defaultTestDuration,
				UploadTestDuration:      defaultTestDuration,
				DownloadTestConcurrency: concurrency,
				UploadTestConcurrency:   concurrency,
				CallbackPollInterval:    100 * time.Millisecond,
			}
			testSpeed(options)
		},
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("interface", "I", "", "Attempt to bind to the specified interface when connecting to servers")
	rootCmd.PersistentFlags().StringP("ip", "i", "", "Attempt to bind to the specified IP address when connecting to servers")

	viper.SetDefault("license", "ISC")
}
