# Unofficial BenchBee Speedtest

![](https://i.imgur.com/P3UxSQe.png)

**Currently in BETA.**

This is an unofficial Speedtest CLI that uses the same method as beta.benchbee.com, built for the situations where you don't have access to a full GUI environment and web browser.

## Disclaimer

Short: Just use your "free will."

This was developed just for fun. I am not responsible for this being used for such malicious purposes, and you may use and distribute this at your own risk.

## Usage

```shell script
An unofficial BenchBee Speeedtest CLI.

Usage:
  benchbee [flags]

Flags:
  -h, --help               help for benchbee
  -I, --interface string   Attempt to bind to the specified interface when connecting to servers
  -i, --ip string          Attempt to bind to the specified IP address when connecting to servers
```

For example, if you want to use en16 interface:

```
$ benchbee -I en16

        BENCHBEE BETA SPEEDTEST (Unofficial)

          Server: LG DACOM Corporation - Yongsan-dong, Seoul, South Korea
             ISP: LGU+
         Latency:   20.90 ms   (4.78 ms jitter)
        Download:  280.58 Mbps (data used: 210 MB)
          Upload:   79.46 Mbps (data used: 60 MB)
```

## License

This project is published under the ISC license.
