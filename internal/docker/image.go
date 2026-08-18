package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ImageInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	IsOfficial  bool   `json:"is_official"`
}

type dockerHubResponse struct {
	Results []hubResult `json:"results"`
}

type hubResult struct {
	RepoName         string `json:"repo_name"`
	ShortDescription string `json:"short_description"`
	StarCount        int    `json:"star_count"`
	IsOfficial       bool   `json:"is_official"`
}

func (dm *DockerManager) SearchImages(query string) ([]ImageInfo, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/search/repositories/?query=%s", query)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con Docker Hub: %w", err)
	}
	defer resp.Body.Close()

	var hubData dockerHubResponse
	if err := json.NewDecoder(resp.Body).Decode(&hubData); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta de Docker Hub: %w", err)
	}

	var images []ImageInfo
	for _, res := range hubData.Results {
		images = append(images, ImageInfo{
			Name:        res.RepoName,
			Description: res.ShortDescription,
			Stars:       res.StarCount,
			IsOfficial:  res.IsOfficial,
		})
	}

	return images, nil
}