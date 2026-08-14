package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"gopkg.in/yaml.v3"
)

// --- Estructuras para parsear el docker-compose.yml ---
type ComposeConfig struct {
	Services map[string]ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	Ports []string `yaml:"ports"`
}

// GetProjectFiles busca qué archivos de configuración existen usando el FSManager
func (dm *DockerManager) GetProjectFiles(projectName string) ([]string, error) {
	// Usamos ListDir del FSManager para ver qué hay en la carpeta del proyecto
	entries, err := dm.FS.ListDir(projectName)
	if err != nil {
		return nil, err
	}

	targets := map[string]bool{
		"docker-compose.yml":  true,
		"docker-compose.yaml": true,
		".env":                true,
		"Dockerfile":          true,
	}

	var foundFiles []string
	for _, entry := range entries {
		if targets[entry.Name()] {
			foundFiles = append(foundFiles, entry.Name())
		}
	}
	return foundFiles, nil
}

// --- LÓGICA DE PUERTOS ---

// GetRequiredPorts lee el docker-compose.yml y extrae los puertos del host
func (dm *DockerManager) GetRequiredPorts(projectName string) ([]string, error) {
	files, err := dm.GetProjectFiles(projectName)
	if err != nil {
		return nil, err
	}

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

	// IMPORTANTE: Ahora usamos dm.FS.ReadFile en lugar de dm.ReadFile
	// Unimos el nombre del proyecto y el archivo para crear la ruta relativa
	content, err := dm.FS.ReadFile(filepath.Join(projectName, ymlFile))
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

// CheckPortConflicts verifica si algún puerto requerido ya está siendo usado
func (dm *DockerManager) CheckPortConflicts(projectName string) (string, error) {
	requiredPorts, err := dm.GetRequiredPorts(projectName)
	if err != nil {
		return "", err
	}
	if len(requiredPorts) == 0 {
		return "", nil
	}

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