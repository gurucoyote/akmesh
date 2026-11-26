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
setlocal
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

rem apk's own root inside the bundle, and the directories a package would write
rem into. winget is the one source configured in root\etc\apk\repositories.
set "AKMESH_ROOT=%HERE%\root"
set "PATH=%HERE%\bin;%HERE%\root\usr\bin;%HERE%\root\bin;%PATH%"
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
