package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FSManager struct {
	RootPath string
}

// NewFSManager inicializa el gestor de archivos con una ruta raíz (ej: /stacks)
func NewFSManager(root string) *FSManager {
	return &FSManager{
		RootPath: root,
	}
}

// SafePath es la función más importante. 
// Limpia la ruta y verifica que el resultado final esté DENTRO de RootPath.
// Esto evita ataques de Path Traversal (../../)
func (fsm *FSManager) SafePath(userInputPath string) (string, error) {
	// 1. Unimos la raíz con la ruta pedida por el usuario
	fullPath := filepath.Join(fsm.RootPath, userInputPath)

	// 2. Limpiamos la ruta (resuelve los .. y .)
	cleanedPath := filepath.Clean(fullPath)

	// 3. Verificamos que la ruta resultante empiece con RootPath
	if !strings.HasPrefix(cleanedPath, fsm.RootPath) {
		return "", fmt.Errorf("acceso denegado: intento de salir del directorio raíz")
	}

	return cleanedPath, nil
}

// ListDir devuelve los archivos y carpetas de una ruta específica
func (fsm *FSManager) ListDir(path string) ([]os.DirEntry, error) {
	safePath, err := fsm.SafePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(safePath)
	if err != nil {
		return nil, fmt.Errorf("error al leer el directorio: %w", err)
	}

	return entries, nil
}

// ReadFile lee el contenido de un archivo
func (fsm *FSManager) ReadFile(path string) (string, error) {
	safePath, err := fsm.SafePath(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("error al leer el archivo: %w", err)
	}

	return string(content), nil
}

// WriteFile guarda el contenido en un archivo (creándolo o sobreescribiéndolo)
func (fsm *FSManager) WriteFile(path string, content string) error {
	safePath, err := fsm.SafePath(path)
	if err != nil {
		return err
	}

	// 0644: El dueño puede leer/escribir, los demás solo leer
	err = os.WriteFile(safePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error al escribir el archivo: %w", err)
	}

	return nil
}

// DeleteFile elimina un archivo o carpeta (recursivamente)
func (fsm *FSManager) DeleteFile(path string) error {
	safePath, err := fsm.SafePath(path)
	if err != nil {
		return err
	}

	err = os.RemoveAll(safePath)
	if err != nil {
		return fmt.Errorf("error al eliminar: %w", err)
	}

	return nil
}