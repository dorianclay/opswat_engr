package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

const apiURL = "https://api.metadefender.com/v4"
const apiKey = "6bee20afb8e4dc522b9d931f4490b540"

var filename string

func getPathURL(path string) string {
	return fmt.Sprintf("%s/%s", apiURL, path)
}

func bodyToMap(res *http.Response) map[string]interface{} {
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Could not read response body\n", err)
	}
	var resData map[string]interface{}
	err = json.Unmarshal(resBody, &resData)
	if err != nil {
		log.Fatal("Failed to convert JSON to map\n", err)
	}

	return resData
}

func hashLookup(content []byte) *http.Response {
	print("Generating MD5 hash...")
	hash := md5.Sum(content)
	fmt.Printf("got: %x", hash)

	print("\nAttempting to retrieve scan report with file hash...")
	requestURL := fmt.Sprintf("%s/%x", getPathURL("hash"), hash)
	// fmt.Println("Got requestURL:", requestURL)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		log.Fatal("Error generating http request\n", err)
	}
	req.Header.Set("apikey", apiKey)

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		log.Fatal("Error making http request\n", err)
	}

	println("done.")
	return res
}

func scanFile(content []byte) *http.Response {
	println("Checking file against Metadefender Cloud:")
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal("Error opening file\n", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()
	_, err = writer.CreateFormFile("file", filename)
	if err != nil {
		log.Fatal("Error creating multipart writer form file\n", err)
	}

	println("Uploading file to Metadefender Cloud...")

	requestURL := fmt.Sprintf("%s", getPathURL("file"))
	// fmt.Println("Got requestURL:", requestURL)
	// bodyReader := bytes.NewReader(content)
	req, err := http.NewRequest(http.MethodPost, requestURL, body)
	if err != nil {
		log.Fatal("Error generating http request\n", err)
	}
	req.Header.Set("apikey", apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("filename", filename)

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		log.Fatal("Error making http request\n", err)
	}

	println("done.")

	if res.StatusCode == 400 {
		resData := bodyToMap(res)
		log.Fatalf("Bad response, status: %s, got message: %s", res.Status, resData["messages"])
	} else if res.StatusCode == 200 {
		println("Successfully uploaded file.")
	} else {
		log.Fatalf("Got unhandled response: %s", res.Status)
	}
	return res
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Expected exactly one filename for the file to scan, got %d.", len(os.Args)-1)
	} else {
		filename = os.Args[1]
	}

	println("Reading file:", filename)
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal("Error reading file\n", err)
	}

	res := hashLookup(content)

	println("Got http response:", res.Status)
	if res.StatusCode == 404 {
		res = scanFile(content)
		println("Got response status:", res.Status)
	} else if res.StatusCode == 200 {
		println("Need to display results...")
	} else {
		log.Fatal("Got unhandled response:", res.Status)
	}
}
