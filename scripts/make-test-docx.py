#!/usr/bin/env python3
"""Writes a minimal .docx for the battery: one Heading2 paragraph and one text
paragraph. The battery needs a document it can check, and a checked-in binary
would be one more thing to keep in step."""
import sys
import zipfile

CONTENT_TYPES = (
    '<?xml version="1.0"?>'
    '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
    '<Default Extension="xml" ContentType="application/xml"/></Types>'
)
DOCUMENT = (
    '<?xml version="1.0"?>'
    '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
    '<w:body>'
    '<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr>'
    '<w:r><w:t>Kapitel</w:t></w:r></w:p>'
    '<w:p><w:r><w:t>Text</w:t></w:r></w:p>'
    '</w:body></w:document>'
)

out = sys.argv[1] if len(sys.argv) > 1 else "test.docx"
with zipfile.ZipFile(out, "w") as z:
    z.writestr("[Content_Types].xml", CONTENT_TYPES)
    z.writestr("word/document.xml", DOCUMENT)
