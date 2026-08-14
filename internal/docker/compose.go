package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"gopkg.in/yaml.v3"
)

const BasePath = "/stacks"

// --- Estructuras para parsear el docker-compose.yml ---
type ComposeConfig struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	Ports []string `yaml:"ports"`
}

// GetProjectFiles busca qué archivos de configuración existen
func (dm *DockerManager) GetProjectFiles(projectName string) ([]string, error) {
	projectPath := filepath.Join(BasePath, projectName)
	targets := []string{"docker-compose.yml", "docker-compose.yaml", ".env", "Dockerfile"}
	var foundFiles []string

	for _, target := range targets {
		fullPath := filepath.Join(projectPath, target)
		if _, err := os.Stat(fullPath); err == nil {
			foundFiles = append(foundFiles, target)
		}
	}
	return foundFiles, nil
}

func (dm *DockerManager) ReadFile(projectName, fileName string) (string, error) {
	fullPath := filepath.Join(BasePath, projectName, fileName)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("error al leer el archivo %s: %w", fileName, err)
	}
	return string(content), nil
}

func (dm *DockerManager) WriteFile(projectName, fileName, content string) error {
	fullPath := filepath.Join(BasePath, projectName, fileName)
	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error al guardar el archivo %s: %w", fileName, err)
	}
	return nil
}

// --- NUEVA LÓGICA DE PUERTOS ---

// GetRequiredPorts lee el docker-compose.yml y extrae los puertos del host (ej: "8080:80" -> "8080")
func (dm *DockerManager) GetRequiredPorts(projectName string) ([]string, error) {
	files, _ := dm.GetProjectFiles(projectName)
	var ymlFile string
	for _, f := range files {
		if strings.HasSuffix(f, ".yml") || strings.HasSuffix(f, ".yaml") {
			ymlFile = f
			break
		}
	}

	if ymlFile == "" {
		return nil, nil
	}

	content, err := dm.ReadFile(projectName, ymlFile)
	if err != nil {
		return nil, err
	}

	var config ComposeConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("error al parsear YAML: %w", err)
	}

	var ports []string
	for _, service := range config.Services {
		for _, p := range service.Ports {
			parts := strings.Split(p, ":")
			if len(parts) > 0 {
				ports = append(ports, parts[0])
			}
		}
	}

	return ports, nil
}

// CheckPortConflicts verifica si algún puerto requerido ya está siendo usado por otro contenedor activo
func (dm *DockerManager) CheckPortConflicts(projectName string) (string, error) {
	requiredPorts, err := dm.GetRequiredPorts(projectName)
	if err != nil {
		return "", err
	}
	if len(requiredPorts) == 0 {
		return "", nil
	}

	// Listamos contenedores activos con container.ListOptions
	containers, err := dm.Cli.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, c := range containers {
		for _, portMapping := range c.Ports {
			hostPort := fmt.Sprintf("%d", portMapping.PublicPort)
			
			for _, reqPort := range requiredPorts {
				if hostPort == reqPort {
					containerName := "desconocido"
					if len(c.Names) > 0 {
						containerName = c.Names[0][1:]
					}
					return containerName, nil
				}
			}
		}
	}

	return "", nil
}