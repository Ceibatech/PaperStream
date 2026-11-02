<#
.SYNOPSIS
  Envoie un fichier vers le dossier MULCV portant l'identifiant indique, comme le ferait PaperStream.
#>
param(
  [string]$Fichier = $env:MULCV_FICHIER,
  [string]$Dossier = $env:MULCV_DOSSIER
)
$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($Fichier) -or [string]::IsNullOrWhiteSpace($Dossier)) { throw "Indiquer le fichier et l'identifiant du dossier." }
$Dossier = $Dossier.Trim()
if ($Dossier -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$') { throw "Identifiant de dossier invalide : $Dossier" }
$racine = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $racine 'bin\mulcv-depot.exe'
if (-not (Test-Path (Join-Path $racine 'config\mulcv-depot.json'))) { throw "Le poste n'est pas installe : lancer d'abord installer.cmd" }
$Fichier = $Fichier.Trim().Trim('"')
if (-not (Test-Path $Fichier)) { throw "Fichier introuvable : $Fichier" }
# Copie nommee comme PaperStream la produira : <identifiant du dossier>.<extension>
$copie = Join-Path $racine ('numerisation\1-a-envoyer\' + $Dossier + [IO.Path]::GetExtension($Fichier).ToLower())
Copy-Item $Fichier $copie -Force
& $exe $copie
Write-Host ''
if ($LASTEXITCODE -eq 0) {
  Write-Host "Envoye. Dans MULCV : Scan / Production / <zone> / <carton> / $Dossier"
  Write-Host "Copie locale : numerisation\2-envoyes"
} else {
  Write-Host "Echec : voir numerisation\3-erreurs et journal\mulcv-depot.log"
}
