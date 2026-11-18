#!/usr/bin/env python3
"""Writes a minimal PDF for the battery: one page with one line of text. The
fixture is generated instead of checked in, so the claim and the file stay
next to each other."""
import sys

content = b"BT /F1 12 Tf 72 700 Td (Hallo PDF) Tj ET"
objects = [
    b"<< /Type /Catalog /Pages 2 0 R >>",
    b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    b"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
    b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
    b"<< /Length " + str(len(content)).encode() + b" >>\nstream\n" + content + b"\nendstream",
]
out = bytearray(b"%PDF-1.4\n")
for i, body in enumerate(objects, start=1):
    out += str(i).encode() + b" 0 obj\n" + body + b"\nendobj\n"
out += b"trailer\n<< /Root 1 0 R >>\n%%EOF\n"
with open(sys.argv[1] if len(sys.argv) > 1 else "test.pdf", "wb") as f:
    f.write(out)
