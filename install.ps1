<#
.SYNOPSIS
  Installe le depot PaperStream -> MULCV sur un poste de numerisation.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 1
  powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 2 -Surveillance

.NOTES
  A lancer avec le compte Windows de l'operateur associe au poste.
  mulcv-depot.exe doit se trouver a cote de ce script (zip de la release).
#>
param(
  [Parameter(Mandatory = $true)][ValidateSet(1, 2)][int]$Poste,
  [string]$Dossier = 'C:\MULCV',
  # Cree une tache planifiee qui surveille C:\MULCV\Sortie a l'ouverture de session,
  # pour une destination PaperStream "Folder" au lieu de "Other App".
  [switch]$Surveillance
)
$ErrorActionPreference = 'Stop'
$ici = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $ici 'mulcv-depot.exe'
$modele = Join-Path $ici "postes\poste-$Poste.json"
if (-not (Test-Path $exe)) { throw "mulcv-depot.exe introuvable a cote de install.ps1" }
if (-not (Test-Path $modele)) { throw "Configuration du poste $Poste introuvable : $modele" }

foreach ($d in @($Dossier, "$Dossier\Envoyes", "$Dossier\Erreurs", "$Dossier\Sortie")) {
  New-Item -ItemType Directory -Force -Path $d | Out-Null
}
Copy-Item $exe "$Dossier\mulcv-depot.exe" -Force

$config = Get-Content $modele -Raw | ConvertFrom-Json
$secure = Read-Host "Secret Keycloak du client $($config.clientId)" -AsSecureString
$secret = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure))
if ([string]::IsNullOrWhiteSpace($secret)) { throw "Secret vide" }
$config.clientSecret = $secret
$config.sentDir = "$Dossier\Envoyes"
$config.errorDir = "$Dossier\Erreurs"
$config.logFile = "$Dossier\mulcv-depot.log"
$cible = "$Dossier\mulcv-depot.json"
$config | ConvertTo-Json | Set-Content -Path $cible -Encoding ASCII

# Le secret n'est lisible que par l'operateur, SYSTEM et les administrateurs.
$acl = New-Object Security.AccessControl.FileSecurity
$acl.SetAccessRuleProtection($true, $false)
foreach ($compte in @("$env:USERDOMAIN\$env:USERNAME", 'NT AUTHORITY\SYSTEM', 'BUILTIN\Administrators')) {
  $acl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($compte, 'FullControl', 'Allow')))
}
Set-Acl -Path $cible -AclObject $acl

& "$Dossier\mulcv-depot.exe" -test
if ($LASTEXITCODE -ne 0) { throw "Le test de connexion a echoue : verifier le secret et le reseau." }

if ($Surveillance) {
  $action = New-ScheduledTaskAction -Execute "$Dossier\mulcv-depot.exe" -Argument "-watch `"$Dossier\Sortie`"" -WorkingDirectory $Dossier
  $declencheur = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"
  $reglages = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
  Register-ScheduledTask -TaskName 'MULCV depot PaperStream' -Action $action -Trigger $declencheur -Settings $reglages -Force | Out-Null
  Start-ScheduledTask -TaskName 'MULCV depot PaperStream'
  Write-Host "Surveillance active sur $Dossier\Sortie."
}

Write-Host ''
Write-Host "Poste $Poste installe ($($config.clientId))."
Write-Host 'PaperStream Capture, destination "Other App" :'
Write-Host "  Application Path : $Dossier\mulcv-depot.exe"
Write-Host '  Parameters       : %f'
Write-Host "  Work Folder      : $Dossier"
