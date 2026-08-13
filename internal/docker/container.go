package docker

import (
	"context"
	"fmt"
	"os" // Importante para os.Stdout y os.Stderr

	"github.com/docker/docker/api/types"             // Añadido para tipos generales
	"github.com/docker/docker/api/types/container"   // Para opciones de contenedores
	"github.com/docker/docker/pkg/stdcopy"           // Fundamental para procesar logs y exec
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

// GetLogs obtiene los logs recientes de un contenedor
func (dm *DockerManager) GetLogs(id string) error {
	ctx := context.Background()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
		Follow:     false,
	}

	out, err := dm.Cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return fmt.Errorf("error al obtener logs: %w", err)
	}
	defer out.Close()

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	if err != nil {
		return fmt.Errorf("error al procesar la salida de logs: %w", err)
	}

	return nil
}

// ExecCommand ejecuta un comando dentro de un contenedor y devuelve la salida
func (dm *DockerManager) ExecCommand(id string, cmd []string) error {
	ctx := context.Background()

	// FIX: Usamos types.ExecConfig en lugar de container.ExecOptions
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := dm.Cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return fmt.Errorf("error al crear el comando exec: %w", err)
	}

	// FIX: Usamos types.ExecStartCheck{} para el Attach
	resp, err := dm.Cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("error al adjuntar al comando exec: %w", err)
	}
	defer resp.Close()

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, resp.Reader)
	if err != nil {
		return fmt.Errorf("error al leer la respuesta del comando: %w", err)
	}

	return nil
}