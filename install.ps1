<#
.SYNOPSIS
  Installe le depot PaperStream -> MULCV sur un poste de numerisation.

.EXAMPLE
  installer.cmd                (double-clic : demande le numero du poste)
  powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 2 -Surveillance

.NOTES
  A lancer avec le compte Windows de l'operateur associe au poste.
  Le secret du poste se lit sur le serveur :
    cat /opt/ceiba/keycloak/secrets/mulcv-depot-poste<N>.secret
#>
param(
  [Parameter(Mandatory = $true)][ValidateSet(1, 2)][int]$Poste,
  # Tache planifiee qui envoie ce que PaperStream depose dans numerisation\1-a-envoyer (mode "Folder").
  [switch]$Surveillance
)
$ErrorActionPreference = 'Stop'
# Le depot clone est le dossier de travail : C:\MULCV si le clone a ete fait a cet endroit.
$racine = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe = Join-Path $racine 'bin\mulcv-depot.exe'
$modele = Join-Path $racine "config\postes\poste-$Poste.json"
$cible = Join-Path $racine 'config\mulcv-depot.json'
if (-not (Test-Path $exe)) { throw "bin\mulcv-depot.exe introuvable : cloner le depot complet." }
if (-not (Test-Path $modele)) { throw "Modele du poste $Poste introuvable : $modele" }

$garder = $false
if (Test-Path $cible) {
  $actuel = Get-Content $cible -Raw | ConvertFrom-Json
  $reponse = Read-Host "Ce poste est deja configure ($($actuel.clientId)). Garder cette configuration ? [O/n]"
  $garder = ($reponse -eq '' -or $reponse -match '^[oOyY]')
}
if (-not $garder) {
  $config = Get-Content $modele -Raw | ConvertFrom-Json
  $secure = Read-Host "Secret Keycloak du client $($config.clientId)" -AsSecureString
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try { $secret = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
  finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
  if ([string]::IsNullOrWhiteSpace($secret)) { throw "Secret vide" }
  $config | Add-Member -NotePropertyName clientSecret -NotePropertyValue $secret.Trim() -Force
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

& $exe -test
if ($LASTEXITCODE -ne 0) { throw "Le test de connexion a echoue : verifier le secret et la connexion Internet." }

if ($Surveillance) {
  $action = New-ScheduledTaskAction -Execute $exe -Argument '-watch' -WorkingDirectory $racine
  $declencheur = New-ScheduledTaskTrigger -AtLogOn -User ([Security.Principal.WindowsIdentity]::GetCurrent().Name)
  $reglages = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
  Register-ScheduledTask -TaskName 'MULCV depot PaperStream' -Action $action -Trigger $declencheur -Settings $reglages -Force | Out-Null
  Start-ScheduledTask -TaskName 'MULCV depot PaperStream'
  Write-Host "Surveillance active sur $racine\numerisation\1-a-envoyer."
}

Write-Host ''
Write-Host "Poste $Poste installe. Connexion a MULCV verifiee."
Write-Host ''
Write-Host 'Dans PaperStream Capture, destination "Other App" :'
Write-Host "  Application Path : $exe"
Write-Host '  Parameters       : %f'
Write-Host "  Work Folder      : $racine"
Write-Host ''
Write-Host 'Pour un essai sans scanner : tester-envoi.cmd'
