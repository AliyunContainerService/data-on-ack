/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package utils

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/klog"
)

func IsDomainNameAvailable(domain string) bool {
	if domain == "" {
		return false
	}
	ips, err := net.LookupIP(domain)
	if err != nil || ips == nil || len(ips) < 1 {
		log.Infof("loop up ip for domain err:%s", err)
		return false
	}
	return true
}

// getClient is get a default httpClient
func getClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
		DisableCompression: true,
		DisableKeepAlives:  true,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}
	return client
}

// RequestWithPost is a http client with url, header and params
func RequestWithPost(reqUrl string, header map[string]string, params map[string]string) (status int, body string, err error) {
	return RequestWithHeader(http.MethodPost, reqUrl, header, params)
}

// RequestWithHeader is a http client with header, url, and params
func RequestWithHeader(method string, reqUrl string, header map[string]string, params map[string]string) (status int, body string, err error) {

	client := getClient()
	paramsA := ""
	if method == http.MethodPost {
		values := url.Values{}
		for k, v := range params {
			values.Add(k, v)
		}
		paramsA = values.Encode()
	}

	req, _ := http.NewRequest(method, reqUrl, strings.NewReader(paramsA))

	for k, v := range header {
		req.Header.Set(k, v)
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if method == http.MethodGet {
		values := req.URL.Query()
		for k, v := range params {
			values.Add(k, v)
		}
		req.URL.RawQuery = values.Encode()
	}

	getResp, err := client.Do(req)
	if err != nil {
		return status, body, err
	}
	defer getResp.Body.Close()
	status = getResp.StatusCode
	getBody, err := ioutil.ReadAll(getResp.Body)
	if err != nil {
		return status, body, err
	}

	decodeBytes, err := url.QueryUnescape(string(getBody))
	if err != nil {
		return status, body, err
	}
	return status, string(decodeBytes), nil
}

func NewKubeflowProxy() (*httputil.ReverseProxy, error) {
	targetHost := "http://ml-pipeline-ui"
	url, err := url.Parse(targetHost)
	if err != nil {
		klog.Error("fail to parse url:" + targetHost)
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(url), nil
}

func NewMlflowProxy() (*httputil.ReverseProxy, error) {
	rawURL := "http://mlflow:5000"
	url, err := url.Parse(rawURL)
	if err != nil {
		klog.Error("failed to prase url: ", rawURL)
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(url), nil
}
