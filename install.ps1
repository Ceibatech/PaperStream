<#
.SYNOPSIS
  Installe ou met a jour le depot PaperStream -> MULCV sur un poste de numerisation.

.DESCRIPTION
  Cree C:\MULCV et toute son arborescence (bin, config, numerisation, journal),
  y copie le programme, puis ecrit la configuration du poste et verifie la connexion.
  Relancer apres un "git pull" met a jour le programme sans toucher a la configuration
  ni aux fichiers numerises.

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
  [string]$Destination = 'C:\MULCV',
  # Tache planifiee qui envoie ce que PaperStream depose dans numerisation\1-a-envoyer (mode "Folder").
  [switch]$Surveillance
)
$ErrorActionPreference = 'Stop'
$kit = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'poste'
if (-not (Test-Path (Join-Path $kit 'bin\mulcv-depot.exe'))) { throw "poste\bin\mulcv-depot.exe introuvable : cloner le depot complet." }

# 1. Arborescence et programme : tout est cree s'il manque, rien de l'utilisateur n'est efface.
foreach ($d in @('bin', 'config\postes', 'numerisation\1-a-envoyer', 'numerisation\2-envoyes', 'numerisation\3-erreurs', 'journal')) {
  New-Item -ItemType Directory -Force -Path (Join-Path $Destination $d) | Out-Null
}
Get-ChildItem $kit -Recurse -File | ForEach-Object {
  $relatif = $_.FullName.Substring($kit.Length + 1)
  Copy-Item $_.FullName (Join-Path $Destination $relatif) -Force
}
$exe = Join-Path $Destination 'bin\mulcv-depot.exe'
$cible = Join-Path $Destination 'config\mulcv-depot.json'
$modele = Join-Path $Destination "config\postes\poste-$Poste.json"
Write-Host "Arborescence prete dans $Destination."

# 2. Configuration du poste (le secret n'est demande qu'a la premiere installation).
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

# 3. Verification de la connexion a MULCV.
& $exe -test
if ($LASTEXITCODE -ne 0) { throw "Le test de connexion a echoue : verifier le secret et la connexion Internet." }

if ($Surveillance) {
  $action = New-ScheduledTaskAction -Execute $exe -Argument '-watch' -WorkingDirectory $Destination
  $declencheur = New-ScheduledTaskTrigger -AtLogOn -User ([Security.Principal.WindowsIdentity]::GetCurrent().Name)
  $reglages = New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
  Register-ScheduledTask -TaskName 'MULCV depot PaperStream' -Action $action -Trigger $declencheur -Settings $reglages -Force | Out-Null
  Start-ScheduledTask -TaskName 'MULCV depot PaperStream'
  Write-Host "Surveillance active sur $Destination\numerisation\1-a-envoyer."
}

Write-Host ''
Write-Host "Poste $Poste installe dans $Destination. Connexion a MULCV verifiee."
Write-Host ''
Write-Host 'Dans PaperStream Capture :'
Write-Host "  Dossier de sortie : $Destination\numerisation\1-a-envoyer"
Write-Host '  Destination       : Other App'
Write-Host "  Application Path  : $exe"
Write-Host '  Parameters        : %f'
Write-Host "  Work Folder       : $Destination"
Write-Host ''
Write-Host "Essai sans scanner : $Destination\tester-envoi.cmd"
