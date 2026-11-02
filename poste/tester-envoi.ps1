<#
.SYNOPSIS
  Envoie un fichier vers MULCV comme le ferait PaperStream.
  MULCV lit le QR du carton ou du dossier dans le fichier. Si le fichier n'a pas de QR,
  indiquer l'identifiant du dossier : il sert alors de nom de fichier.
#>
param(
  [string]$Fichier = $env:MULCV_FICHIER,
  [string]$Dossier = $env:MULCV_DOSSIER
)
$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Fichier)) { throw "Indiquer le fichier a envoyer." }
$Dossier = "$Dossier".Trim()
if ($Dossier -and $Dossier -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$') { throw "Identifiant de dossier invalide : $Dossier" }
$racine = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $racine 'bin\mulcv-depot.exe'
if (-not (Test-Path (Join-Path $racine 'config\mulcv-depot.json'))) { throw "Le poste n'est pas installe : lancer d'abord installer.cmd" }
$Fichier = $Fichier.Trim().Trim('"')
if (-not (Test-Path $Fichier)) { throw "Fichier introuvable : $Fichier" }
$extension = [IO.Path]::GetExtension($Fichier).ToLower()
if ($Dossier) { $nom = $Dossier + $extension } else { $nom = 'TEST_' + (Get-Date -Format 'yyyyMMdd_HHmmss') + $extension }
$copie = Join-Path $racine ('numerisation\1-a-envoyer\' + $nom)
Copy-Item $Fichier $copie -Force
& $exe $copie
Write-Host ''
if ($LASTEXITCODE -eq 0) {
  Write-Host "Accepte par MULCV (detail dans journal\mulcv-depot.log). Copie locale : numerisation\2-envoyes"
} else {
  Write-Host "Echec : voir numerisation\3-erreurs et journal\mulcv-depot.log"
}
