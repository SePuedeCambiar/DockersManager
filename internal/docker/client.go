package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"go-docker-manager/internal/fs" // <--- Importamos el nuevo paquete de archivos
)

type DockerManager struct {
	Cli *client.Client
	FS  *fs.FSManager // <--- Añadimos el gestor de archivos a la estructura
}

func NewDockerClient() (*DockerManager, error) {
	// 1. Crear el cliente de Docker
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error al crear el cliente de Docker: %w", err)
	}

	// 2. Verificar conexión con el motor de Docker
	ctx := context.Background()
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con el motor de Docker. ¿está corriendo?: %w", err)
	}

	// 3. Retornamos el DockerManager con el Cliente de Docker Y el Gestor de Archivos
	return &DockerManager{
		Cli: cli,
		FS:  fs.NewFSManager("/stacks"), // <--- Inicializamos la raíz en /stacks
	}, nil
}

func (dm *DockerManager) Close() {
	dm.Cli.Close()
}