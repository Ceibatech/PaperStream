<#
.SYNOPSIS
  Installe le depot PaperStream -> MULCV sur un poste de numerisation.

.EXAMPLE
  installer.cmd                (double-clic : demande le numero du poste)
  powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 1
  powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 2 -Surveillance

.NOTES
  A lancer avec le compte Windows de l'operateur associe au poste.
  Le secret du poste se lit sur le serveur :
    cat /opt/ceiba/keycloak/secrets/mulcv-depot-poste<N>.secret
#>
param(
  [Parameter(Mandatory = $true)][ValidateSet(1, 2)][int]$Poste,
  [string]$Dossier = 'C:\MULCV',
  # Tache planifiee qui surveille C:\MULCV\Sortie, pour une destination PaperStream "Folder".
  [switch]$Surveillance
)
$ErrorActionPreference = 'Stop'
$ici = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $ici 'bin\mulcv-depot.exe'
$modele = Join-Path $ici "postes\poste-$Poste.json"
if (-not (Test-Path $exe)) { throw "bin\mulcv-depot.exe introuvable : cloner le depot complet." }
if (-not (Test-Path $modele)) { throw "Configuration du poste $Poste introuvable : $modele" }

foreach ($d in @($Dossier, "$Dossier\Envoyes", "$Dossier\Erreurs", "$Dossier\Sortie", "$Dossier\Essais")) {
  New-Item -ItemType Directory -Force -Path $d | Out-Null
}
Copy-Item $exe "$Dossier\mulcv-depot.exe" -Force

$config = Get-Content $modele -Raw | ConvertFrom-Json
$cible = "$Dossier\mulcv-depot.json"
$garder = $false
if (Test-Path $cible) {
  $reponse = Read-Host "Une configuration existe deja. La conserver (secret inclus) ? [O/n]"
  $garder = ($reponse -eq '' -or $reponse -match '^[oOyY]')
}
if (-not $garder) {
  $secure = Read-Host "Secret Keycloak du client $($config.clientId)" -AsSecureString
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try { $secret = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
  finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
  if ([string]::IsNullOrWhiteSpace($secret)) { throw "Secret vide" }
  $config.clientSecret = $secret.Trim()
  $config.sentDir = "$Dossier\Envoyes"
  $config.errorDir = "$Dossier\Erreurs"
  $config.logFile = "$Dossier\mulcv-depot.log"
  $config | ConvertTo-Json | Set-Content -Path $cible -Encoding ASCII
  Remove-Variable secret

  # Le secret n'est lisible que par l'operateur, SYSTEM et les administrateurs (SID : independant de la langue).
  $acl = New-Object Security.AccessControl.FileSecurity
  $acl.SetAccessRuleProtection($true, $false)
  $comptes = @(
    [Security.Principal.WindowsIdentity]::GetCurrent().User,
    (New-Object Security.Principal.SecurityIdentifier 'S-1-5-18'),
    (New-Object Security.Principal.SecurityIdentifier 'S-1-5-32-544'))
  foreach ($compte in $comptes) {
    $acl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($compte, 'FullControl', 'Allow')))
  }
  Set-Acl -Path $cible -AclObject $acl
}

& "$Dossier\mulcv-depot.exe" -test
if ($LASTEXITCODE -ne 0) { throw "Le test de connexion a echoue : verifier le secret et la connexion Internet." }

if ($Surveillance) {
  $action = New-ScheduledTaskAction -Execute "$Dossier\mulcv-depot.exe" -Argument "-watch `"$Dossier\Sortie`"" -WorkingDirectory $Dossier
  $declencheur = New-ScheduledTaskTrigger -AtLogOn -User ([Security.Principal.WindowsIdentity]::GetCurrent().Name)
  $reglages = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
  Register-ScheduledTask -TaskName 'MULCV depot PaperStream' -Action $action -Trigger $declencheur -Settings $reglages -Force | Out-Null
  Start-ScheduledTask -TaskName 'MULCV depot PaperStream'
  Write-Host "Surveillance active sur $Dossier\Sortie."
}

Write-Host ''
Write-Host "Poste $Poste installe ($($config.clientId)). Connexion a MULCV verifiee."
Write-Host ''
Write-Host 'Dans PaperStream Capture, destination "Other App" :'
Write-Host "  Application Path : $Dossier\mulcv-depot.exe"
Write-Host '  Parameters       : %f'
Write-Host "  Work Folder      : $Dossier"
Write-Host ''
Write-Host 'Pour un essai sans scanner : tester-envoi.cmd'
