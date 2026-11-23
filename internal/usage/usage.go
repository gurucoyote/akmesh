// Package usage carries the short help of the commands that had none: one
// table, one printer, so that 39 texts do not grow apart (acmesh-py8).
//
// The texts describe the flags the implementation really reads, not the flags
// of GNU. Where GNU's own help is longer, ours stays short on purpose: an agent
// needs the spelling, not the manual.
//
// Measured against Debian bookworm (coreutils 9.1, mawk, net-tools hostname) on
// 19.11.2026; the numbers are in docs/measurements.md, section "Hilfe-Texte".
package usage

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Entry is one command's help: the part after the name on the usage line and
// the flags, one per line and already indented.
type Entry struct {
	Synopsis string
	Flags    string
}

// exit pins the commands whose help does not end with 0: GNU false reports its
// usage and still exits 1 (measured 19.11.2026).
var exit = map[string]int{"false": 1}

// ExitCode is the status --help leaves behind for name (0 unless pinned above).
func ExitCode(name string) int { return exit[name] }

// Texts is the help of every command that answers --help from here. A command
// with its own help (curl, wget, jq, ps, policy, sh, ssh, scp, xargs, chmod,
// file, sort, shuf, the markdown converters) is deliberately absent: its text
// is longer than a table entry and already correct.
var Texts = map[string]Entry{
	"awk": {
		"[-F SEP] [-v VAR=VALUE]... 'PROGRAM' [FILE...]",
		"  -F SEP        field separator for input\n" +
			"  -v VAR=VALUE  assign a variable before the program runs\n" +
			"  -f FILE       read the program from FILE\n" +
			"  -e PROGRAM    add a program text (repeatable)",
	},
	"base64": {
		"[-d] [-i] [-w COLS] [FILE]",
		"  -d                decode instead of encode\n" +
			"  -i                ignore characters outside the alphabet when decoding\n" +
			"  -w COLS           wrap output at COLS characters (default 76, 0 = one line)",
	},
	"basename": {
		"NAME [SUFFIX]\n       basename [OPTION]... NAME...",
		"  -a, --multiple        all arguments are names\n" +
			"  -s, --suffix=SUFFIX   remove SUFFIX from each name\n" +
			"  -z, --zero            end each line with NUL, not newline",
	},
	"comm": {
		"[-1] [-2] [-3] [--total] [--check-order] [--output-delimiter=STR] FILE1 FILE2",
		"  -1, -2, -3               suppress column 1, 2 or 3 (glued: -12, -123)\n" +
			"  --total                  print the counts and their sum at the end\n" +
			"  --check-order            fail when a file is not sorted\n" +
			"  --output-delimiter=STR   separate the columns with STR instead of a tab\n" +
			"  A file name of - reads standard input.",
	},
	"cp": {
		"[-rR] [-f] [-i] [-n] [-p] [-v] SOURCE... DEST",
		"  -r, -R, --recursive   copy directories recursively\n" +
			"  -f, --force           remove the destination if it cannot be opened\n" +
			"  -i, --interactive     ask before overwriting\n" +
			"  -n, --no-clobber      never overwrite an existing file\n" +
			"  -p, --preserve        keep mode, ownership and times\n" +
			"  -a, --archive         -r -p\n" +
			"  -P, --no-dereference  never follow a symbolic link\n" +
			"  -t DIR                copy all sources into DIR\n" +
			"  -v, --verbose         name each file copied",
	},
	"cut": {
		"(-b LIST | -c LIST | -f LIST) [-d CHAR] [-s] [--complement] [-o] [FILE...]",
		"  -b, --bytes=LIST        select bytes\n" +
			"  -c, --characters=LIST   select characters\n" +
			"  -f, --fields=LIST       select fields, separated by -d\n" +
			"  -d, --delimiter=CHAR    field delimiter (default TAB)\n" +
			"  -s, --only-delimited    print no line without a delimiter\n" +
			"      --complement        invert the selection\n" +
			"  -o                      use -d between output fields too",
	},
	"df": {
		"[-h] [-k|-m|-g|-b] [-i] [-T] [-P] [FILE...]",
		"  -h, --human-readable  sizes with a unit suffix\n" +
			"  -k, -m, -g, -b        unit: kilo, mega, giga, bytes\n" +
			"  -i, --inodes          inode counts instead of blocks\n" +
			"  -T, --print-type      print the file system type\n" +
			"  -P, --portability     POSIX one-line format",
	},
	"diff": {
		"[-u] [-q] [-r] [-N] [-i] [-w] [-b] [-L LABEL] FILE1 FILE2",
		"  -u, --unified         unified diff (the form agents read)\n" +
			"  -q, --brief           only say whether the files differ\n" +
			"  -r, --recursive       compare directories\n" +
			"  -N, --new-file        treat a missing file as empty\n" +
			"  -i, --ignore-case     ignore case\n" +
			"  -w, --ignore-all-space  ignore all white space\n" +
			"  -b, --ignore-space-change  ignore changes in white space\n" +
			"  -L LABEL, --label=LABEL    use LABEL instead of the file name",
	},
	"dirname": {
		"NAME...",
		"  -z, --zero   end each line with NUL, not newline",
	},
	"du": {
		"[-h] [-s] [-k|-m|-g|-b] [-d DEPTH] [FILE...]",
		"  -h, --human-readable  sizes with a unit suffix\n" +
			"  -s, --summarize       only a total per argument\n" +
			"  -k, -m, -g, -b        unit: kilo, mega, giga, bytes\n" +
			"  -d, --max-depth=DEPTH limit the depth of the report",
	},
	"env": {
		"[-i] [-0] [-u NAME]... [NAME=VALUE]... [COMMAND [ARG]...]",
		"  -i, --ignore-environment  start with an empty environment\n" +
			"  -0, --null                end each line with NUL, not newline\n" +
			"  -u NAME, --unset=NAME     remove NAME from the environment",
	},
	"echo": {
		"[SHORT-OPTION]... [STRING]...\n       echo LONG-OPTION",
		"  -n    no trailing newline\n" +
			"  -e    interpret backslash escapes\n" +
			"  -E    do not interpret backslash escapes (the default)\n" +
			"--help alone prints this text; with any other word it is echoed like\n" +
			"every other argument (GNU, measured 19.11.2026).",
	},
	"false": {
		"[ignored command line arguments]",
		"Exit with a status code indicating failure (1). Arguments are ignored.",
	},
	"find": {
		"[PATH...] [EXPRESSION]",
		"  -name PATTERN   file name matches the shell pattern\n" +
			"  -path PATTERN   full path matches the shell pattern\n" +
			"  -regex PATTERN  full path matches the regular expression\n" +
			"  -type TYPE      f file, d directory, l symbolic link\n" +
			"  -maxdepth N     do not descend below N levels\n" +
			"  -mindepth N     do not report above N levels\n" +
			"  -not, !         negate the next test\n" +
			"  -print          print the match (the default)\n" +
			"  -print0         print the match, NUL-terminated\n" +
			"  -exec CMD {} ;  run CMD on each match\n" +
			"  -d              depth first",
	},
	"free": {
		"[-h] [-k|-m|-g|-b] [-t] [-w]",
		"  -h, --human-readable  sizes with a unit suffix\n" +
			"  -k, -m, -g, -b        unit: kilo, mega, giga, bytes\n" +
			"  -t, --total           add a total line\n" +
			"  -w, --wide            wide output",
	},
	"grep": {
		"[-E] [-F] [-P] [-i] [-v] [-w] [-c] [-l] [-L] [-n] [-h] [-q] [-o] [-a] [-I] [-r] [-A N] [-B N] [-C N] [-e PATTERN] [PATTERN] [FILE...]",
		"  -E, --extended-regexp   PATTERN is an extended regular expression\n" +
			"  -F, --fixed-strings     PATTERN is a literal string\n" +
			"  -P, --perl-regexp       PATTERN is a Perl/PCRE2 regular expression\n" +
			"  -i, --ignore-case       ignore case\n" +
			"  -a, --text              read a binary file as text\n" +
			"  -I                      a binary file counts as no match\n" +
			"      --binary-files=TYPE text, binary or without-match\n" +
			"  -L, --files-without-match  name the files where nothing matched\n" +
			"  -v, --invert-match      select non-matching lines\n" +
			"  -w, --word-regexp       match whole words only\n" +
			"  -c, --count             print a count of matches per file\n" +
			"  -l, --files-with-matches  only the names of matching files\n" +
			"  -n, --line-number       prefix each line with its line number\n" +
			"  -h, --no-filename       never print the file name\n" +
			"  -q, --quiet             no output, only the exit status\n" +
			"  -o, --only-matching     print only the matching part\n" +
			"  -r, --recursive         search directories recursively\n" +
			"  -A N, --after-context=N   print N lines after a match\n" +
			"  -B N, --before-context=N  print N lines before a match\n" +
			"  -C N, --context=N         print N lines around a match\n" +
			"  -e PATTERN, --regexp=PATTERN  the pattern (repeatable)\n" +
			"  -x, --line-regexp       the whole line has to match\n" +
			"  -H, --with-filename     always print the file name\n" +
			"  -a, --text              read binary files as text (there is no binary\n" +
			"                          detection, so this is the only behaviour)\n" +
			"  -z, --null-data         records are separated by NUL instead of newline\n" +
			"  -m NUM, --max-count=NUM stop after NUM matching lines per file\n" +
			"  --include=GLOB          only search files whose name matches GLOB\n" +
			"  --exclude=GLOB          skip files whose name matches GLOB",
	},
	"head": {
		"[-n N] [-c N] [-q] [-v] [FILE...]",
		"  -n N, --lines=N   print the first N lines (default 10)\n" +
			"  -c N, --bytes=N   print the first N bytes\n" +
			"  -q, --quiet       never print a file name header\n" +
			"  -v, --verbose     always print a file name header",
	},
	"certinfo": {
		"[-j] [--resolve HOST:PORT:ADDR] FILE | HOST[:PORT]",
		"  -j, --json  machine-readable output\n" +
			"  --resolve   connect to ADDR for that host and port, the name stays;\n" +
			"              this is how a local name like test1.local is read without\n" +
			"              an entry in /etc/hosts\n" +
			"\n" +
			"A file prints every certificate in it, in order; a host prints the chain\n" +
			"the server sends, leaf first. The operand is a file when it exists, a host\n" +
			"otherwise.\n" +
			"\n" +
			"Limits: this only reads. It does not check the chain and does not look at\n" +
			"revocation (OCSP, CRL, Certificate Transparency); that is what verify is\n" +
			"for, and even there revocation is not covered. A server is dialled with\n" +
			"TLS as it comes, so certinfo also works against expired or self-signed\n" +
			"hosts.",
	},
	"hexdump": {
		"[-C] [-n LENGTH] [FILE...]",
		"  -C, --canonical   canonical hex and ASCII view\n" +
			"  -n LENGTH         read at most LENGTH bytes",
	},
	"hostname": {
		"",
		"Print the host name. Setting the name is not supported.",
	},
	"iconv": {
		"[-f FROM] [-t TO] [-c] [-l] [FILE]",
		"  -f, --from-code=FROM  source encoding\n" +
			"  -t, --to-code=TO      target encoding\n" +
			"  -c                    drop characters that cannot be converted\n" +
			"  -l, --list            list the known encodings",
	},
	"chown": {
		"[-R] [-h] [-v] [-c] OWNER[:GROUP] FILE...",
		"  -R, --recursive       change directories and everything inside them\n" +
			"  -h, --no-dereference  change a symbolic link itself, not its target\n" +
			"  -v, --verbose         name every file that was changed\n" +
			"  -c, --changes         name only the files whose owner really changed\n" +
			"  OWNER is a name from /etc/passwd or a number; \":GROUP\", \"OWNER.GROUP\"\n" +
			"  and a bare \":GROUP\" work as in coreutils. Not supported on Windows.",
	},
	"mkpasswd": {
		"[-m sha-512|sha-256] [-s SALT] [PASSWORD]",
		"  -m METHOD   sha-512 (default) or sha-256; the crypt hash in /etc/shadow\n" +
			"  -s SALT     use this salt instead of a random one\n" +
			"  Without PASSWORD the password is read from standard input.",
	},
	"od": {
		"[-A RADIX] [-t TYPE]... [-N BYTES] [-j BYTES] [-v] [FILE]",
		"  -A d|o|x|n   address radix: decimal, octal (default), hex or none\n" +
			"  -t TYPE      a, c, dN, oN, uN, xN or f4/f8; repeatable, one line per type\n" +
			"  -N BYTES     stop after this many bytes\n" +
			"  -j BYTES     skip this many bytes first\n" +
			"  -v           print repeated blocks instead of a *\n" +
			"  The form agents type, \"od -An -tx1\", works: hex bytes, no address.",
	},
	"passwd": {
		"[USER]",
		"  Sets a password: asks for it twice, hashes it with SHA-512 crypt and\n" +
			"  rewrites the line in /etc/shadow. Without USER the own account is meant.\n" +
			"  Changing another account needs root. Not supported on Windows.",
	},
	"setsid": {
		"[-w] [-f] PROGRAM [ARGS...]",
		"  -w, --wait   wait for the program and return its exit code (default)\n" +
			"  -f, --fork   start it and return at once\n" +
			"  The program runs in its own session, so it survives the terminal.\n" +
			"  Not supported on Windows, where a process does not die with its console.",
	},
	"su": {
		"[-] [-c COMMAND] [USER]",
		"  -c COMMAND   run this command through the user's shell instead of starting it\n" +
			"  -            a login shell: the user's home and environment\n" +
			"  Without root the target user's password is asked for. Not supported on\n" +
			"  Windows, which has no setuid.",
	},
	"sum": {
		"[-r] [-s] [FILE...]",
		"  -r, --bsd    the BSD rotating checksum, the default\n" +
			"  -s, --sysv   the System V 16 bit checksum\n" +
			"  Prints the checksum, the size in 1K blocks and the name. For real\n" +
			"  integrity checks use md5sum or sha256sum.",
	},
	"unzip": {
		"[-l] [-o] [-q] [-d DIR] FILE [MEMBER...]",
		"  -l, --list        list the entries instead of extracting\n" +
			"  -d DIR            extract into DIR\n" +
			"  -o, --overwrite   replace files that already exist; without it they are\n" +
			"                    reported and left alone\n" +
			"  -q, --quiet       print nothing per file\n" +
			"  -p                write one member to standard output\n" +
			"  Members stay inside DIR: \"../\" in an archive is stripped. Encrypted\n" +
			"  entries cannot be read.",
	},
	"kill": {
		"[-SIGNAL | -s SIGNAL] PID...\n       kill -l [SIGNAL]",
		"  -l, --list        list the signal names\n" +
			"  -s SIGNAL         the signal to send by name or number\n" +
			"  A signal may also be written as -SIGNAL or -NUMBER.",
	},
	"killall": {
		"[-SIGNAL | -s SIGNAL] [-i] [-q] [-w] [-l] NAME...",
		"  -s SIGNAL     the signal to send by name or number\n" +
			"  -i            ask before killing\n" +
			"  -q, --quiet   print nothing on success\n" +
			"  -w, --wait    wait until the process has really exited (30s limit)\n" +
			"  -l, --list    list the signal names\n" +
			"  The name is compared exactly and without .exe, so killall sleep never\n" +
			"  catches mysleep.",
	},
	"pgrep": {
		"[-flxanc] [-u USER] [-P PPID] [-d SEP] PATTERN",
		"  -f, --full        match against the whole command line\n" +
			"  -x, --exact       the pattern has to cover the whole name\n" +
			"  -u USER           only processes owned by this user\n" +
			"  -P PPID           only children of this process\n" +
			"  -n, --newest      only the youngest match\n" +
			"  -o, --oldest      only the oldest match\n" +
			"  -l, --list-name   print the process name after the pid\n" +
			"  -a, --list-full   print the full command line after the pid\n" +
			"  -c, --count       print only the number of matches\n" +
			"  -d SEP            separator between two pids, default a newline\n" +
			"  PATTERN is a regular expression and may be left out when -u or -P\n" +
			"  already says which processes are meant; exit 1 means nothing matched.",
	},
	"pkill": {
		"[-SIGNAL | -s SIGNAL] [-f] [-x] [-u USER] [-P PPID] PATTERN",
		"  -f, --full    match against the whole command line\n" +
			"  -x, --exact   the pattern has to cover the whole name\n" +
			"  -u USER       only processes owned by this user\n" +
			"  -P PPID       only children of this process\n" +
			"  -s SIGNAL     the signal to send by name or number; -TERM or -9\n" +
			"                work as they do for kill. Default TERM.\n" +
			"  -n NEWEST     only the youngest match, -o OLDEST only the oldest\n" +
			"  The calling process is never signalled; exit 1 means nothing matched.\n" +
			"  -f matches the whole command line, so it can also match the shell that\n" +
			"  called pkill when that shell has the pattern in its arguments: run\n" +
			"  pgrep with the same flags first.",
	},
	"mkcert": {
		"-host NAME[,NAME...] [-days N] [-key-type ecdsa|rsa|ed25519] [-out BASE] [-ca BASE] [-ca-domains SUFFIX] [-org NAME]",
		"  -host NAMES       names to certify, comma separated; an address becomes an IP SAN\n" +
			"  -days N           how long the leaf is valid, default 365\n" +
			"  -key-type KIND    ecdsa (P-256, default), rsa (2048) or ed25519\n" +
			"  -out BASE         writes BASE-cert.pem and BASE-key.pem, default the first name\n" +
			"  -ca BASE          sign with BASE-cert.pem and BASE-key.pem, creating them\n" +
			"                    on first use; without -ca the leaf signs itself\n" +
			"  -install          put BASE-cert.pem into the system trust store (root rights\n" +
			"                    on Linux, the user's root store on Windows); the file there is\n" +
			"                    named after the build name (akmesh-BASE.crt), not -org; a CA that\n" +
			"                    does not exist yet is created and installed in one go\n" +
			"  -uninstall        take it out again\n" +
			"  -org NAME         organisation in the certificate subject, the issuer a\n" +
			"                    reader sees; default the build name plus \" mkcert\"; AKMESH_CERT_ORG\n" +
			"                    sets it for a session. A CA that already exists keeps its\n" +
			"                    issuer: the subject is in every certificate it signed and in\n" +
			"                    every store that trusts it, so -org applies to a new CA, made\n" +
			"                    under another base name\n" +
			"  -ca-domains SUF   new CA may only sign names ending in these comma separated\n" +
			"                    suffixes (.local,.test) and no IP address at all; a\n" +
			"                    trusted local root that can vouch for nothing else\n" +
			"\n" +
			"Limits, and they are real: PEM only, so PKCS#12 (.pfx/.p12) and JKS are\n" +
			"refused with a message naming the format. No passphrase, because the\n" +
			"standard library cannot write encrypted PKCS#8; the key lies in the clear\n" +
			"guarded only by mode 0600. No HSM, no PKCS#11, no TPM: signing happens in\n" +
			"memory. Good for test and intranet certificates, not for a real CA.",
	},
	"nl": {
		"[-b STYLE] [-n FORMAT] [-w WIDTH] [-s SEP] [-d DELIM] [FILE]",
		"  -b, --body-numbering=STYLE   a all lines, t text lines only, n none\n" +
			"  -n, --number-format=FORMAT   ln left, rn right, rz right with zeros\n" +
			"  -w, --number-width=WIDTH     width of the number\n" +
			"  -s, --number-separator=SEP   text after the number (default TAB)\n" +
			"  -d, --section-delimiter=DELIM  section delimiter (default \\:)",
	},
	"patch": {
		"[-R] [-N] [-f] [-i FILE] [-p NUM] [--dry-run] [FILE]",
		"  -R, --reverse     apply the patch backwards\n" +
			"  -N, --forward     skip patches that already applied\n" +
			"  -f, --force       apply even when the file looks wrong\n" +
			"  -i FILE, --input=FILE  read the patch from FILE\n" +
			"  -p NUM, --strip=NUM    drop NUM leading directories from file names\n" +
			"      --dry-run     say what would happen without writing",
	},
	"pwd": {
		"",
		"Print the current working directory.",
	},
	"rg": {
		"[-i] [-F] [-P] [-w] [-n] [-l] [-c] [-o] [-A N] [-B N] [-C N] [-g GLOB] [-t TYPE] [--engine E] PATTERN [PATH...]",
		"Recursive, gitignore-aware search over text files; no -r needed.\n" +
			"Exit 0 on a match, 1 on none, 2 on an error.\n" +
			"  -e PATTERN          add a pattern (repeatable)\n" +
			"  -i, --ignore-case   case-insensitive\n" +
			"  -S, --smart-case    case-insensitive unless the pattern has uppercase\n" +
			"  -F, --fixed-strings PATTERN is literal, not a regexp\n" +
			"  -P, --pcre2         search with PCRE2, the way grep -P does\n" +
			"                      (lookahead, lookbehind, \\K, a++)\n" +
			"                      lookbehind, \\K, possessive quantifiers\n" +
			"      --engine E      default, pcre2 or auto (auto falls back to PCRE2\n" +
			"                      when the pattern needs it)\n" +
			"  -w, --word-regexp   match whole words only\n" +
			"  -x, --line-regexp   match whole lines only\n" +
			"  -v, --invert-match  print non-matching lines\n" +
			"  -n, --line-number   print line numbers (off when the output is piped)\n" +
			"  -l, --files-with-matches  print only the names of matching files\n" +
			"  -c, --count         print the match count per file\n" +
			"  -o, --only-matching print only the matching part of a line\n" +
			"  -A/-B/-C N          N lines of after/before/both context\n" +
			"  -m N, --max-count   stop after N matches per file\n" +
			"  -d N, --max-depth   descend at most N directory levels\n" +
			"  -g GLOB, --glob     include (or exclude with !GLOB) paths\n" +
			"  -t TYPE / -T TYPE   include / exclude a known file type (go, py, ...)\n" +
			"      --files         list the files that would be searched\n" +
			"      --hidden        also search dot-files\n" +
			"      --no-ignore     ignore .gitignore, .ignore and .rgignore\n" +
			"  -j N, --threads     worker count\n" +
			"      --pre CMD       search documents through a converter: any2md (the\n" +
			"                      default) reads docx, pdf, epub, rtf and html in\n" +
			"                      process, any other CMD is run with the file path as\n" +
			"                      its last argument. Line and column numbers refer to\n" +
			"                      the converted text, not to the original file.\n" +
			"      --pre-glob GLOB which files go through --pre (default: the any2md\n" +
			"                      suffixes); everything else is searched raw\n" +
			"Not built (unlike ripgrep): --type-add/--type-clear, --encoding,\n" +
			"and the global gitignore file (core.excludesFile).",
	},
	"sed": {
		"[-n] [-i] [-E] [-z] (-e SCRIPT | SCRIPT) [FILE...]",
		"  -e, --expression=SCRIPT  add SCRIPT (repeatable)\n" +
			"  -f, --file=FILE          read the script from FILE\n" +
			"  -n, --quiet, --silent    print nothing unless asked with p\n" +
			"  -i, --in-place           edit the files in place\n" +
			"  -z, --zero-terminated    records end at NUL and are written with NUL\n" +
			"                    instead of a line break (for find -print0)\n" +
			"  -E, -r, --regexp-extended  extended regular expressions\n" +
			"  Short flags may be glued: -zn, -nE.",
	},
	"split": {
		"[-l N] [-b SIZE] [-n N] [-a N] [-d] [--additional-suffix=SUF] [FILE [PREFIX]]",
		"  -l N, --lines=N         N lines per file (default 1000)\n" +
			"  -b SIZE, --bytes=SIZE   SIZE bytes per file, with k/m/g suffix\n" +
			"  -n N, --number=N        split into N files\n" +
			"  -a N, --suffix-length=N suffix length (default 2)\n" +
			"  -d, --numeric-suffixes  numeric instead of alphabetic suffixes\n" +
			"      --additional-suffix=SUF  append SUF to each name",
	},
	"stat": {
		"[-c FORMAT | --printf=FORMAT] [-t] [-f] [-L] FILE...",
		"  -c FORMAT, --format=FORMAT  print FORMAT, then a newline\n" +
			"      --printf=FORMAT         print FORMAT, interpret backslash escapes\n" +
			"  -t, --terse               terse form\n" +
			"  -f, --file-system         file system instead of file status\n" +
			"  -L, --dereference         follow symbolic links",
	},
	"sync": {
		"[FILE...]",
		"Without FILE operands the whole file system is flushed; with operands only\n" +
			"the named files are. Windows has no whole-system flush and says so.",
	},
	"tac": {
		"[-s SEP] [-r] [FILE...]",
		"  -s, --separator=SEP  record separator (default newline)\n" +
			"  -r, --regex          the separator is a regular expression",
	},
	"tar": {
		"-c|-t|-x [-z] [-v] [-f ARCHIVE] [-C DIR] [--strip-components=N] [--exclude=PATTERN] [FILE...]",
		"  -c, --create            create an archive\n" +
			"  -t, --list              list the contents\n" +
			"  -x, --extract, --get    extract\n" +
			"  -z, --gzip, --gunzip    gzip compression, detected on read\n" +
			"  -f ARCHIVE, --file=ARCHIVE  the archive instead of stdin/stdout\n" +
			"  -v, --verbose           name each file\n" +
			"  -C DIR, --directory=DIR change to DIR first\n" +
			"      --strip-components=N  drop N leading directories when extracting\n" +
			"      --exclude=PATTERN     skip names matching PATTERN\n" +
			"      --no-recursion        do not descend into directories",
	},
	"timeout": {
		"[-s SIGNAL] DURATION COMMAND [ARG]...",
		"  -s SIGNAL, --signal=SIGNAL  signal to send (default TERM)\n" +
			"  DURATION takes s, m, h and d suffixes; 0 disables the limit.\n" +
			"  Exit 124 when the limit was hit, 125 when timeout itself failed.",
	},
	"tr": {
		"[-c] [-d] [-s] SET1 [SET2]",
		"  -c, -C, --complement       use the complement of SET1\n" +
			"  -d, --delete               delete characters in SET1\n" +
			"  -s, --squeeze-repeats      squeeze repeated characters\n" +
			"  Ranges like a-z, octal escapes and [:class:] are understood.",
	},
	"uniq": {
		"[-c|-d|-u] [-i] [-f N] [-s N] [-w N] [-z] [FILE...]",
		"  -c, --count              prefix the count in seven columns\n" +
			"  -d, --repeated           print only lines that repeat\n" +
			"  -u, --unique             print only lines that do not repeat\n" +
			"  -i, --ignore-case        compare case insensitively\n" +
			"  -f N, --skip-fields=N    ignore the first N blanks and fields\n" +
			"  -s N, --skip-chars=N     ignore the first N characters after -f\n" +
			"  -w N, --check-chars=N    compare at most N characters\n" +
			"  -z, --zero-terminated    records end at NUL (pair to find -print0)\n" +
			"  Several FILEs are read as one stream.",
	},
	"touch": {
		"[-a] [-m] [-c] [-t STAMP] [-d STRING] [-r FILE] FILE...",
		"  -a                 change only the access time\n" +
			"  -m                 change only the modification time\n" +
			"  -c, --no-create    do not create a missing file\n" +
			"  -t STAMP           [[CC]YY]MMDDhhmm[.ss]\n" +
			"  -d STRING          a date such as '2026-11-19 15:37', RFC3339 or 'now'\n" +
			"  -r FILE, --reference=FILE  use the times of FILE",
	},
	"true": {
		"[ignored command line arguments]",
		"Exit with status 0. Arguments are ignored.",
	},
	"truncate": {
		"[-c] -s SIZE FILE...",
		"  -s SIZE, --size=SIZE  the new size, with k/m/g suffix and + - / % prefixes\n" +
			"  -c, --no-create       do not create a missing file",
	},
	"uname": {
		"[-a] [-s] [-n] [-r] [-v] [-m] [-p] [-i] [-o]",
		"  -a, --all                everything below\n" +
			"  -s, --kernel-name        kernel name (the default)\n" +
			"  -n, --nodename           network node host name\n" +
			"  -r, --kernel-release     kernel release\n" +
			"  -v, --kernel-version     kernel version\n" +
			"  -m, --machine            machine hardware name\n" +
			"  -p, --processor          processor type\n" +
			"  -i, --hardware-platform  hardware platform\n" +
			"  -o, --operating-system   operating system",
	},
	"verify": {
		"[-ca CAFILE] [-host NAME] CERT [CHAIN...]",
		"  -ca CAFILE   trust this CA instead of the system roots; repeatable\n" +
			"  -host NAME   also check that NAME is in the certificate\n" +
			"\n" +
			"Every file after CERT is an intermediate. The exit code is 0 only when the\n" +
			"chain is good; the reason is printed otherwise (expired, wrong host,\n" +
			"unknown authority).\n" +
			"\n" +
			"Limits: a revoked certificate passes. OCSP, CRL and Certificate\n" +
			"Transparency are not consulted, so verify answers 'chain and name are\n" +
			"sound', not 'this certificate is still trusted today'.",
	},
	"id": {
		"[-g|-G|-u] [-n] [-r] [USER]",
		"  -g, --group     print only the effective group ID\n" +
			"  -G, --groups    print all group IDs\n" +
			"  -u, --user      print only the effective user ID\n" +
			"  -n, --name      print a name instead of a number, for -ugG\n" +
			"  -r, --real      print the real ID instead of the effective one\n" +
			"  Without /etc/passwd or /etc/group the numbers are printed, as GNU\n" +
			"  id does on a system without an account database; -n then reports\n" +
			"  the missing name and exits 1.",
	},
	"wc": {
		"[-l] [-w] [-c] [-m] [-L] [-b] [--files0-from=FILE] [FILE...]",
		"  -l, --lines       count lines\n" +
			"  -w, --words       count words\n" +
			"  -c, --bytes       count bytes\n" +
			"  -m, --chars       count characters\n" +
			"  -L, --max-line-length  print the longest line (tabs count to the\n" +
			"                    next multiple of 8, as GNU counts them)\n" +
			"  -b                count broken UTF-8 codes\n" +
			"      --files0-from=FILE  read NUL-separated file names from FILE\n" +
			"  With no letter, lines, words and bytes are reported. Numbers are\n" +
			"  right-aligned to one width, like GNU, and a total line follows two\n" +
			"  or more files.",
	},
}

// sums are four names on one implementation; they share one text.
func init() {
	for _, kind := range []string{"md5", "sha1", "sha256", "sha512"} {
		name := kind + "sum"
		Texts[name] = Entry{
			"[-c] [-b|-t] [--quiet] [--status] [FILE...]",
			"  -c, --check    check the sums in FILE (or stdin) against the files\n" +
				"  -b, --binary   read in binary mode (the default here)\n" +
				"  -t, --text     read in text mode\n" +
				"      --quiet    print only failures when checking\n" +
				"      --status   print nothing, only the exit status",
		}
	}
}

// dashH names the commands where -h means help as well. Measured: GNU hostname
// answers -h with usage, mawk answers "awk: not an option: -h". Everything
// else keeps -h for what its own parser does with it (grep -h is no-filename,
// touch -h is a missing operand), which is what the originals do.
var dashH = map[string]bool{"hostname": true}

// Wanted reports whether this call asks for help. --help counts in any
// position, as getopt would read it; -h only for the names in dashH.
//
// echo is the exception the measurement forces: GNU /usr/bin/echo prints usage
// only when --help is the sole argument and echoes "--help foo" otherwise, so
// its entry is only honoured for the one-argument call (see SoleHelp).
func Wanted(name string, args []string) bool {
	if _, ok := Texts[name]; !ok {
		return false
	}
	for _, a := range args {
		if a == "--help" {
			if name == "echo" {
				return len(args) == 1
			}
			return true
		}
		if a == "-h" && dashH[name] {
			return true
		}
	}
	return false
}

// Func is the shape every command's Run has. The dispatcher, the tests and the
// per-command main packages all pass their implementation through this type.
type Func func(args []string, stdin, stdout, stderr *os.File) int

// Run is the one gate every command goes through: it answers --help when the
// call asks for it and otherwise calls fn. Because the cmd/<name> mains call
// it, a command cannot forget the rule, and a broken parser cannot swallow
// "--help" as an operand (acmesh-py8). Before this, hostname read it as a new
// host name and sync as a file to flush.
func Run(name string, args []string, stdin, stdout, stderr *os.File, fn Func) int {
	if Wanted(name, args) {
		if code, ok := Help(name, stdout); ok {
			return code
		}
	}
	return fn(args, stdin, stdout, stderr)
}

// RunNamed is Run for the packages that serve several commands from one Run
// (sedawk, disk, proc, corecmd, diffpatch, sum): they take the command name as
// their first argument.
func RunNamed(name string, args []string, stdin, stdout, stderr *os.File, fn func(string, []string, *os.File, *os.File, *os.File) int) int {
	return Run(name, args, stdin, stdout, stderr, func(args []string, stdin, stdout, stderr *os.File) int {
		return fn(name, args, stdin, stdout, stderr)
	})
}

// RunIO is Run for the commands whose Run takes io.Reader and io.Writer (the
// forks of u-root and the checksum family), so they need no adapter either.
func RunIO(name string, args []string, stdin io.Reader, stdout, stderr io.Writer, fn func([]string, io.Reader, io.Writer, io.Writer) int) int {
	if Wanted(name, args) {
		if code, ok := Help(name, stdout); ok {
			return code
		}
	}
	return fn(args, stdin, stdout, stderr)
}

// RunNamedIO is RunNamed for the same io-based signature.
func RunNamedIO(name string, args []string, stdin io.Reader, stdout, stderr io.Writer, fn func(string, []string, io.Reader, io.Writer, io.Writer) int) int {
	return RunIO(name, args, stdin, stdout, stderr, func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
		return fn(name, args, stdin, stdout, stderr)
	})
}

// Help writes the help of name and reports the exit status and whether there
// was a text. false keeps its 1, everything else ends with 0.
func Help(name string, w io.Writer) (int, bool) {
	e, ok := Texts[name]
	if !ok {
		return 0, false
	}
	if e.Synopsis == "" {
		fmt.Fprintf(w, "Usage: %s\n", name)
	} else {
		fmt.Fprintf(w, "Usage: %s %s\n", name, e.Synopsis)
	}
	if e.Flags != "" {
		fmt.Fprintf(w, "\n%s\n", strings.TrimRight(e.Flags, "\n"))
	}
	return exit[name], true
}

// Render writes the help of name and reports whether it had one.
func Render(name string, w io.Writer) bool {
	_, ok := Help(name, w)
	return ok
}

// BadOption reports an unknown option the way coreutils does: the message, the
// pointer to --help, and exit 1 (acmesh-4qw). Measured on coreutils 9.1:
//
//	$ sha256sum --bogus
//	sha256sum: unrecognized option '--bogus'
//	Try 'sha256sum --help' for more information.
//	$ echo $?
//	1
//
// timeout reports the same lines but exits 125; it passes that code in.
func BadOption(prog, opt string, stderr io.Writer, code int) int {
	fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", prog, opt)
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", prog)
	return code
}
