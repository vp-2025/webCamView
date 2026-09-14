package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var renameLog = log.New(os.Stderr, "", log.Ldate|log.Ltime)

func initRenameLogging(logFile string, bStdout bool) {
	var w io.Writer
	if bStdout {
		w = os.Stdout
	} else {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file %s: %v", logFile, err)
			return
		}
		w = f
	}
	renameLog = log.New(w, "", log.Ldate|log.Ltime)
}

func renameDir(src, dst string) {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		renameLog.Printf("  %s: not found, skipping", filepath.Base(src))
		return
	}

	for i := range 60 {
		err := os.Rename(src, dst)
		if err == nil {
			renameLog.Printf("  %s -> %s: ok", filepath.Base(src), filepath.Base(dst))
			return
		}
		renameLog.Printf("  WARNING: %s: rename attempt %d failed: %v", filepath.Base(src), i+1, err)

		select {
		case <-time.After(30 * time.Second):
		}
	}
	renameLog.Printf("  ERROR: %s: gave up after 60 attempts", filepath.Base(src))
}

func rename() {
	yesterday := time.Now().AddDate(0, 0, -1)
	sDate := "_" + yesterday.Format("2006-01-02")

	for _, sCamN := range cameras {
		renameDir(filepath.Join(basePath, sCamN), filepath.Join(basePath, sCamN+sDate))
	}
}

func runRenameLoop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	tm := time.Now()
	renameLog.Print("rename loop started")

	for {
		now := time.Now()

		if tm.YearDay() != now.YearDay() || tm.Year() != now.Year() {
			tm = now
			rename()
			renameLog.Print("renamed")
		}

		renameLog.Println(".")
		nextHour := now.Truncate(time.Hour).Add(time.Hour)
		d := nextHour.Sub(now)

		select {
		case <-stop:
			renameLog.Println("rename loop stopping")
			return
		case <-time.After(d):
		}
	}
}
