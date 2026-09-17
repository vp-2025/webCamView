package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
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

type ViewPageData struct {
	CameraImage
	Date  string
	Dates []string
}

var dateFolderSuffix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func parseDateParam(date string) (string, bool) {
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return "", false
		}
	}
	return date, true
}

func cameraDir(cam, date string) string {
	if date != "" {
		return filepath.Join(basePath, cam+"_"+date)
	}
	return filepath.Join(basePath, cam)
}

func getCameraDates(cam string) (dates []string) {
	if _, err := os.Stat(cameraDir(cam, "")); err == nil {
		dates = append(dates, "")
	}

	prefix := cam + "_"
	var archived []string
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return dates
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		datePart := strings.TrimPrefix(name, prefix)
		if !dateFolderSuffix.MatchString(datePart) {
			continue
		}
		if _, err := os.Stat(filepath.Join(basePath, name)); err != nil {
			continue
		}
		archived = append(archived, datePart)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(archived)))
	dates = append(dates, archived...)
	return dates
}

func getImageFiles(cam, date string) (files []string) {
	dir := cameraDir(cam, date)
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

func imageQuery(cam, date, fileName string) string {
	q := url.Values{}
	q.Set("act", "image")
	q.Set("cam", cam)
	q.Set("foto", fileName)
	if date != "" {
		q.Set("date", date)
	}
	return "?" + q.Encode()
}

func getLatestImages() (images []CameraImage) {
	for _, cam := range cameras {
		files := getImageFiles(cam, "")
		if len(files) == 0 {
			continue
		}
		latest := files[len(files)-1]
		images = append(images, CameraImage{
			Camera:   cam,
			FileName: latest,
			Time:     formatTime(latest),
			Path:     imageQuery(cam, "", latest),
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
		handleImage(w, r, values.Get("cam"), values.Get("date"), values.Get("foto"))
	case "list":
		handleList(w, r, values.Get("cam"), values.Get("date"))
	case "view":
		handleView(w, r, values.Get("cam"), values.Get("date"), values.Get("idx"))
	default:
		handleIndex(w)
	}
}

func handleView(w http.ResponseWriter, r *http.Request, camera, dateParam, idxStr string) {
	if slices.Index(cameras, camera) == -1 {
		http.NotFound(w, r)
		return
	}
	date, ok := parseDateParam(dateParam)
	if !ok {
		http.NotFound(w, r)
		return
	}
	dates := getCameraDates(camera)
	if date != "" && !slices.Contains(dates, date) {
		http.NotFound(w, r)
		return
	}
	files := getImageFiles(camera, date)
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
	page := ViewPageData{
		CameraImage: CameraImage{
			Camera:   camera,
			FileName: fileName,
			Time:     formatTime(fileName),
			Path:     imageQuery(camera, date, fileName),
			Count:    len(files),
			Index:    idx,
		},
		Date:  date,
		Dates: dates,
	}
	tmpl, err := template.ParseFS(templateFS, "templates/view.html")
	if err != nil {
		log.Printf("Failed to parse view template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "view.html", page); err != nil {
		log.Printf("View template error: %v", err)
	}
}

func handleList(w http.ResponseWriter, r *http.Request, camera, dateParam string) {
	if slices.Index(cameras, camera) == -1 {
		http.NotFound(w, r)
		return
	}
	date, ok := parseDateParam(dateParam)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if date != "" && !slices.Contains(getCameraDates(camera), date) {
		http.NotFound(w, r)
		return
	}
	files := getImageFiles(camera, date)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func handleImage(w http.ResponseWriter, r *http.Request, camera, dateParam, fileName string) {
	if slices.Index(cameras, camera) == -1 {
		http.NotFound(w, r)
		return
	}
	date, ok := parseDateParam(dateParam)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if date != "" && !slices.Contains(getCameraDates(camera), date) {
		http.NotFound(w, r)
		return
	}

	if fileName == "" || strings.Contains(fileName, "..") || strings.ContainsAny(fileName, `/\`) {
		http.NotFound(w, r)
		return
	}
	filePath := filepath.Join(cameraDir(camera, date), fileName)
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
