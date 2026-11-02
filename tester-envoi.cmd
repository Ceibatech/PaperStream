@echo off
rem Envoie un fichier vers un dossier MULCV deja transfere par GeoArchiv, comme PaperStream.
cd /d "%~dp0"
set /p MULCV_FICHIER=Fichier a envoyer (glisser le PDF dans cette fenetre puis Entree) : 
set /p MULCV_DOSSIER=Identifiant du dossier GeoArchiv (ex. TEST-DOS-001) : 
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0tester-envoi.ps1"
pause
