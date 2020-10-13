package cmd

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ssut/unofficial-benchbee-speedtest/benchbee"
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

			options := benchbee.SpeedtestOptions{
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
