#!/usr/bin/env python3
"""Writes a PDF whose text sits in a form XObject, for the battery. Form fields
and stamps are drawn that way: their own content stream with their own
resources."""
import sys

form = b"BT /F1 11 Tf 5 6 Td (Feldtext) Tj ET"
page = b"q 1 0 0 1 70 700 cm /X1 Do Q"
objects = [
    b"<< /Type /Catalog /Pages 2 0 R >>",
    b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    b"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> /XObject << /X1 6 0 R >> >> /Contents 5 0 R >>",
    b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
    b"<< /Length " + str(len(page)).encode() + b" >>\nstream\n" + page + b"\nendstream",
    b"<< /Type /XObject /Subtype /Form /BBox [0 0 200 20] /Resources << /Font << /F1 4 0 R >> >> /Length "
    + str(len(form)).encode() + b" >>\nstream\n" + form + b"\nendstream",
]
out = bytearray(b"%PDF-1.4\n")
for i, body in enumerate(objects, start=1):
    out += str(i).encode() + b" 0 obj\n" + body + b"\nendobj\n"
out += b"trailer\n<< /Root 1 0 R >>\n%%EOF\n"
with open(sys.argv[1] if len(sys.argv) > 1 else "test.pdf", "wb") as f:
    f.write(out)
