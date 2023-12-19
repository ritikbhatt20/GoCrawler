package main

import (
    "encoding/csv"
    "fmt"
    "log"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/PuerkitoBio/goquery"
)

// ScrapedData represents the data to be stored
type ScrapedData struct {
    URL     string
    Content string
}

var (
    db          []ScrapedData
    dbLock      sync.Mutex
    requestRate = time.Second / 2         // Adjust the rate as needed
    tokenBucket = make(chan struct{}, 10) // Adjust the bucket size as needed
)

// scrapeURL scrapes a webpage, finds links, and runs the same function for each link
func scrapeURL(url, keyword string, wg *sync.WaitGroup) {
    defer wg.Done()
    // Acquire token from the bucket (rate limiter)
    tokenBucket <- struct{}{}
    defer func() { <-tokenBucket }()

    // Make an HTTP request to the URL
    response, err := http.Get(url)
    if err != nil {
        log.Printf("Error while making request to %s: %v", url, err)
        return
    }
    defer response.Body.Close()

    if response.StatusCode != http.StatusOK {
        log.Printf("Failed to retrieve %s: %s", url, response.Status)
        return
    }

    // Parse the HTML content of the page
    doc, err := goquery.NewDocumentFromReader(response.Body)
    if err != nil {
        log.Printf("Error while parsing HTML of %s: %v", url, err)
        return
    }

    // Check if the keyword is present in the page content
    if strings.Contains(doc.Text(), keyword) {
        // Store the data in the database
        dbLock.Lock()
        db = append(db, ScrapedData{URL: url, Content: doc.Text()})
        dbLock.Unlock()

        // Save data to CSV file
        saveToCSV(url, doc.Text())
    }

    // Find all links on the page
    doc.Find("a").Each(func(i int, s *goquery.Selection) {
        if href, exists := s.Attr("href"); exists {
            fullURL := href
            if !strings.HasPrefix(href, "http") {
                fullURL = url + "/" + href
            }
            wg.Add(1)
            go scrapeURL(fullURL, keyword, wg)
        }
    })
}

// saveToCSV saves the data to a CSV file
func saveToCSV(url, content string) {
    fileName := getFileName(url)

    // Create or open the file for writing
    file, err := os.Create(fileName)
    if err != nil {
        log.Printf("Error creating file %s: %v", fileName, err)
        return
    }
    defer file.Close()

    // Create a CSV writer
    writer := csv.NewWriter(file)
    defer writer.Flush()

    // Write data to CSV
    err = writer.Write([]string{"URL", "Content"})
    if err != nil {
        log.Printf("Error writing header to CSV file %s: %v", fileName, err)
        return
    }

    err = writer.Write([]string{url, content})
    if err != nil {
        log.Printf("Error writing data to CSV file %s: %v", fileName, err)
        return
    }
}

// getFileName generates a unique filename based on the URL
func getFileName(url string) string {
    // Replace non-alphanumeric characters in URL to create a valid filename
    replacer := strings.NewReplacer("https://", "", "http://", "", "/", "_", ".", "_")
    return fmt.Sprintf("%s.csv", replacer.Replace(url))
}

func main() {
    startURL := "https://wordpress.com"
    keywordToFind := "example"

    var wg sync.WaitGroup
    wg.Add(1)
    go scrapeURL(startURL, keywordToFind, &wg)
    wg.Wait()

    // Display the scraped data
    for _, data := range db {
        fmt.Printf("URL: %s\nContent: %s\n\n", data.URL, data.Content)
    }
}