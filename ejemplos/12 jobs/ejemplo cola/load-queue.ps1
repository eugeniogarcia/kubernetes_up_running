$baseUrl = 'http://gz.com/memq/server/queues/keygen'

# Crear la cola
try {
    Invoke-RestMethod -Uri $baseUrl -Method Put -ErrorAction Stop
    Write-Host "Queue 'keygen' created or already exists."
} catch {
    $err = $_
    $ex = $err.Exception
    $statusCode = $null
    if ($ex -and $ex.Response) {
        try {
            $webResp = [System.Net.HttpWebResponse]$ex.Response
            $statusCode = [int]$webResp.StatusCode
        } catch {
            # ignore casting errors
        }
    }
    $message = $err.Exception.Message
    $fullText = $err.ToString()
    if ($statusCode -eq 409 -or $statusCode -eq [int][System.Net.HttpStatusCode]::Conflict -or ($message -and $message -match 'already exists') -or ($fullText -and $fullText -match 'already exists')) {
        Write-Host "Queue 'keygen' already exists; continuing."
    } else {
        Write-Error "Failed to create queue 'keygen': $err"
        exit 1
    }
}

# Crear 100 items y cargarlos en la cola
for ($i = 0; $i -lt 100; $i++) {
    $item = "work-item-$i"
    try {
        $resp = Invoke-RestMethod -Uri ("$baseUrl/enqueue") -Method Post -Body $item -ContentType 'text/plain' -ErrorAction Stop
        Write-Host "Enqueued: ${item} (id: $($resp.id))"
    } catch {
        Write-Warning "Failed to enqueue ${item}: $($_)"
    }
}