package anymd

// Convert returns data as Markdown, with the format read from the bytes. It is
// the library entry the rg preprocessor uses: the same five readers as the
// command and the same detection, without the argument loop, so a fix in the
// reading path reaches both callers at once.
func Convert(data []byte) (string, error) {
	f, ok := Detect(data)
	if !ok {
		return "", UnknownError("any2md")
	}
	md, _, err := convertAs(f, data, "")
	return md, err
}

// Extensions are the file suffixes whose content Convert understands, without
// the dot. The rg preprocessor searches files with these suffixes through
// Convert when --pre was given and --pre-glob was not.
var Extensions = []string{"pdf", "docx", "epub", "rtf", "html", "htm"}
