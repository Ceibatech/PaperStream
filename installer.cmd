@echo off
rem Installe ou met a jour le depot MULCV de ce poste dans C:\MULCV. Double-cliquer.
cd /d "%~dp0"
echo.
echo  Installation du depot MULCV pour PaperStream (C:\MULCV)
echo  ------------------------------------------------------
set /p POSTE=Numero de ce poste (1 ou 2) : 
if not "%POSTE%"=="1" if not "%POSTE%"=="2" (
  echo Numero invalide : tapez 1 ou 2.
  pause
  exit /b 1
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" -Poste %POSTE%
pause
