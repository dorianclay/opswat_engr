package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	gojsonq "github.com/thedevsaddam/gojsonq/v2"
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

	log.Println("done.")
	return res
}

func scanFile() *http.Response {
	// Even though we already read the file, we now need to open it
	// so it can be uploaded as a multi-part form
	log.Println("Checking file against Metadefender Cloud:")
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal("Error opening file\n", err)
	}

	// Initialize an empty buffer
	body := &bytes.Buffer{}
	// Create a new multipart writer with the buffer and the file
	writer := multipart.NewWriter(body)
	partial, err := writer.CreateFormFile("file", filename)
	if err != nil {
		log.Fatal("Error creating multipart writer form file\n", err)
	}

	// Copy the file into the writer, then close
	io.Copy(partial, file)
	writer.Close()

	// Create an HTTP post request to upload
	log.Print("Uploading file to Metadefender Cloud...")
	requestURL := fmt.Sprintf("%s", getPathURL("file"))
	req, err := http.NewRequest(http.MethodPost, requestURL, body)
	if err != nil {
		log.Fatal("Error generating http request\n", err)
	}
	// Set required headers (filename is optional for API, but recommended)
	req.Header.Set("apikey", apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("filename", filename)

	// Create a client with limited timeout (default is infinite)
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	// Do the request
	res, err := client.Do(req)
	if err != nil {
		log.Fatal("Error making http request\n", err)
	}

	log.Println("done.")

	// If the status code is 400, the request was likely incorrect...
	if res.StatusCode == 400 {
		resData := bodyToMap(res)
		log.Fatalf("Bad response, status: %s, got message: %s", res.Status, resData["error"])
		// ...or if the status isn't 200, then something went completely wrong.
	} else if res.StatusCode != 200 {
		log.Fatalf("Got unhandled response: %s", res.Status)
	}

	// ...otherwise the status code was 200, and upload was successful
	resData := bodyToMap(res)
	log.Printf("Upload successful. Spot %f in queue.\n", resData["in_queue"])

	log.Printf("Scanning in progress...\n")
	// Keep fetching the result until it is complete
	// Create the request
	requestURL = fmt.Sprintf("%s/%s", getPathURL("file"), resData["data_id"])
	req, err = http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		log.Fatal("Error generating http request\n", err)
	}
	req.Header.Set("apikey", apiKey)
	req.Header.Set("x-file-metadata", "0")

	// Do the request until complete
	//   Note that although a websocket could be used here so the program is not busy waiting,
	//   we use a simple loop so to avoid global IP setup headaches
	for {
		res, err = client.Do(req)
		if err != nil {
			log.Fatal("Error making http request\n", err)
		}

		// Query for the progress, if scan is done, stop fetching
		jsonq := gojsonq.New().FromInterface(bodyToMap(res))
		scanProgress := jsonq.Find("scan_results.progress_percentage")
		log.Printf("%f%% finished", scanProgress)
		if scanProgress == 100.0 {
			return res
		}
		// Wait a for 200ms
		time.Sleep(time.Millisecond * 200)
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Expected exactly one filename for the file to scan, got %d.", len(os.Args)-1)
	} else {
		filename = os.Args[1]
	}

	log.Println("Reading file:", filename)
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal("Error reading file\n", err)
	}

	res := hashLookup(content)

	log.Println("Got http response:", res.Status)
	if res.StatusCode == 404 {
		res = scanFile()
		log.Println("Got response status:", res.Status)
	} else if res.StatusCode == 200 {
		log.Println("Need to display results...")
	} else {
		log.Fatal("Got unhandled response:", res.Status)
	}
}
