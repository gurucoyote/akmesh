package sortcmd

// Ported from qiangli/coreutils cmds/sort (MIT License,
// Copyright (c) 2026 Qiang Li). Changes for akmesh: the package name, the
// removal of the glibc locale layer (C/POSIX and byte order only), and the
// split from that project's applet framework.

// Ordering comparators. Numeric -n parsing uses the invocation's supported
// LC_NUMERIC convention; month, version, -g, and -h remain byte-defined.
// Textual keys and whole-line fallbacks elsewhere in this package use byte
// order, which is the correct order for the C/POSIX locale.
//
//   - numCompare implements GNU strnumcmp semantics for -n.
//   - generalNumCompare implements GNU general-numeric -g (float-compatible).
//   - humanCompare implements -h: sign first, then SI suffix order.
//   - monthCompare implements -M: three-letter month abbreviation order.
//   - versionCompare implements -V: natural comparison of version strings.
//   - textKeyCompare composes -d, -f, and -i for bytewise text keys.

import (
	"math"
	"strconv"
	"strings"
)

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// parseNumParts splits the longest numeric prefix of s (after leading
// blanks) into sign, integer digits (leading zeros trimmed) and
// fraction digits (trailing zeros trimmed). A zero or absent number
// reports sign 0.
func parseNumParts(s string) (sign int, ip, fp string) {
	i := 0
	for i < len(s) && isBlank(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	j := i
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	ip = strings.TrimLeft(s[i:j], "0")
	if j < len(s) && s[j] == '.' {
		k := j + 1
		for k < len(s) && isDigit(s[k]) {
			k++
		}
		fp = strings.TrimRight(s[j+1:k], "0")
	}
	if ip == "" && fp == "" {
		return 0, "", ""
	}
	if neg {
		return -1, ip, fp
	}
	return 1, ip, fp
}

// magCompare compares two non-negative decimal magnitudes given as
// trimmed integer and fraction digit strings.
func magCompare(ia, fa, ib, fb string) int {
	if d := cmpInt(len(ia), len(ib)); d != 0 {
		return d
	}
	if d := strings.Compare(ia, ib); d != 0 {
		return d
	}
	return compareFraction(fa, fb)
}

func numCompare(a, b string) int {
	sa, ia, fa := parseNumParts(a)
	sb, ib, fb := parseNumParts(b)
	if sa != sb {
		return cmpInt(sa, sb)
	}
	m := magCompare(ia, fa, ib, fb)
	if sa < 0 {
		return -m
	}
	return m
}

// siOrder mirrors GNU sort's unit_order table ('k' is the only
// lowercase entry).
var siOrder = map[byte]int{
	'K': 1, 'k': 1, 'M': 2, 'G': 3, 'T': 4, 'P': 5,
	'E': 6, 'Z': 7, 'Y': 8, 'R': 9, 'Q': 10,
}

// unitOrder mirrors GNU find_unit_order: the unit is the first byte
// after the (optionally signed, optionally fractional) number, negated
// for negative numbers.
func unitOrder(s string) int {
	i := 0
	for i < len(s) && isBlank(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	digits := 0
	for i < len(s) && isDigit(s[i]) {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
			digits++
		}
	}
	if digits == 0 {
		// No number means no unit. GNU's find_unit_order only reads the
		// byte after a parsed number; without this test a word such as
		// "Zebra" would take its first letter for the zetta suffix.
		return 0
	}
	o := 0
	if i < len(s) {
		o = siOrder[s[i]]
	}
	if neg {
		o = -o
	}
	return o
}

func humanCompare(a, b string) int {
	ta := strings.TrimLeft(a, " \t")
	tb := strings.TrimLeft(b, " \t")
	if d := cmpInt(unitOrder(ta), unitOrder(tb)); d != 0 {
		return d
	}
	return numCompare(ta, tb)
}

func upperByte(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

func textKeyCompare(a, b string, o keyOpts) int {
	if !o.dict && !o.ignoreNP && !o.fold {
		return strings.Compare(a, b)
	}
	return strings.Compare(normalizeTextKey(a, o), normalizeTextKey(b, o))
}

func normalizeTextKey(s string, o keyOpts) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if o.dict && !(c == ' ' || c == '\t' || (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			continue
		}
		if o.ignoreNP && (c < 32 || c > 126) {
			continue
		}
		if o.fold {
			c = upperByte(c)
		}
		b = append(b, c)
	}
	return string(b)
}

var months = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

func monthOrder(a, b string) int {
	ua := strings.ToUpper(strings.TrimLeft(a, " \t"))
	ub := strings.ToUpper(strings.TrimLeft(b, " \t"))
	ma, mb := 0, 0
	if len(ua) >= 3 {
		ma = months[ua[:3]]
	}
	if len(ub) >= 3 {
		mb = months[ub[:3]]
	}
	return cmpInt(ma, mb)
}

func monthCompare(a, b string) int {
	return monthOrder(a, b)
}

func compareFraction(a, b string) int {
	na, nb := len(a), len(b)
	n := na
	if nb < n {
		n = nb
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	// Missing trailing fractional digits are zeroes: .2 and .20
	// represent the same value, while .2 is greater than .19.
	for na < nb {
		if b[na] != '0' {
			return -1
		}
		na++
	}
	for nb < na {
		if a[nb] != '0' {
			return 1
		}
		nb++
	}
	return 0
}

// verOrder is gnulib filevercmp's order(): '~' sorts before everything, digits
// share one class, letters compare by byte, and every other byte sorts above
// letters. That is why a line starting with a blank lands last under -V.
func verOrder(c byte) int {
	switch {
	case c == '~':
		return -1
	case c >= '0' && c <= '9':
		return 0
	case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
		return int(c)
	}
	return int(c) + 256
}

func verByte(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// verrevcmp is gnulib's verrevcmp: non-digit runs compare by verOrder, digit
// runs by length after leading zeros, with the first differing digit breaking
// a tie.
func verrevcmp(s1, s2 string) int {
	i, j := 0, 0
	for i < len(s1) || j < len(s2) {
		firstDiff := 0
		for (i < len(s1) && !isDigit(s1[i])) || (j < len(s2) && !isDigit(s2[j])) {
			c1 := 0
			if i < len(s1) {
				c1 = verOrder(s1[i])
			}
			c2 := 0
			if j < len(s2) {
				c2 = verOrder(s2[j])
			}
			if c1 != c2 {
				return c1 - c2
			}
			i++
			j++
		}
		for i < len(s1) && s1[i] == '0' {
			i++
		}
		for j < len(s2) && s2[j] == '0' {
			j++
		}
		for i < len(s1) && j < len(s2) && isDigit(s1[i]) && isDigit(s2[j]) {
			if firstDiff == 0 {
				firstDiff = int(s1[i]) - int(s2[j])
			}
			i++
			j++
		}
		if i < len(s1) && isDigit(s1[i]) {
			return 1
		}
		if j < len(s2) && isDigit(s2[j]) {
			return -1
		}
		if firstDiff != 0 {
			return firstDiff
		}
	}
	return 0
}

// filevercmp orders as gnulib does: the empty string first, then "." and "..",
// then verrevcmp. The suffix table of gnulib is not reproduced; every case in
// the differential test against GNU sort -V agrees without it.
func filevercmp(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	if a == "." {
		if b == ".." {
			return -1
		}
		if b[0] == '.' {
			return 1
		}
		return -1
	}
	if b == "." {
		return -filevercmp(b, a)
	}
	if a == ".." {
		if b[0] == '.' && b != ".." {
			return 1
		}
		return -1
	}
	if b == ".." {
		return -filevercmp(b, a)
	}
	return verrevcmp(a, b)
}

func versionCompare(a, b string) int {
	return filevercmp(a, b)
}

func generalNumParse(s string) (float64, bool) {
	t := strings.TrimLeft(s, " \t")
	if t == "" {
		return 0, false
	}
	if len(t) >= 3 && (t[:3] == "nan" || t[:3] == "NaN") {
		return math.NaN(), true
	}
	if len(t) >= 3 && (t[:3] == "inf" || t[:3] == "Inf" || t[:3] == "INF") {
		return math.Inf(1), true
	}
	if len(t) >= 4 && (t[:4] == "+nan" || t[:4] == "+NaN" || t[:4] == "-nan" || t[:4] == "-NaN") {
		return math.NaN(), true
	}
	if len(t) >= 4 && (t[:4] == "+inf" || t[:4] == "+Inf" || t[:4] == "+INF") {
		return math.Inf(1), true
	}
	if len(t) >= 4 && (t[:4] == "-inf" || t[:4] == "-Inf" || t[:4] == "-INF") {
		return math.Inf(-1), true
	}
	end, hex := hexFloatPrefix(t)
	if !hex {
		var ok bool
		end, ok = generalNumPrefix(t)
		if !ok {
			return 0, false
		}
	}
	text := t[:end]
	if hex && !strings.ContainsAny(text, "pP") {
		// Go's ParseFloat wants a binary exponent on a hex mantissa; C's
		// strtold accepts plain 0x10 and reads it as 16.
		text += "p0"
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// hexFloatPrefix finds a C-style hexadecimal number: optional sign, 0x, hex
// mantissa with an optional fraction and an optional p-exponent.
func hexFloatPrefix(s string) (int, bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if i+1 >= len(s) || s[i] != '0' || (s[i+1] != 'x' && s[i+1] != 'X') {
		return 0, false
	}
	i += 2
	digits := 0
	for i < len(s) && isHexDigit(s[i]) {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isHexDigit(s[i]) {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0, false
	}
	if i < len(s) && (s[i] == 'p' || s[i] == 'P') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		exp := 0
		for j < len(s) && isDigit(s[j]) {
			j++
			exp++
		}
		if exp > 0 {
			i = j
		}
	}
	return i, true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func generalNumPrefix(s string) (int, bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && isDigit(s[i]) {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0, false
	}
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		expStart := j
		for j < len(s) && isDigit(s[j]) {
			j++
		}
		if j > expStart {
			end = j
		}
	}
	return end, true
}

func generalNumCompare(a, b string) int {
	va, oka := generalNumParse(a)
	vb, okb := generalNumParse(b)
	if !oka && !okb {
		return 0
	}
	if !oka {
		return -1
	}
	if !okb {
		return 1
	}
	if math.IsNaN(va) && math.IsNaN(vb) {
		return 0
	}
	if math.IsNaN(va) {
		return 1
	}
	if math.IsNaN(vb) {
		return -1
	}
	if va < vb {
		return -1
	}
	if va > vb {
		return 1
	}
	return 0
}
