@echo off
rem Envoie un PDF de test vers un dossier MULCV deja transfere par GeoArchiv.
cd /d "%~dp0"
set /p FICHIER=Chemin du PDF a envoyer (glisser le fichier ici) : 
set /p DOSSIER=Identifiant du dossier GeoArchiv (ex. TEST-DOS-001) : 
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0tester-envoi.ps1" -Fichier %FICHIER% -Dossier "%DOSSIER%"
pause
