package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type requestInfo struct {
	request, concurrency, timeout, timelimit *int64
	link                                     string
}

type responseInfo struct {
	status   int
	bytes    int
	duration time.Duration
}

type currencyInfo struct {
	requested int64
	responded int64
}

type summaryInfo struct {
	hostName         string
	port             string
	documentPath     string
	concurrencyLevel int64
	timeTakenToTest  time.Duration
	requestTime      time.Duration
	completedRequest int
	requests         int
	totalTransfered  int
}

func (v *summaryInfo) failedRequest() int {
	return v.requests - v.completedRequest
}

func (v *summaryInfo) timePerRequest() float64 {
	return v.requestTime.Seconds() / float64(v.requests)
}

func (v *summaryInfo) requestsPerSecond() float64 {
	return float64(v.requests) / v.requestTime.Seconds()
}

func (v *summaryInfo) transferRate() float64 {
	return (float64(v.totalTransfered) / v.requestTime.Seconds()) / 1024
}

func (v *summaryInfo) documentLength() float64 {
	return (float64(v.totalTransfered) / 1024) / float64(v.requests)
}

var (
	curInfo   = currencyInfo{}
	summary   summaryInfo
	startTest time.Time
)

func main() {
	initialize()
	resInfo, ok := getValues()
	if !ok {
		fmt.Printf("<< DOCUMENT >>\n")
		flag.PrintDefaults()
		fmt.Printf("<< DOCUMENT >>\n")
		os.Exit(-1)
	}
	extractServerInfo(resInfo)
	resultChannel := make(chan responseInfo)
	doBenchMark(resInfo, resultChannel)
	printReport()
}

func initialize() {
	summary = summaryInfo{"", "", "", 0, 0, 0, 0, 0, 0}
	startTest = time.Now()
}

func getValues() (resquestInfo requestInfo, ok bool) {
	ok = true
	request := flag.Int64("n", 0, "Number of requests to perform")
	concurrency := flag.Int64("c", 0, "Number of multiple requests to make at a time")
	timeout := flag.Int64("s", 30, "Maximum number of seconds to wait before the socket times out. Default is 30 seconds")
	timelimit := flag.Int64("t", 0, "Maximum number of seconds to spend for benchmarking. This implies a -n 50000 internally. Use this to benchmark the server within a fixed total amount of time. Per default there is no timelimi")
	flag.Parse()

	if *request == 0 {
		fmt.Printf("Invalid the number of requests of flag -n.")
		fmt.Println("")
		fmt.Println("Expect: n > 0")
		fmt.Printf("Actual: %v", *request)
		fmt.Println("")
		ok = false
	}
	if *concurrency == 0 || *request < *concurrency {
		fmt.Printf("Invalid the number of concurrency of flag -c.")
		fmt.Println("")
		fmt.Println("Expect: c > 0 and c > n")
		fmt.Printf("Actual: %v", *request)
		fmt.Println("")
		ok = false
	}

	link := ""
	if flag.NArg() == 0 {
		fmt.Println("The first argument, URL, is nil.")
		ok = false
	} else {
		link = flag.Arg(0)
	}

	resquestInfo = requestInfo{
		request:     request,
		concurrency: concurrency,
		timeout:     timeout,
		timelimit:   timelimit,
		link:        link,
	}
	return
}

func extractServerInfo(reqInfo requestInfo) {
	url, error := url.Parse(reqInfo.link)
	if error != nil {
		log.Fatal(error)
	}
	summary.hostName = url.Hostname()

	urlPort := url.Port()
	if len(urlPort) != 0 {
		summary.port = urlPort
	} else {
		if url.Scheme == "http" {
			summary.port = "80"
		} else {
			summary.port = "443"
		}
	}

	summary.documentPath = url.Path
	summary.concurrencyLevel = *reqInfo.concurrency
}

func worker(jobs chan requestInfo, results chan responseInfo) {
	for reqInfo := range jobs {
		checkLink(reqInfo, results)
	}
}

func doBenchMark(reqInfo requestInfo, resultChannel chan responseInfo) {
	fmt.Printf("Running benchmark on %s\n\n", reqInfo.link)

	requestChannel := make(chan requestInfo, *reqInfo.concurrency)

	for i := int64(0); i < *reqInfo.concurrency; i++ {
		go worker(requestChannel, resultChannel)
	}
	for i := int64(0); i < *reqInfo.request; i++ {
		curInfo.requested++
		requestChannel <- reqInfo
	}
	close(requestChannel)

	for response := range resultChannel {
		_ = response
		curInfo.responded++
		// fmt.Printf("%d %d %s", response.status, response.bytes, response.duration)
		if curInfo.requested == curInfo.responded {
			summary.timeTakenToTest = time.Now().Sub(startTest)
			close(resultChannel)
		}
	}
}

func checkLink(reqInfo requestInfo, c chan responseInfo) {
	start := time.Now()
	client := http.Client{
		Timeout: time.Duration(int(*reqInfo.timeout)) * time.Second,
	}
	res, error := client.Get(reqInfo.link)
	summary.requests++
	defer res.Body.Close()
	if error != nil {
		panic(error)
	} else {
		contentLength, _ := ioutil.ReadAll(res.Body)
		duration := time.Now().Sub(start)
		summary.completedRequest++
		summary.totalTransfered += len(contentLength)
		summary.requestTime += duration
		c <- responseInfo{
			status:   res.StatusCode,
			bytes:    len(contentLength),
			duration: duration,
		}
	}
}

func printReport() {
	fmt.Printf("Hostname:\t\t\t%s\n", summary.hostName)
	fmt.Printf("Port:\t\t\t\t%s\n\n", summary.port)
	fmt.Printf("Document Path:\t\t\t%s\n", summary.documentPath)
	fmt.Printf("Document Length:\t\t%.2f (bytes)\n\n", summary.documentLength())
	fmt.Printf("Concurrency Level:\t\t%d\n", summary.concurrencyLevel)
	fmt.Printf("Time taken to test:\t\t%.2f (s)\n", summary.timeTakenToTest.Seconds())
	fmt.Printf("Complete requests:\t\t%d\n", summary.requests-summary.failedRequest())
	fmt.Printf("Failed requests:\t\t%d\n", summary.failedRequest())
	fmt.Printf("Total transferred:\t\t%.2f (bytes)\n", float64(summary.totalTransfered)/1024)
	fmt.Printf("Request per seconds:\t\t%.2f (requests/s)\n", summary.requestsPerSecond())
	fmt.Printf("Time per request:\t\t%.2f (s)\n", summary.timePerRequest())
	fmt.Printf("Transfer rate:\t\t\t%.2f (Bytes/s)\n", summary.transferRate())
}
