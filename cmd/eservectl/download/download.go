package download

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

// https://gist.github.com/albulescu/e61979cc852e4ee8f49c but better

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func PrintDownloadPercent(done chan int64, path string, total int64) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	prevSize, prevTime, speed := int64(0), time.Now(), 0.0

	// renders one line in place via \r; called every tick and once more on done
	render := func() {
		fi, err := file.Stat()
		if err != nil {
			log.Fatal(err)
		}

		size, now := fi.Size(), time.Now()

		// bytes since last tick -> speed
		if dt := now.Sub(prevTime).Seconds(); dt > 0 && size >= prevSize {
			speed = float64(size-prevSize) / dt
		}
		prevSize, prevTime = size, now

		var eta string
		if speed > 0 {
			eta = (time.Duration(float64(total-size)/speed) * time.Second).Round(time.Second).String()
		} else {
			eta = "-"
		}

		var percent string
		if total > 0 {
			percent = fmt.Sprintf("%.0f%%", float64(size)/float64(total)*100)
		} else {
			percent = "?"
		}

		fmt.Printf("\r%s  %s / %s  %.1f MiB/s  ETA %s",
			percent, humanBytes(size), humanBytes(total), speed/(1024*1024), eta)
	}

	for {
		select {
		case <-done:
			render()
			fmt.Println()
			return
		default:
			render()
		}
		time.Sleep(time.Second)
	}
}

func DownloadFile(url string, dest string) (string, error) {
	file := path.Base(url)
	log.Printf("Downloading file %s from %s\n", file, url)

	fullPath := filepath.Join(dest, file)
	start := time.Now()
	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// avoid panic here from -1 total (from server header)
	total := int64(0)
	if resp, err := http.Head(url); err == nil {
		total = resp.ContentLength
	}

	done := make(chan int64)
	go PrintDownloadPercent(done, fullPath, total)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	done <- n
	elapsed := time.Since(start)
	log.Printf("Download completed in %s", elapsed)
	return file, nil
}
