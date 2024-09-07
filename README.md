# htp

HTTP Tick Ping - a tool to send HTTP probe requests at regular intervals

The requests are sent at the exact scheduled time depending on the set interval, even if the previous requests have not completed yet. This might help determine how long the service exposed on the target URL stays unavailable from the user's perspective after e.g. k8s pod or web server restart.

## Usage

```
A tool to send HTTP probe requests at regular intervals

Usage:
  htp URL [flags]

Flags:
  -i, --interval int    interval between requests in milliseconds (default 1000)
  -l, --limit int       number of requests to make (default unlimited)
  -p, --pager int       number of requests to pager (default 25)
  -m, --method string   specify HTTP request method (default "GET")
  -j, --json string     jq-compatible filter for JSON response
  -k, --insecure        allow insecure connections
  -h, --help            help for htp
  -v, --version         version for htp
```

## Example

```sh
❯ htp http://ifconfig.co/json -l 5 -j .ip
GET http://ifconfig.co/json [1000ms]

1: start=09:56:40.594, duration=93ms, end=09:56:40.688, url=http://ifconfig.co/json [200] => "92.193.54.221"
2: start=09:56:41.594, duration=64ms, end=09:56:41.658, url=http://ifconfig.co/json [200] => "92.193.54.221"
3: start=09:56:42.594, duration=66ms, end=09:56:42.661, url=http://ifconfig.co/json [200] => "92.193.54.221"
4: start=09:56:43.595, duration=63ms, end=09:56:43.658, url=http://ifconfig.co/json [200] => "92.193.54.221"
5: start=09:56:44.594, duration=64ms, end=09:56:44.659, url=http://ifconfig.co/json [200] => "92.193.54.221"
```
