//docker run -v .:/tmp -p 2379:2379 -d etcd /go/bin/etcd --listen-client-urls 'http://0.0.0.0:2379'

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

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
			//log.Println(err.Error())
			time.Sleep(time.Second * 1)
			continue
		}
		t.SessionWg.Done()
		t.ConnSuccessFlag <- true

		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			//log.Println(err.Error())
			resp.Body.Close()
			time.Sleep(time.Second * 1)
			continue
		}
		resp.Body.Close()
		break
	}
	t.RespWg.Done()
}

func (t *TestingJob) MakePutRequest(form string) {
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
		if err != nil {
			log.Println(err.Error())
			resp.Body.Close()
			time.Sleep(time.Second * 1)
			continue
		}
		resp.Body.Close()
		break
	}
}

func (t *TestingJob) Report() {
	totalConn := 0
	for {
		select {
		case <-time.After(time.Second * 1):
			//fmt.Println(totalConn)
		case <-t.ConnSuccessFlag:
			totalConn += 1
			//for _, val := range t.PeriodReport {
			//	if val == totalConn {
			//		unixTime := time.Now().Unix()
			//		var m0 runtime.MemStats
			//		runtime.ReadMemStats(&m0)
			//		fmt.Printf("time: %d    totalCount: %d     NumGoroutine: %d    Memory: %.2f mb  \n", unixTime, totalConn, runtime.NumGoroutine(), float64(m0.Sys)/1024/1024)
			//	}
			//}
			if totalConn%1000 == 0 {
				unixTime := time.Now().Unix()
				var m0 runtime.MemStats
				runtime.ReadMemStats(&m0)
				//fmt.Printf("time: %d    totalCount: %d     NumGoroutine: %d    Memory: %.2f mb  \n", unixTime, totalConn, runtime.NumGoroutine(), float64(m0.Sys)/1024/1024)
				fmt.Printf("%d,%d,%d,%.2f mb\n", unixTime, totalConn, runtime.NumGoroutine(), float64(m0.Sys)/1024/1024)
			}
		}
	}
}

func (t *TestingJob) WaitTesting() string {
	// Report connection numbers
	go t.Report()

	for i := 0; i <= t.ConnAmount; i++ {
		t.SessionWg.Add(1)
		t.RespWg.Add(1)
		go t.MakeSession()
	}
	t.SessionWg.Wait()
	startTime := time.Now()
	t.MakePutRequest("value=jex")
	t.RespWg.Wait()

	endTime := time.Now()
	spendTime := endTime.Sub(startTime)
	return spendTime.String()
}

func (t *TestingJob) RandomKeyTesting() string {
	startTime := time.Now()
	for i := 0; i < t.ConnAmount; i++ {
		r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
		f := r.Int()
		t.Url = fmt.Sprintf("http://54.68.2.231:4001/v2/keys/%d", f)
		go t.MakePutRequest(fmt.Sprintf("value=%d", f))
	}
	endTime := time.Now()
	spendTime := endTime.Sub(startTime)
	return spendTime.String()
}

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	connAmount := flag.Int("c", 2000, "Testing connection amount")
	testType := flag.Int("t", 1, "1: Wait the same key, 2: Put random key")
	flag.Parse()

	var latency string
	t := TestingJob{}
	t.ConnAmount = *connAmount
	t.PeriodReport = []int{1000, 2000, 4000, 8000, 16000, 20000, 32000, 40000, 45000, 50000, 55000, 60000, 64000}
	t.ConnSuccessFlag = make(chan bool)

	switch *testType {
	case 1:
		t.Url = "http://54.68.2.231:4001/v2/keys/name"
		latency = t.WaitTesting()
	case 2:
		latency = t.RandomKeyTesting()
	}
	fmt.Printf("complete %v connections in %v \n", *connAmount, latency)
}
