package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
)

// ContainerInfo es una estructura simplificada para mostrar la info del contenedor
type ContainerInfo struct {
	ID    string
	Name  string
	Image string
	State string
}

// ListContainers devuelve una lista de todos los contenedores (activos e inactivos)
func (dm *DockerManager) ListContainers() ([]ContainerInfo, error) {
	ctx := context.Background()

	// container.ListOptions{All: true} nos permite ver también los contenedores apagados
	containers, err := dm.Cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("error al listar contenedores: %w", err)
	}

	var result []ContainerInfo

	for _, c := range containers {
		// Los nombres de Docker vienen con un "/" al principio (ej: "/mi_bot"), lo limpiamos
		name := "Sin nombre"
		if len(c.Names) > 0 {
			name = c.Names[0][1:] 
		}

		result = append(result, ContainerInfo{
			ID:    c.ID[:12], // Solo tomamos los primeros 12 caracteres del ID
			Name:  name,
			Image: c.Image,
			State: c.State,
		})
	}

	return result, nil
}