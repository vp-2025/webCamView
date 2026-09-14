package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kardianos/service"
)

const sPort = ":8088"

func setupFileLogging(logFile string, bStdout bool) {
	if bStdout {
		log.SetOutput(os.Stdout)
	} else {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file %s: %v", logFile, err)
			return
		}
		log.SetOutput(f)
	}
	log.SetFlags(log.Ldate | log.Ltime) //  | log.Lshortfile
}

type program struct {
	server     *http.Server
	stop, done chan struct{}
}

func (p *program) init() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleWeb)
	p.server = &http.Server{
		Addr:              sPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func (p *program) run() {
	log.Printf("HTTP server listening on %s", sPort)
	if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP server error: %v", err)
	}
}

func (p *program) Start(_ service.Service) error {
	log.Println("Service starting...")
	p.stop, p.done = make(chan struct{}), make(chan struct{})
	p.init()
	go p.run()
	go runRenameLoop(p.stop, p.done)
	return nil
}

func (p *program) Stop(_ service.Service) error {
	log.Println("Service stopping...")
	if p.stop != nil {
		close(p.stop)
		<-p.done
	}
	if p.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.server.Shutdown(ctx)
}

func main() {
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop, run")
	flag.Parse()

	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	logFile := strings.TrimSuffix(exePath, filepath.Ext(exePath)) + ".log"
	setupFileLogging(logFile, service.Interactive())

	renameLogFile := filepath.Join(filepath.Dir(exePath), "webCamDate.log")
	initRenameLogging(renameLogFile, service.Interactive())

	log.Print("base path: ", basePath)

	svcConfig := &service.Config{
		Name:        "vCamView",
		DisplayName: "vCam View",
		Description: "vp"}

	prg := &program{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(*svcFlag) != 0 {
		err = service.Control(svc, *svcFlag)
		if err != nil {
			log.Fatalf("valid actions: install, uninstall, start, stop, run\n%v", err)
		}
		fmt.Printf("service action '%s' completed successfully\n", *svcFlag)
		return
	}

	err = svc.Run()
	if err != nil {
		log.Fatal(err)
	}
}
