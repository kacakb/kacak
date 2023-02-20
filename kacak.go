package kacak

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func main() {
	var fileName, outputFileName, regexFileName string
	flag.StringVar(&fileName, "l", "jsfiles.txt", "filename to read URLs from")
	flag.StringVar(&outputFileName, "o", "output", "filename to write the output to")
	flag.StringVar(&regexFileName, "r", "regex.txt", "filename to read the regex from")
	flag.Parse()

	// Read URLs from file
	urlsFile, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}
	defer urlsFile.Close()

	urls := make([]string, 0)
	scanner := bufio.NewScanner(urlsFile)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	// Read regex from file
	regexFile, err := os.Open(regexFileName)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}
	defer regexFile.Close()

	regexScanner := bufio.NewScanner(regexFile)
	if !regexScanner.Scan() {
		fmt.Println("No regex found in file.")
		return
	}
	regexString := regexScanner.Text()

	// Configure HTTPS client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{Transport: tr, Timeout: 60 * time.Second}

	// Process URLs
	var wg sync.WaitGroup
	var outputMutex sync.Mutex
	output := make([]string, 0)
	const maxConcurrent = 10 // aynı anda en fazla kaç URL işlenecek
	const waitSeconds = 5    // her işlem arasında kaç saniye beklenecek
	sem := make(chan struct{}, maxConcurrent)

	for _, url := range urls {
		if url == "" {
			continue
		}
		sem <- struct{}{} // semafora giriş yapılıyor
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			defer func() {
				<-sem                                                // semafor çıkışı yapılıyor
				time.Sleep(time.Duration(waitSeconds) * time.Second) // bekleniyor
			}()

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				fmt.Printf("Error creating GET request: %v\n", err)
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.190 Safari/537.36")

			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Error getting GET request: %v\n", err)
				return
			}
			defer resp.Body.Close()

			if resp == nil {
				fmt.Printf("Error response is nil")
				return
			}

			if resp.StatusCode == http.StatusOK {
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					fmt.Printf("Error reading response body: %v\n", err)
					return
				}

				sensitiveRegex := regexp.MustCompile(regexString)
				matches := sensitiveRegex.FindAllString(string(body), -1)
				if len(matches) > 0 {
					for _, match := range matches {
						fmt.Printf("Found url: %s --> %s\n", url, match)
						outputMutex.Lock()
						output = append(output, fmt.Sprintf("Found url: %s --> %s", url, match))
						outputMutex.Unlock()
					}
				}
			}
		}(url)
	}

	wg.Wait()

	if outputFileName != "" {
		err := ioutil.WriteFile(outputFileName, []byte(strings.Join(output, "\n")), 0644)
		if err != nil {
			fmt.Printf("Error writing to output file: %v\n", err)
			return
		}
	}
}
