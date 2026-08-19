package docker

import (
	"testing"
	"os"
	"path/filepath"
)

func TestFullSystemFlow(t *testing.T) {
	// 1. Inicializar Cliente
	mgr, err := NewDockerClient()
	if err != nil {
		t.Skipf("⏭️ Saltando prueba de integración: Docker no disponible: %v", err)
	}
	defer mgr.Close()

	// Datos de prueba
	testContainerName := "e2e-test-container"
	testImage := "alpine" // Imagen ultra ligera

	// Limpieza previa: Borrar el contenedor si ya existía de una prueba fallida
	_ = mgr.StopContainer(testContainerName)
	// Nota: Deberías implementar mgr.RemoveContainer si quieres borrarlo totalmente
	
	t.Log("🚀 Iniciando flujo de prueba general...")

	// 2. PROBAR CREACIÓN (Fase 4)
	err = mgr.CreateManagedContainer(testContainerName, testImage, []string{})
	if err != nil {
		t.Fatalf("❌ Falló la creación del contenedor: %v", err)
	}
	t.Log("✅ Contenedor creado y lanzado con éxito")

	// 3. PROBAR SISTEMA DE ARCHIVOS (Fase 1)
	// Verificar que la carpeta en /stacks fue creada automáticamente
	projectPath := filepath.Join(mgr.FS.RootPath, testContainerName)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		t.Errorf("❌ La carpeta del proyecto no fue creada en %s", projectPath)
	} else {
		t.Log("✅ Carpeta en /stacks verificada")
	}

	// 4. PROBAR LISTADO (Fase 2)
	containers, err := mgr.ListContainers()
	if err != nil {
		t.Fatalf("❌ Error al listar contenedores: %v", err)
	}

	found := false
	for _, c := range containers {
		if c.Name == testContainerName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("❌ El contenedor %s no aparece en la lista", testContainerName)
	} else {
		t.Log("✅ Contenedor encontrado en la lista")
	}

	// 5. PROBAR DETENCIÓN
	err = mgr.StopContainer(testContainerName)
	if err != nil {
		t.Errorf("❌ Error al detener el contenedor: %v", err)
	} else {
		t.Log("✅ Contenedor detenido con éxito")
	}

	t.Log("🎉 PRUEBA GENERAL COMPLETADA CON ÉXITO")
}
