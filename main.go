package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	gojsonq "github.com/thedevsaddam/gojsonq/v2"
)

const apiURL = "https://api.metadefender.com/v4"
const apiKey = ""

var filename string

// Generate the URL for a given API path
func getPathURL(path string) string {
	return fmt.Sprintf("%s/%s", apiURL, path)
}

// Generate a map from a given response body
func bodyToMap(resBody []byte) map[string]interface{} {
	// Unpack the body into a <string, interface> map
	var resData map[string]interface{}
	err := json.Unmarshal(resBody, &resData)
	if err != nil {
		log.Fatal("Failed to convert JSON to map\n", err)
	}

	// Return the map
	return resData
}

// Calculate the MD5 hash for a file and check if it has already been
// submitted to MetaDefender Cloud
func hashLookup() *http.Response {
	// Read the file
	log.Println("Reading file:", filename)
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal("Error reading file\n", err)
	}

	// Calculate the MD5 hash
	//   Note that the MD5 hash is used here because we are simply calculating a file
	//   hash to see if the file has already been scanned. Given that the hash is
	//   not being used for crytographic purposes, we optimize for the fastest hash
	//   supported by the API of {MD5, SHA1, SHA256}.
	log.Println("Generating MD5 hash...")
	hash := md5.Sum(content)
	log.Printf("Got hash: %x", hash)

	// Create an HTTP GET request to check if the file has already been scanned
	log.Println("Attempting to retrieve scan report with file hash...")
	requestURL := fmt.Sprintf("%s/%x", getPathURL("hash"), hash)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		log.Fatal("Error generating http request\n", err)
	}
	// Set required headers, here only apikey for authentication
	req.Header.Set("apikey", apiKey)

	// Create a client with limited timeout (default is infinite)
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	// Do the request
	res, err := client.Do(req)
	if err != nil {
		log.Fatal("Error making http request\n", err)
	}

	// Return the response no matter what it is
	return res
}

// Scan a file with the MetaDefender Cloud API
func scanFile() []byte {
	// Even though we already read the file, we now need to open it
	// so it can be uploaded as a multi-part form
	log.Println("Checking file against Metadefender Cloud:")
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Error opening file\n", err)
	}
	defer file.Close()

	// Initialize an empty buffer
	body := &bytes.Buffer{}
	// Create a new multipart writer with the buffer and file
	writer := multipart.NewWriter(body)
	partial, err := writer.CreateFormFile("file", filename)
	if err != nil {
		log.Fatal("Error creating multipart writer form file\n", err)
	}

	// Copy the file into the writer, then close
	io.Copy(partial, file)
	writer.Close()

	// Create an HTTP POST request to upload
	log.Print("Uploading file to Metadefender Cloud...")
	requestURL := getPathURL("file")
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

	// If the status code is 400, the request was likely incorrect...
	if res.StatusCode == 400 {
		// Read the response body
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal("Could not read response body\n", err)
		}
		resData := bodyToMap(resBody)
		log.Fatalf("Bad response, status: %s, got message: %s", res.Status, resData["error"])
		// ...or if the status isn't 200, then something went completely wrong.
	} else if res.StatusCode != 200 {
		log.Fatalf("Got unhandled response: %s", res.Status)
	}

	// ...otherwise the status code was 200, and upload was successful
	// Read the response body
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Could not read response body\n", err)
	}
	resData := bodyToMap(resBody)
	log.Printf("Upload successful. Spot %.0f in queue.\n", resData["in_queue"])

	// Keep fetching the result until it is complete
	log.Printf("Scanning in progress...\n")

	// Create the request to repeat until scanning is complete
	requestURL = fmt.Sprintf("%s/%s", getPathURL("file"), resData["data_id"])
	req, err = http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		log.Fatal("Error generating http request\n", err)
	}
	req.Header.Set("apikey", apiKey)
	req.Header.Set("x-file-metadata", "0")

	// Do the request until complete
	//   Note that although a websocket could be used here so the program is not busy waiting,
	//   we use a simple loop to avoid global IP setup headaches
	for {
		res, err = client.Do(req)
		if err != nil {
			log.Fatal("Error making http request\n", err)
		}

		// Query for the progress
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal("Could not read response body\n", err)
		}
		jsonq := gojsonq.New().FromInterface(bodyToMap(resBody))
		scanProgress := jsonq.Find("scan_results.progress_percentage")
		log.Printf("%.0f%% finished", scanProgress)

		// If the scan is done, return the response
		if scanProgress == 100.0 {
			return resBody
		}
		// Wait a for 500ms
		time.Sleep(time.Millisecond * 500)
	}
}

// Print the output of a successful response from MetaDefender Cloud API for file scanning
func printOutput(resBody []byte) {
	log.Println("Printing output to command line...")

	// Create a JSON Query object to get response attributes
	resData := bodyToMap(resBody)
	jsonq := gojsonq.New().FromInterface(resData)

	// Print the generic information
	fmt.Println("overall_status:", jsonq.Copy().Find("scan_results.scan_all_result_a"))

	// Get the list of details for all engines
	scanDetails := jsonq.Copy().Find("scan_results.scan_details").(map[string]interface{})
	// For each engine...
	for engine, data := range scanDetails {
		// ...create a new JSON query object
		jsonq = gojsonq.New().FromInterface(data)
		// ...and print all the information for the engine
		fmt.Println("engine:", engine)
		fmt.Println("threat_found:", jsonq.Copy().Find("threat_found"))
		fmt.Println("scan_result:", jsonq.Copy().Find("scan_result_i"))
		fmt.Println("def_time:", jsonq.Copy().Find("def_time"))
	}
}

// Main program entry point.
func main() {
	// Check that filename arguments are passed correctly
	if len(os.Args) != 2 {
		log.Fatalf("Expected exactly one filename for the file to scan, got %d.", len(os.Args)-1)
	} else {
		filename = os.Args[1]
	}

	// Check that API key has been set
	if apiKey == "" {
		log.Fatalf("Please modify the API key. Got: '%s'", apiKey)
	}

	// Open a file to log to
	logfile, err := os.OpenFile("filescan.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Println("Failed to open logging file. Logging to console instead.")
	} else {
		defer logfile.Close()
		log.SetOutput(logfile)
		log.Printf("SCANNING A NEW FILE...\n")
	}

	// Print the filename immediately to indicate program is running to user
	fmt.Println("filename:", filename)

	// Perform hash lookup
	res := hashLookup()

	log.Println("Got response:", res.Status)

	// If the file hash wasn't found...
	if res.StatusCode == 404 {
		// ...scan the file
		resBody := scanFile()
		printOutput(resBody)
		// ...otherwise, if successful, print the output
	} else if res.StatusCode == 200 {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal("Could not read response body\n", err)
		}
		printOutput(resBody)
		// ...otherwise, the status is undefined by the API
	} else {
		log.Fatal("Got unhandled response:", res.Status)
	}
}
