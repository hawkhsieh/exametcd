//docker run -v .:/tmp -p 2379:2379 -d etcd /go/bin/etcd --listen-client-urls 'http://0.0.0.0:2379'

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

func makeSession(url string, respChannel chan string) {

	var respBodyString string
	for {
		resp, err := http.Get(url)
		if err != nil {
			//		log.Println(err)
			<-time.After(1)
			continue
		}

		respChannel <- "ready"
		respBodyByte, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			resp.Body.Close()
			<-time.After(1)
			continue
		}

		respBodyString = string(respBodyByte[:])
		resp.Body.Close()
		break
	}
	respChannel <- respBodyString
}

func makePUTRequest(url *string, form string) string {

	var respBodyString string
	for {
		client := &http.Client{}
		request, err := http.NewRequest("PUT", *url, strings.NewReader(form))
		if err != nil {
			log.Println(err)
			<-time.After(1)
			continue
		}
		resp, err := client.Do(request)
		if err != nil {
			log.Println(err)
			<-time.After(1)
			continue
		}

		respBodyByte, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			<-time.After(1)
			resp.Body.Close()
			continue
		}
		respBodyString = string(respBodyByte[:len(respBodyByte)])
		resp.Body.Close()
		break
	}
	return respBodyString
}

//測試大量長連線連到同一個key並監看，從寫入此KEY的開始到所有的回應都接收到，會花多少時間
func test_LatencyOfPresistentConnection(testAmount int, keyURL *string) int {

	summary := 0
	repeatNumber := 3
	for repeat := 0; repeat < repeatNumber; repeat++ {

		respChannel := make(chan string)
		for i := 0; i < testAmount; i++ {
			go makeSession(*keyURL+"?wait=true", respChannel)
		}

		for i := 0; i < testAmount; i++ {
			<-respChannel
		}
		log.Printf("%d connection are established\n", testAmount)
		makePUTRequest(keyURL, "value=hawk")

		timeStart := time.Now().UnixNano()
		for i := 0; i < testAmount; i++ {
			<-respChannel
		}
		timeEnd := time.Now().UnixNano()
		diff := int((timeEnd - timeStart) / 1e6)
		log.Printf("Test %v spent %vms\n", repeat, diff)
		summary = summary + diff
	}
	return summary / repeatNumber

}

// -------------------------

type TestingJob struct {
	Url             string
	ConnAmount      int
	PeriodReport    []int
	SessionWg       sync.WaitGroup
	RespWg          sync.WaitGroup
	ConnSuccessFlag chan bool
}

func (t *TestingJob) MakeSession() {
	for {
		resp, err := http.Get(t.Url + "?wait=true")
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 1)
			continue
		}
		t.SessionWg.Done()
		t.ConnSuccessFlag <- true

		_, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 1)
			continue
		}
		break
	}
	t.RespWg.Done()
}

func (t *TestingJob) MakePutRequest(form string) {
	//r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	//f := r.Int()
	for {
		client := &http.Client{}
		request, err := http.NewRequest("PUT", t.Url, strings.NewReader(form))
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 1)
			continue
		}
		resp, err := client.Do(request)
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 1)
			continue
		}

		_, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Println(err.Error())
			time.Sleep(time.Second * 1)
			continue
		}
		break
	}
}

func (t *TestingJob) Report() {
	totalConn := 0
	for {
		select {
		case <-time.After(time.Second * 1):
			fmt.Println(totalConn)
		case <-t.ConnSuccessFlag:
			totalConn += 1
			for _, val := range t.PeriodReport {
				if val == totalConn {
					unixTime := time.Now().Unix()
					var m0 runtime.MemStats
					runtime.ReadMemStats(&m0)
					fmt.Printf("time: %d    totalCount: %d     NumGoroutine: %d    Memory: %.2f mb  \n", unixTime, totalConn, runtime.NumGoroutine(), float64(m0.Sys)/1024/1024)
				}
			}
		}
	}
}

func (t *TestingJob) StartTesting() string {
	// Report connection numbers
	go t.Report()

	for i := 0; i <= t.ConnAmount; i++ {
		t.SessionWg.Add(1)
		t.RespWg.Add(1)
		go t.MakeSession()
	}
	t.SessionWg.Wait()
	//t.MakePutRequest("value=jex")
	startTime := time.Now()
	t.RespWg.Wait()

	endTime := time.Now()
	spendTime := endTime.Sub(startTime)
	return spendTime.String()
}

func main() {

	//runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)

	keyURL := flag.String("url", "http://54.148.22.36:4001/v2/keys/name", "The url stores key-value")

	// keyURL := flag.String("url", "http://127.0.0.1:4001/v2/keys/name", "The url stores key-value")
	connAmount := flag.Int("c", 20000, "Testing connection amount")
	flag.Parse()

	// Put
	//t := TestingJob{}
	//t.Url = *keyURL
	//startTime := time.Now()
	//for i := 0; i <= 50000; i++ {
	//	go t.MakePutRequest("value=jex")
	//}
	//endTime := time.Now()
	//spendTime := endTime.Sub(startTime)
	//log.Println(spendTime)

	// Jex
	t := TestingJob{}
	t.Url = *keyURL
	t.ConnAmount = *connAmount
	t.PeriodReport = []int{1000, 2000, 4000, 8000, 16000, 20000, 32000, 64000}
	t.ConnSuccessFlag = make(chan bool)
	//for i := range t.PeriodReport {
	//	log.Printf("Start to test %v connections to %v\n", t.PeriodReport[i], *keyURL)
	//	latency := t.StartTesting()
	//	log.Printf("complete %v in %v \n", *connAmount, latency)
	//}
	latency := t.StartTesting()
	fmt.Printf("complete %v connections in %v \n", *connAmount, latency)

	// Hawk
	//connection := [6]int{1000, 5000, 10000, 20000, 40000}
	//connection := [1]int{10000}
	//latency := make([]int, len(connection))
	//for c := 0; c < len(connection); c++ {
	//	log.Printf("Start to test %v connections to %v\n", connection[c], *keyURL)
	//	latency[c] = test_LatencyOfPresistentConnection(connection[c], keyURL)
	//	log.Printf("complete %v in %v ms\n", connection[c], latency)
	//}
}
