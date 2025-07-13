package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	JellyfinURL string `json:"jellyfin_url"`
	APIKey      string `json:"api_key"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
}

type Library struct {
	Name string
	Id   string
}

type User struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type Movie struct {
	Id              string  `json:"Id"`
	Name            string  `json:"Name"`
	ProductionYear  int     `json:"ProductionYear"`
	RunTimeTicks    int64   `json:"RunTimeTicks"`
	CommunityRating float64 `json:"CommunityRating"`
	Overview        string  `json:"Overview"`
}

type MovieResponse struct {
	Id          string  `json:"id"`
	Name        string  `json:"name"`
	Year        int     `json:"year"`
	Duration    int     `json:"duration"`
	Rating      float64 `json:"rating"`
	Overview    string  `json:"overview"`
	ImageUrl    string  `json:"imageUrl"`
	JellyfinUrl string  `json:"jellyfinUrl"`
}

var config Config
var libraries []Library

func noCacheHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		h.ServeHTTP(w, r)
	})
}

func loadConfig() error {
	configPath := "/app/data/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.json"
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &config)
}

func saveConfig(c Config) error {
	// Essayer d'abord dans le volume Docker
	configPath := "/app/data/config.json"
	dirPath := "/app/data"

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		configPath = "config.json"
		dirPath = "."
	}

	os.MkdirAll(dirPath, 0755)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func getUsers() ([]User, error) {
	url := config.JellyfinURL + "/Users"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Emby-Token", config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// Alternative version that checks the content of each library
func getLibraries() error {
	url := fmt.Sprintf("%s/Library/MediaFolders", config.JellyfinURL)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Emby-Token", config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	var result struct {
		Items []struct {
			Name           string `json:"Name"`
			Id             string `json:"Id"`
			CollectionType string `json:"CollectionType"`
		} `json:"Items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	libraries = nil
	for _, item := range result.Items {
		// Method 1: Filter by CollectionType
		if item.CollectionType == "movies" {
			libraries = append(libraries, Library{
				Name: item.Name,
				Id:   item.Id,
			})
			continue
		}

		if item.CollectionType == "" {
			hasMovies, err := checkLibraryHasMovies(item.Id)
			if err == nil && hasMovies {
				libraries = append(libraries, Library{
					Name: item.Name,
					Id:   item.Id,
				})
			}
		}
	}
	return nil
}

func checkLibraryHasMovies(libraryId string) (bool, error) {
	url := fmt.Sprintf("%s/Users/%s/Items?ParentId=%s&IncludeItemTypes=Movie&Recursive=true&Limit=1",
		config.JellyfinURL, config.UserID, libraryId)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Emby-Token", config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Items            []Movie `json:"Items"`
		TotalRecordCount int     `json:"TotalRecordCount"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	return result.TotalRecordCount > 0, nil
}

func getRandomMovie(libraryId string) (*Movie, error) {
	url := fmt.Sprintf("%s/Users/%s/Items?ParentId=%s&IncludeItemTypes=Movie&Recursive=true&Fields=ProductionYear,RunTimeTicks,CommunityRating,Overview&StartIndex=0&Limit=1000",
		config.JellyfinURL, config.UserID, libraryId)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Emby-Token", config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Items []Movie `json:"Items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no movies found")
	}

	randomIndex := rand.Intn(len(result.Items))
	return &result.Items[randomIndex], nil
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/setup", setupHandler)
	http.HandleFunc("/random", randomHandler)
	http.Handle("/static/", noCacheHandler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))))

	log.Printf("-------------------------")
	log.Println("Server started on :8080")
	log.Printf("-------------------------")
	http.ListenAndServe(":8080", nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if err := loadConfig(); err != nil {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	if config.UserID == "" {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	if len(libraries) == 0 {
		if err := getLibraries(); err != nil {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, struct{ Libraries []Library }{libraries})
}

func setupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if userID := r.FormValue("user_id"); userID != "" {
			config.UserID = userID
			config.UserName = r.FormValue("user_name")
			if err := saveConfig(config); err != nil {
				http.Error(w, "Error saving config", 500)
				return
			}

			if err := getLibraries(); err != nil {
				http.Error(w, "Error retrieving libraries", 500)
				return
			}

			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		jellyfinURL := r.FormValue("jellyfin_url")
		apiKey := r.FormValue("api_key")

		if jellyfinURL == "" || apiKey == "" {
			http.Error(w, "URL and API key required", 400)
			return
		}

		jellyfinURL = strings.TrimSuffix(jellyfinURL, "/")
		config.JellyfinURL = jellyfinURL
		config.APIKey = apiKey

		if err := saveConfig(config); err != nil {
			http.Error(w, "Error saving config", 500)
			return
		}

		users, err := getUsers()
		if err != nil {
			http.Error(w, "Unable to connect to Jellyfin. Please check the URL and API key.", 500)
			return
		}

		tmpl := template.Must(template.ParseFiles("templates/setup.html"))
		tmpl.Execute(w, struct {
			Users []User
			Step  string
		}{users, "user_selection"})
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/setup.html"))
	tmpl.Execute(w, struct{ Step string }{"initial"})
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func randomHandler(w http.ResponseWriter, r *http.Request) {
	libraryId := r.URL.Query().Get("library")
	if libraryId == "" {
		http.Error(w, "Library ID required", 400)
		return
	}

	clientIP := getClientIP(r)
	userAgent := r.UserAgent()

	movie, err := getRandomMovie(libraryId)
	if err != nil {
		http.Error(w, "Error retrieving movie", 500)
		return
	}

	green := "\033[32m"
	yellow := "\033[33m"
	cyan := "\033[36m"
	blue := "\033[34m"
	reset := "\033[0m"

	log.Printf(
		"%sSuggested movie:%s %s%s%s | %sClient IP:%s %s | %sUser-Agent:%s %s",
		green, reset, yellow, movie.Name, reset,
		cyan, reset, clientIP,
		blue, reset, userAgent,
	)

	duration := int(movie.RunTimeTicks / 10000000 / 60)
	imageUrl := fmt.Sprintf("%s/Items/%s/Images/Primary", config.JellyfinURL, movie.Id)
	jellyfinMovieUrl := fmt.Sprintf("%s/web/index.html#!/details?id=%s", config.JellyfinURL, movie.Id)

	response := MovieResponse{
		Id:          movie.Id,
		Name:        movie.Name,
		Year:        movie.ProductionYear,
		Duration:    duration,
		Rating:      movie.CommunityRating,
		Overview:    movie.Overview,
		ImageUrl:    imageUrl,
		JellyfinUrl: jellyfinMovieUrl,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Error encoding response", 500)
		return
	}
}
