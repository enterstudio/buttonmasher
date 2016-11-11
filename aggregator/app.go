/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	"k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/rest"

	"k8s.io/client-go/1.4/pkg/fields"
	"k8s.io/client-go/1.4/pkg/labels"
)

var (
	addr       = flag.String("address", "localhost:8080", "The address to serve on")
	selector   = flag.String("selector", "", "The label selector for pods")
	sleep      = flag.Duration("sleep", 3*time.Second, "The sleep period between aggregations")
	httpClient = http.Client{
		Timeout: time.Duration(2 * time.Second),
	}

	serveData = []byte{}
	lock      = sync.Mutex{}
)

type Stat struct {
	Version       string  `json:"v"`
	Rps           float64 `json:"rps"`
	TotalRequests uint64  `json:"t"`
}

type Response struct {
	Servers int    `json:"servers"`
	Stats   []Stat `json:"stats"`
}

func getData() []byte {
	lock.Lock()
	defer lock.Unlock()
	return serveData
}

func setData(data []byte) {
	lock.Lock()
	defer lock.Unlock()
	serveData = data
}

func serveHTTP(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Access-Control-Allow-Origin", "*")
	res.WriteHeader(http.StatusOK)
	res.Write(getData())
}

func healthCheck(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("ok"))
}

func main() {
	flag.Parse()

	http.HandleFunc("/api/healthz", healthCheck)
	http.HandleFunc("/api/aggregator/stats", serveHTTP)
	go http.ListenAndServe(*addr, nil)

	for {
		start := time.Now()
		loadData()
		latency := time.Now().Sub(start)
		if latency < *sleep {
			time.Sleep(*sleep - latency)
		}
		fmt.Printf("%v\n", time.Now().Sub(start))
	}
}

func getField(obj map[string]interface{}, fields ...string) (interface{}, bool) {
	nextObj, found := obj[fields[0]]
	if !found {
		return nil, false
	}
	if len(fields) > 1 {
		return getField(nextObj.(map[string]interface{}), fields[1:]...)
	}
	return nextObj, true
}

func loadData() {
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error creating client config: %v", err)
		return
	}
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating client: %v", err)
		return
	}
	var labelSelector labels.Selector
	if *selector != "" {
		labelSelector, err = labels.Parse(*selector)
		if err != nil {
			fmt.Printf("Parse label selector err: %v", err)
			return
		}
	} else {
		labelSelector = labels.Everything()
	}
	pods, err := c.Core().Pods(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fields.Everything(),
	})
	if err != nil {
		fmt.Printf("Error getting pods: %v", err)
		return
	}
	instances := []*v1.Pod{}
	for ix := range pods.Items {
		pod := &pods.Items[ix]
		if pod.Status.PodIP == "" || pod.Status.Phase != "Running" {
			continue
		}
		instances = append(instances, pod)
	}
	response := Response{
		Servers: len(instances),
		Stats:   []Stat{},
	}
	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(instances))
	for ix := range instances {
		go func(ix int) {
			defer wg.Done()
			pod := instances[ix]
			var data []byte
			url := "http://" + pod.Status.PodIP + ":8080/api/stats"
			resp, err := httpClient.Get(url)
			if err != nil {
				fmt.Printf("Error getting: %v\n", err)
				return
			}
			defer resp.Body.Close()
			if data, err = ioutil.ReadAll(resp.Body); err != nil {
				fmt.Printf("Error reading: %v\n", err)
				return
			}

			var stat Stat
			if err := json.Unmarshal(data, &stat); err != nil {
				fmt.Printf("Error decoding: %v\n", err)
				return
			}
			lock.Lock()
			defer lock.Unlock()
			response.Stats = append(response.Stats, stat)
		}(ix)
	}
	wg.Wait()
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("Error marshaling: %v", err)
	}
	setData(data)
	fmt.Printf("Updated.\n")
}
