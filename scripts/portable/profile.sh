# akmesh portable bundle: read by "sh -l" (start.cmd names this file through
# AKMESH_PROFILE). Everything here is plain shell.

# The bundle's own programs, then what Windows already offers. The separator is
# the platform's: a shell splits PATH the way its system does, ";" on Windows and
# ":" elsewhere, and one wrong character glues two entries into one that does not
# exist (measured 26.11.2026: "ls: executable file not found in $PATH" while the
# file sat right there).
if [ -n "$PATHEXT" ]; then
	_sep=";"
else
	_sep=":"
fi
case "${_sep}$PATH${_sep}" in
*"${_sep}$HOME/.local/bin${_sep}"*) ;;
*) PATH="$HOME/.local/bin${_sep}$PATH" ;;
esac
export PATH

# Two lines, place first, then who and the sign. The shell prints the window
# title from the same string.
PS1='\e]0;\h \w\a\h:\w\n\u\$ '
export PS1

# A login shell starts where a person expects to be: in the home directory.
cd "$HOME" 2>/dev/null || true

# Not a shell feature but a convenience people miss after switching: two letters
# for "leave", and "cls" as on the Windows side.
if command -v clear >/dev/null 2>&1; then
	cls() { clear "$@"; }
fi
