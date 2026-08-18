package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"           // <--- AGREGADO
	"path/filepath" // <--- AGREGADO
	"strings"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat" // <--- AGREGADO (Para los puertos)
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

// StartContainer inicia un contenedor apagado, pero primero valida los puertos
func (dm *DockerManager) StartContainer(id string) error {
	ctx := context.Background()

	inspect, err := dm.Cli.ContainerInspect(ctx, id)
	if err == nil {
		projectName := inspect.Name
		if strings.HasPrefix(projectName, "/") {
			projectName = projectName[1:]
		}

		conflictContainer, err := dm.CheckPortConflicts(projectName)
		if err != nil {
			fmt.Printf("⚠️ Aviso: No se pudo validar puertos: %v\n", err)
		} else if conflictContainer != "" {
			return fmt.Errorf("conflicto de puertos: el puerto ya está siendo usado por el contenedor '%s'", conflictContainer)
		}
	}

	err = dm.Cli.ContainerStart(ctx, id, container.StartOptions{})
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

// RestartContainer reinicia un contenedor deteniéndolo y volviéndolo a iniciar
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


// GetLogs obtiene los logs. Si 'until' está vacío, trae los últimos. 
// Si 'until' tiene un valor, trae los logs anteriores a ese tiempo.
func (dm *DockerManager) GetLogs(id string, until string) (string, error) {
	ctx := context.Background()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
		Follow:     false,
		Timestamps: true, // <--- ACTIVADO: Necesitamos la hora para saber dónde cortar
	}

	if until != "" {
		options.Until = until // Traer logs anteriores a este timestamp
	}

	out, err := dm.Cli.ContainerLogs(ctx, id, options)
	if err != nil {
		return "", fmt.Errorf("error al obtener logs: %w", err)
	}
	defer out.Close()

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, out)
	if err != nil {
		return "", fmt.Errorf("error al procesar la salida de logs: %w", err)
	}

	return buf.String(), nil
}


// ExecCommand ejecuta un comando y devuelve la respuesta como string
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

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, resp.Reader)
	if err != nil {
		return "", fmt.Errorf("error al leer la respuesta del comando: %w", err)
	}

	return buf.String(), nil
}

// CreateManagedContainer crea el contenedor y su carpeta de configuración
func (dm *DockerManager) CreateManagedContainer(name, image string, ports []string) error {
	ctx := context.Background()

	// 1. DESCARGAR LA IMAGEN (Crucial: sin esto, ContainerCreate falla si la imagen no existe)
	fmt.Printf("🚚 Descargando imagen %s...\n", image)
	reader, err := dm.Cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("error al solicitar descarga de imagen: %w", err)
	}
	defer reader.Close()
	
	// Leemos el reader hasta el final para esperar a que la descarga termine
	// (Si no hacemos esto, el código sigue y falla porque la imagen aún no está lista)
	_, _ = io.ReadAll(reader) 
	fmt.Println("✅ Imagen descargada.")

	// 2. Crear la carpeta en /stacks
	// NOTA: Si estás en Debian, asegúrate de que la carpeta /stacks exista y tengas permisos:
	// sudo mkdir /stacks && sudo chmod 777 /stacks
	projectPath := filepath.Join(dm.FS.RootPath, name)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Errorf("error creando carpeta del proyecto (¿tienes permisos en /stacks?): %w", err)
	}

	// 3. Configurar el mapeo de puertos
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	for _, p := range ports {
		parts := strings.Split(p, ":")
		if len(parts) == 2 {
			hostPort := parts[0]
			containerPort := parts[1]
			cPort := nat.Port(containerPort + "/tcp")
			exposedPorts[cPort] = struct{}{}
			portBindings[cPort] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}
		}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
	}

	// 4. Crear el contenedor
	resp, err := dm.Cli.ContainerCreate(ctx, &container.Config{
		Image:        image,
		ExposedPorts: exposedPorts,
	}, hostConfig, nil, nil, name)
	if err != nil {
		return fmt.Errorf("error al crear contenedor: %w", err)
	}

	// 5. Iniciar el contenedor
	if err := dm.Cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("error al iniciar contenedor: %w", err)
	}

	return nil
}
