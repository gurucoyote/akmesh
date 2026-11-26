@echo off
rem akmesh portable: open a shell whose PATH leads to this folder's bin.
rem
rem bin\ is not shipped as a folder of copies: one file would be 20 MB times
rem every name. It is laid out on first run as hardlinks, which cost no disk and
rem are what makes "sh", "ls", "grep" answer at all - the binary decides by the
rem name it was started as.
rem
rem The names come from names.txt, written next to this file when the bundle was
rem built: asking the binary here worked badly (a nested command inside for /f
rem needs quoting nobody can read later), and a file cannot drift as long as the
rem bundle is rebuilt from the binary.
setlocal
set "HERE=%~dp0"
if "%HERE:~-1%"=="\" set "HERE=%HERE:~0,-1%"

if not exist "%HERE%\bin\.ready" call :links
if not exist "%HERE%\bin\sh.exe" (
	echo akmesh: bin\ could not be laid out. See the note about hardlinks in README.txt.
	pause
	exit /b 1
)

set "PATH=%HERE%\bin;%PATH%"
set "AKMESH_PROFILE=%HERE%\etc\profile.sh"
if not defined HOME set "HOME=%USERPROFILE%"
if not defined TERM set "TERM=xterm-256color"

rem -l makes it a login shell, so it reads the profile above, and every argument
rem reaches the shell unchanged: "start.cmd -c \"echo hi\"" is a one-shot run.
"%HERE%\bin\sh.exe" -l %*
exit /b %ERRORLEVEL%

:links
if not exist "%HERE%\bin" mkdir "%HERE%\bin"
for /f "usebackq delims=" %%n in ("%HERE%\names.txt") do (
	if not exist "%HERE%\bin\%%n.exe" mklink /H "%HERE%\bin\%%n.exe" "%HERE%\akmesh.exe" >nul 2>&1
)
rem The marker keeps the next start from walking the whole list again.
echo ready> "%HERE%\bin\.ready"
exit /b 0
