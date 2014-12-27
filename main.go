//docker run -v .:/tmp -p 2379:2379 -d etcd /go/bin/etcd --listen-client-urls 'http://0.0.0.0:2379'

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
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

func main() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)

	keyURL := flag.String("url", "http://127.0.0.1:2379/v2/keys/name", "The url stores key-value")
	flag.Parse()
	connection := [6]int{1000, 5000, 10000, 20000, 40000}
	latency := make([]int, len(connection))
	for c := 0; c < len(connection); c++ {
		log.Printf("Start to test %v connections to %v\n", connection[c], *keyURL)
		latency[c] = test_LatencyOfPresistentConnection(connection[c], keyURL)
		log.Printf("complete %v in %v ms\n", connection[c], latency)
	}
}
