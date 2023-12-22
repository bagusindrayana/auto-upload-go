package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	serverURL       string
	uploadDirectory string
	logFile         string
	method          string
	headers         string
	bodyData        string
)

func init() {
	flag.StringVar(&serverURL, "server-url", "http://example.com/upload", "Server URL for file upload")
	flag.StringVar(&uploadDirectory, "upload-dir", "/path/to/upload/directory", "Directory to watch for new files")
	flag.StringVar(&logFile, "log-file", "/path/to/logfile.log", "Log file path")
	flag.StringVar(&method, "method", "POST", "HTTP method for file upload")
	flag.StringVar(&headers, "headers", "", "Headers to include in the request, formatted as 'key1:value1,key2:value2'")
	flag.StringVar(&bodyData, "body", "", "JSON data to include in the request body")
}

func main() {
	flag.Parse()

	// Setup logrus
	logrus.SetFormatter(&logrus.TextFormatter{})
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		logrus.SetOutput(io.MultiWriter(os.Stdout, file))
	} else {
		logrus.Info("Failed to log to file, using default stderr")
	}

	for {
		watchForNewFiles(uploadDirectory)
		time.Sleep(1 * time.Second)
	}

}

func watchForNewFiles(directory string) {
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			uploadFile(path)
		}

		return nil
	})

	if err != nil {
		logrus.Error("Error walking through the directory:", err)
	}
}

func uploadFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		logrus.Error("Error opening file:", err)
		return
	}
	defer file.Close()

	// Check if the file has already been uploaded
	if isFileUploaded(filePath) {
		// logrus.Infof("File already uploaded: %s", filePath)
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create form field for file upload
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		logrus.Error("Error creating form file:", err)
		return
	}

	// Copy file content to form field
	_, err = io.Copy(part, file)
	if err != nil {
		logrus.Error("Error copying file content:", err)
		return
	}

	// Add additional form fields
	if bodyData != "" {
		var jsonData map[string]interface{}
		if err := json.Unmarshal([]byte(bodyData), &jsonData); err != nil {
			logrus.Error("Error parsing JSON data:", err)
			return
		}
		fmt.Println(jsonData)
		for key, value := range jsonData {
			writer.WriteField(key, fmt.Sprintf("%v", value))
		}
	}

	// Close the multipart writer
	err = writer.Close()
	if err != nil {
		logrus.Error("Error closing multipart writer:", err)
		return
	}

	// Perform the upload
	client := &http.Client{}
	req, err := http.NewRequest(method, serverURL, body)
	if err != nil {
		logrus.Error("Error creating request:", err)
		return
	}

	// Set Content-Type header for multipart/form-data
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add headers to the request
	if headers != "" {
		headerList := strings.Split(headers, ",")
		for _, header := range headerList {
			keyValue := strings.SplitN(header, ":", 2)
			if len(keyValue) == 2 {
				req.Header.Add(strings.TrimSpace(keyValue[0]), strings.TrimSpace(keyValue[1]))
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		logrus.Error("Error uploading file:", err)
		return
	}
	defer resp.Body.Close()

	// print response body
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	fmt.Println(buf.String())

	// Check if the upload was successful (you may need to customize this based on your server response)
	if resp.StatusCode == http.StatusOK {
		logrus.Infof("File uploaded successfully: %s", filePath)

		// Log that the file has been uploaded to avoid re-uploading
		logUploadedFile(filePath)
	} else {
		logrus.Errorf("Failed to upload file: %s, Status: %s", filePath, resp.Status)
	}
}

func isFileUploaded(filePath string) bool {
	// fmt.Println(filePath)
	// Read the log file
	logEntries, err := readLogFile(logFile)
	if err != nil {
		logrus.Error("Error reading log file:", err)
		return false
	}

	// Check if the file path exists in the log entries
	for _, entry := range logEntries {
		if strings.Contains(entry, filePath) {
			return true
		}
	}

	return false
}

func logUploadedFile(filePath string) {
	// Log the file path and upload timestamp to a log file
	logEntry := fmt.Sprintf("%s - %s\n", time.Now().Format(time.RFC3339), filePath)
	file, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		logrus.Error("Error opening log file:", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(logEntry); err != nil {
		logrus.Error("Error writing to log file:", err)
	}
}

func readLogFile(logFilePath string) ([]string, error) {
	var logEntries []string

	file, err := os.Open(logFilePath)
	if err != nil {
		return logEntries, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		logEntries = append(logEntries, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		return logEntries, err
	}

	return logEntries, nil
}
