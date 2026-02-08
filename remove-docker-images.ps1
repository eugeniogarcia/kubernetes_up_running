<#
.SYNOPSIS
  Elimina todas las imágenes Docker del sistema.

.DESCRIPTION
  Este script lista todas las imágenes Docker y las borra.
  Opciones:
    -Force:    No pedir confirmación.
    -RemoveContainers: Eliminar (forzar) todos los contenedores antes de borrar imágenes.

.EXAMPLE
  .\remove-docker-images.ps1
  .\remove-docker-images.ps1 -Force
  .\remove-docker-images.ps1 -RemoveContainers -Force
#>

param(
    [switch]$Force,
    [switch]$RemoveContainers
)

function Abort-WithMessage($msg) {
    Write-Error $msg
    exit 1
}

# Verificar que docker exista
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Abort-WithMessage "Docker no está disponible en el PATH. Instale Docker o ajuste el PATH."
}

# Obtener todas las imágenes (IDs únicas)
$images = docker images -a -q | Where-Object { $_ -ne "" } | Select-Object -Unique
if (-not $images) {
    Write-Output "No se encontraron imágenes Docker. Nada que borrar."
    exit 0
}

Write-Output "Imágenes encontradas: $($images.Count)"
# Mostrar lista legible
try {
    docker images --format "{{.Repository}}:{{.Tag}} {{.ID}}" | ForEach-Object { Write-Output "  $_" }
} catch {
    # si falla el formato, mostrar IDs
    $images | ForEach-Object { Write-Output "  $_" }
}

if (-not $Force) {
    $confirm = Read-Host "¿Desea borrar todas las imágenes listadas? (s/N)"
    if ($confirm -notin @('s','S','y','Y')) {
        Write-Output "Operación cancelada por el usuario."
        exit 0
    }
}

if ($RemoveContainers) {
    $containers = docker ps -a -q | Where-Object { $_ -ne "" }
    if ($containers) {
        Write-Output "Eliminando contenedores ($($containers.Count))..."
        try {
            docker rm -f @containers | ForEach-Object { Write-Output "  $_" }
        } catch {
            Write-Warning "Error al eliminar contenedores: $_"
        }
    } else {
        Write-Output "No hay contenedores para eliminar."
    }
} else {
    Write-Output "No se eliminarán contenedores. Si hay contenedores que usan imágenes, el borrado de esas imágenes puede fallar. Use -RemoveContainers para eliminarlos automáticamente."
}

# Borrar imágenes por ID
foreach ($id in $images) {
    Write-Output "Borrando imagen $id ..."
    try {
        $out = docker rmi -f $id 2>&1
        $out | ForEach-Object { Write-Output "  $_" }
    } catch {
        Write-Warning ("Fallo al borrar la imagen {0}: {1}" -f $id, $_)
    }
}

Write-Output "Operación completada."
