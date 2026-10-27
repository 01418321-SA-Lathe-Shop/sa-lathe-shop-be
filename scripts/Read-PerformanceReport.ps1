function Read-PerformanceReport([object[]]$Events, [int]$ExitCode) {
  $outputs = @($Events | Where-Object { $_.Action -eq 'output' -and $_.Test -eq 'TestPerformanceBaseline' })
  # test2json intentionally splits long output at 1 KiB. Preserve each event's
  # original newlines; joining with an added newline would corrupt JSON strings.
  $text = ($outputs | ForEach-Object { $_.Output }) -join ''
  $reports = @()
  foreach ($line in ($text -split '\r?\n')) {
    $marker = 'PERFORMANCE_BASELINE_JSON='
    $position = $line.IndexOf($marker)
    if ($position -ge 0) {
      $reports += ($line.Substring($position + $marker.Length).Trim() | ConvertFrom-Json -ErrorAction Stop)
    } elseif ($line) { Write-Host $line.TrimEnd() }
  }
  $outcomes = @($Events | Where-Object { $_.Test -eq 'TestPerformanceBaseline' -and $_.Action -in @('pass','skip','fail') })
  if ($ExitCode -ne 0 -or $outcomes.Count -ne 1 -or $outcomes[0].Action -ne 'pass' -or $reports.Count -ne 1) {
    throw 'Performance baseline did not execute and pass with exactly one report; no capacity claim is valid.'
  }
  return $reports[0]
}
