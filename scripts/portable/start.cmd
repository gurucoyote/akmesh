@echo off
rem akmesh portable: open a shell whose PATH leads to this folder's bin.
rem
rem bin\ is not shipped as a folder of copies: one file would be 20 MB times
rem every name. It is laid out on first run as hardlinks, which cost no disk and
rem are what makes "sh", "ls", "grep" answer at all - the binary decides by the
rem name it was started as.
rem
rem The binary itself lives in bin\ and not in the root of the folder: up there
rem it would be the first thing a person double clicks, and a program started
rem under the name akmesh has no way to know which command was meant. The only
rem thing to start in the root is this file.
rem
rem The names come from etc\names.txt, written when the bundle was built: asking
rem the binary here worked badly (a nested command inside for /f needs quoting
rem nobody wants to read later), and a generated file cannot drift.
setlocal enabledelayedexpansion
set "HERE=%~dp0"
if "%HERE:~-1%"=="\" set "HERE=%HERE:~0,-1%"

if not exist "%HERE%\bin\akmesh.exe" (
	echo akmesh: bin\akmesh.exe is missing. Unpack the whole folder again.
	pause
	exit /b 1
)

if not exist "%HERE%\bin\.ready" call :links
if not exist "%HERE%\bin\sh.exe" (
	echo akmesh: bin\ could not be laid out. See the note about hardlinks in README.txt.
	pause
	exit /b 1
)

rem apk's own root inside the bundle; winget is the one source configured in
rem root\etc\apk\repositories.
rem
rem The order of PATH is the point: what was installed comes first, the bundle's
rem own set comes after it. A person who installs a tool - from an apk repository
rem or through winget - expects that tool to be the one that answers, even when
rem akmesh ships a command of the same name. Built-ins like cd or echo live in
rem the shell itself and cannot be replaced this way; everything else can.
rem
rem winget writes installed programs into its own folder below Packages and adds
rem each of those folders to the user PATH (measured 26.11.2026: that is the
rem "Befehlszeilenalias hinzugefügt" line). The links folder stays empty, so the
rem folders are collected here instead of trusted to the inherited PATH.
set "AKMESH_ROOT=%HERE%\root"
set "WINGET_LINKS=%LOCALAPPDATA%\Microsoft\WinGet\Links"
set "WINDOWS_APPS=%LOCALAPPDATA%\Microsoft\WindowsApps"
set "WINGET_PKGS="
for /d %%d in ("%LOCALAPPDATA%\Microsoft\WinGet\Packages\*") do set "WINGET_PKGS=!WINGET_PKGS!%%d;"
set "PATH=%HERE%\root\usr\bin;%HERE%\root\bin;%WINGET_LINKS%;%WINGET_PKGS%%WINDOWS_APPS%;%HERE%\bin;%PATH%"
set "AKMESH_PROFILE=%HERE%\etc\profile.sh"
if not defined HOME set "HOME=%USERPROFILE%"
if not defined TERM set "TERM=xterm-256color"

rem -l makes it a login shell, so it reads the profile above, and every argument
rem reaches the shell unchanged: "start.cmd -c \"echo hi\"" is a one-shot run.
"%HERE%\bin\sh.exe" -l %*
exit /b %ERRORLEVEL%

:links
for /f "usebackq delims=" %%n in ("%HERE%\etc\names.txt") do (
	if not exist "%HERE%\bin\%%n.exe" mklink /H "%HERE%\bin\%%n.exe" "%HERE%\bin\akmesh.exe" >nul 2>&1
)
rem The marker keeps the next start from walking the whole list again.
echo ready> "%HERE%\bin\.ready"
exit /b 0
