package docker

import (
	"bytes"   // Añadido: Para capturar la salida en memoria
	"context"
	"fmt"


	"github.com/docker/docker/api/types"             
	"github.com/docker/docker/api/types/container"   
	"github.com/docker/docker/pkg/stdcopy"           
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

	containers, err := dm.Cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("error al listar contenedores: %w", err)
	}

	var result []ContainerInfo

	for _, c := range containers {
		name := "Sin nombre"
		if len(c.Names) > 0 {
			name = c.Names[0][1:] 
		}

		result = append(result, ContainerInfo{
			ID:    c.ID[:12],
			Name:  name,
			Image: c.Image,
			State: c.State,
		})
	}

	return result, nil
}

// StartContainer inicia un contenedor apagado
func (dm *DockerManager) StartContainer(id string) error {
	ctx := context.Background()
	err := dm.Cli.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("error al iniciar el contenedor %s: %w", id, err)
	}
	return nil
}

// StopContainer detiene un contenedor encendido
func (dm *DockerManager) StopContainer(id string) error {
	ctx := context.Background()
	timeout := 10
	stopOptions := container.StopOptions{Timeout: &timeout}
	
	err := dm.Cli.ContainerStop(ctx, id, stopOptions)
	if err != nil {
		return fmt.Errorf("error al detener el contenedor %s: %w", id, err)
	}
	return nil
}

// RestartContainer reinicia un contenedor deteniéndolo y volviéndolo a iniciar.
func (dm *DockerManager) RestartContainer(id string) error {
	fmt.Printf("🔄 Reiniciando contenedor %s...\n", id)

	err := dm.StopContainer(id)
	if err != nil {
		fmt.Printf("⚠️ Aviso: El contenedor %s ya estaba detenido o hubo un error: %v\n", id, err)
	}

	err = dm.StartContainer(id)
	if err != nil {
		return fmt.Errorf("error al iniciar el contenedor %s después del stop: %w", id, err)
	}

	return nil
}

// GetLogs obtiene los logs recientes y los devuelve como un string para la web
func (dm *DockerManager) GetLogs(id string) (string, error) {
	ctx := context.Background()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
		Follow:     false,
	}

	out, err := dm.Cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return "", fmt.Errorf("error al obtener logs: %w", err)
	}
	defer out.Close()

	// Usamos un buffer para guardar la salida en lugar de imprimirla en consola
	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, out) 
	if err != nil {
		return "", fmt.Errorf("error al procesar la salida de logs: %w", err)
	}

	return buf.String(), nil
}

// ExecCommand ejecuta un comando y devuelve la respuesta como un string para la web
func (dm *DockerManager) ExecCommand(id string, cmd []string) (string, error) {
	ctx := context.Background()

	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := dm.Cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return "", fmt.Errorf("error al crear el comando exec: %w", err)
	}

	resp, err := dm.Cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("error al adjuntar al comando exec: %w", err)
	}
	defer resp.Close()

	// Usamos un buffer para capturar la respuesta del comando
	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, resp.Reader)
	if err != nil {
		return "", fmt.Errorf("error al leer la respuesta del comando: %w", err)
	}

	return buf.String(), nil
}