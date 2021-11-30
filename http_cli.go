package mydb

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

var client = &http.Client{}

func doGet(urlStr string) ([]byte, error) {
	method := "GET"

	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println(string(body))

	return body, nil
}

func doPost(urlStr string, payloadStr string) ([]byte, error) {
	method := "POST"

	payload := strings.NewReader(payloadStr)

	req, err := http.NewRequest(method, urlStr, payload)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Authorization", "Basic cm9vdDp0YW9zZGF0YQ==")
	req.Header.Add("Content-Type", "text/plain")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println(string(body))

	return body, nil
}
