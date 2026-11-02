@echo off
rem Envoie un fichier vers MULCV comme PaperStream : MULCV lit le QR du carton ou du dossier.
cd /d "%~dp0"
set /p MULCV_FICHIER=Fichier a envoyer (glisser le PDF dans cette fenetre puis Entree) : 
set MULCV_DOSSIER=
set /p MULCV_DOSSIER=Identifiant du dossier, seulement si le fichier n'a pas de QR (sinon Entree) : 
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0tester-envoi.ps1"
pause
