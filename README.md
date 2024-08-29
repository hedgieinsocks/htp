# htp

`htp` (http tick ping) - a tool to send HTTP probe requests at regular intervals

The requests are sent at the exact scheduled time depending on the set interval, even if the previous requests have not completed yet. This might help determine how long the service exposed on the target URL stays unavailable from the user's perspective after e.g. k8s pod or web server restart.

## Usage

```
A tool to send HTTP probe requests at regular intervals

Usage:
  htp URL [flags]

Flags:
  -i, --interval int    interval between requests in milliseconds (default 1000)
  -l, --limit int       number of requests to make (default unlimited)
  -t, --tail int        number of requests to tail (default 25)
  -m, --method string   specify HTTP request method (default "HEAD")
  -k, --insecure        allow insecure connections
  -h, --help            help for htp
  -v, --version         version for htp
```

## Example

```sh
❯ htp https://google.com -l 10
HEAD http://google.com every 1000ms

1: start=09:35:55.830, duration=262ms, end=09:35:56.092 [200] http://www.google.com/
2: start=09:35:56.831, duration=105ms, end=09:35:56.936 [200] http://www.google.com/
3: start=09:35:57.830, duration=103ms, end=09:35:57.934 [200] http://www.google.com/
4: start=09:35:58.831, duration=104ms, end=09:35:58.935 [200] http://www.google.com/
5: start=09:35:59.831, duration=106ms, end=09:35:59.937 [200] http://www.google.com/
6: start=09:36:00.831, duration=103ms, end=09:36:00.933 [200] http://www.google.com/
7: start=09:36:01.831, duration=105ms, end=09:36:01.936 [200] http://www.google.com/
8: start=09:36:02.830, duration=103ms, end=09:36:02.934 [200] http://www.google.com/
9: start=09:36:03.830, duration=103ms, end=09:36:03.933 [200] http://www.google.com/
10: start=09:36:04.831, duration=127ms, end=09:36:04.958 [200] http://www.google.com/
```
