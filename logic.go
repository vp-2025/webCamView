package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

var cameras = []string{"cam3", "cam4", "cam2", "cam5"}

type CameraImage struct {
	Camera   string `json:"camera"`
	FileName string `json:"fileName"`
	Time     string `json:"time"`
	Path     string `json:"path"`
	Count    int    `json:"count"`
	Index    int    `json:"index"`
}

func getImageFiles(cam string) (files []string) {
	dir := filepath.Join(basePath, cam)
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Error reading directory %s: %v", dir, err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return
}

func formatTime(fileName string) string {
	fTime := ""
	//	if stat, err := os.Stat(filepath.Join(basePath, cam, fileName)); err == nil {
	//		fTime = stat.ModTime().Format("2006-01-02 15:04:05")
	//	}
	fNoExt := strings.TrimSuffix(strings.ToUpper(fileName), ".JPG")
	v := strings.Split(fNoExt, "_")
	if len(v) > 1 {
		fTime += " " + v[len(v)-1]
	}
	return fTime
}

func getLatestImages() (images []CameraImage) {
	for _, cam := range cameras {
		files := getImageFiles(cam)
		if len(files) == 0 {
			continue
		}
		latest := files[len(files)-1]
		images = append(images, CameraImage{
			Camera:   cam,
			FileName: latest,
			Time:     formatTime(latest),
			Path:     fmt.Sprintf("?act=image&cam=%s&foto=%s", cam, latest),
			Count:    len(files),
			Index:    len(files),
		})
	}
	return images
}

func handleWeb(w http.ResponseWriter, r *http.Request) {
	log.Printf("- %s: %s", r.Header.Get("Remote-Addr"), r.URL.String())

	values := r.URL.Query()
	switch values.Get("act") {
	case "image":
		handleImage(w, r, values.Get("cam"), values.Get("foto"))
	case "list":
		handleList(w, r, values.Get("cam"))
	case "view":
		handleView(w, r, values.Get("cam"), values.Get("idx"))
	default:
		handleIndex(w)
	}
}

func handleView(w http.ResponseWriter, r *http.Request, camera, idxStr string) {
	if slices.Index(cameras, camera) == -1 {
		http.NotFound(w, r)
		return
	}
	files := getImageFiles(camera)
	if len(files) == 0 {
		http.NotFound(w, r)
		return
	}
	idx := len(files)
	if idxStr != "" {
		if parsed, err := strconv.Atoi(idxStr); err == nil {
			idx = parsed
		}
	}
	if idx < 1 {
		idx = 1
	}
	if idx > len(files) {
		idx = len(files)
	}
	fileName := files[idx-1]
	img := CameraImage{
		Camera:   camera,
		FileName: fileName,
		Time:     formatTime(fileName),
		Path:     "?act=image&cam=" + url.QueryEscape(camera) + "&foto=" + url.QueryEscape(fileName),
		Count:    len(files),
		Index:    idx,
	}
	tmpl, err := template.ParseFS(templateFS, "templates/view.html")
	if err != nil {
		log.Printf("Failed to parse view template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "view.html", img); err != nil {
		log.Printf("View template error: %v", err)
	}
}

func handleList(w http.ResponseWriter, r *http.Request, camera string) {
	if slices.Index(cameras, camera) == -1 {
		http.NotFound(w, r)
		return
	}
	files := getImageFiles(camera)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func handleImage(w http.ResponseWriter, r *http.Request, camera, fileName string) {
	if slices.Index(cameras, camera) == -1 {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Clean(filepath.Join(basePath, camera, fileName))
	http.ServeFile(w, r, filePath)
}

func handleIndex(w http.ResponseWriter) {
	tmpl, err := template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		log.Printf("Failed to parse index templates: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.ExecuteTemplate(w, "index.html", getLatestImages())
	if err != nil {
		log.Printf("Index template error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
