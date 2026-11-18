#!/bin/sh
# Stands in for go when makebb builds acmesh: drops the flags TinyGo does not
# know and adds -no-debug. makebb is called with -compiler pointing here.
set -u

clean=""

skip_value=0
for a in "$@"; do
	if [ "$skip_value" = 1 ]; then
		skip_value=0
		continue
	fi
	case "$a" in
	-mod=*|-mod|--mod)
		# TinyGo has no module switch; the generated tree carries vendor/.
		[ "$a" = "-mod" ] && skip_value=1
		continue
		;;
	-buildid=*|-buildvcs=*)
		continue
		;;
	esac
	clean="$clean $a"
done

# Every call is logged so a failed build can be read back.
echo "tinygo-args:$*" >> /tmp/tinygo-calls.log

# -no-debug drops the tables we do not need, -panic=trap drops the panic texts.
exec /root/3rd/tinygo44/bin/tinygo $clean -no-debug -panic=trap
