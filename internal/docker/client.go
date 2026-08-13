package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

type DockerManager struct {
	Cli *client.Client
}

func NewDockerClient() (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error al crear el cliente de Docker: %w", err)
	}

	ctx := context.Background()
	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con el motor de Docker. ¿está corriendo?: %w", err)
	}

	return &DockerManager{
		Cli: cli,
	}, nil
}

func (dm *DockerManager) Close() {
	dm.Cli.Close()
}