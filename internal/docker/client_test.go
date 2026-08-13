package docker

import (
	"testing"
)

func TestNewDockerClient(t *testing.T) {
	// Intentamos crear el cliente
	mgr, err := NewDockerClient()
	
	if err != nil {
		t.Fatalf("❌ Error: No se pudo conectar al motor de Docker: %v", err)
	}
	
	if mgr.Cli == nil {
		t.Error("❌ Error: El cliente de Docker es nil")
	}
	
	t.Log("✅ Conexión al cliente de Docker verificada con éxito")
}