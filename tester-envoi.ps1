<#
.SYNOPSIS
  Envoie un fichier vers le dossier MULCV portant l'identifiant indique, comme le ferait PaperStream.
#>
param(
  [Parameter(Mandatory = $true)][string]$Fichier,
  [Parameter(Mandatory = $true)][ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$')][string]$Dossier,
  [string]$Installation = 'C:\MULCV'
)
$ErrorActionPreference = 'Stop'
$exe = Join-Path $Installation 'mulcv-depot.exe'
if (-not (Test-Path $exe)) { throw "Le poste n'est pas installe : lancer d'abord installer.cmd" }
if (-not (Test-Path $Fichier)) { throw "Fichier introuvable : $Fichier" }
$extension = [IO.Path]::GetExtension($Fichier).ToLower()
$essai = Join-Path $Installation 'Essais'
New-Item -ItemType Directory -Force -Path $essai | Out-Null
# Copie nommee comme PaperStream la produira : <identifiant du dossier>.<extension>
$copie = Join-Path $essai ($Dossier + $extension)
Copy-Item $Fichier $copie -Force
& $exe $copie
if ($LASTEXITCODE -eq 0) {
  Write-Host ''
  Write-Host "Envoye. Le fichier apparait dans MULCV, dossier $Dossier (Scan / Production / <zone> / <carton> / $Dossier)."
} else {
  Write-Host ''
  Write-Host "Echec : voir C:\MULCV\Erreurs et C:\MULCV\mulcv-depot.log"
}
