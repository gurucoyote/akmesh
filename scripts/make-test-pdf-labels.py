#!/usr/bin/env python3
"""A PDF whose form labels and values sit in one run each, with no cell gap.

Six rows of "Label: value", every row its own text object on the same page, the
way a form is usually written. pdf2md has to turn this into a table; before the
label-value rule it came out as six lines of prose.
"""

import sys

out_name = sys.argv[1] if len(sys.argv) > 1 else "test-labels.pdf"

rows = [
    ("Ort:", "Pumpkin Mart"),
    ("Datum:", "31. Oktober"),
    ("Kosten:", "12 EUR"),
    ("Anmeldung:", "bis 24. Oktober"),
    ("Treffpunkt:", "Hofladen"),
    ("Hinweis:", "warme Kleidung"),
]

content = []
y = 760
for label, value in rows:
    text = (label + " " + value).replace("(", r"\(").replace(")", r"\)")
    content.append("BT /F1 11 Tf 72 %d Td (%s) Tj ET" % (y, text))
    y -= 18
stream = "\n".join(content).encode("latin-1")

objects = [
    b"<< /Type /Catalog /Pages 2 0 R >>",
    b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
    b"<< /Length " + str(len(stream)).encode() + b" >>\nstream\n" + stream + b"\nendstream",
    b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
]

out = bytearray(b"%PDF-1.4\n")
offsets = []
for i, body in enumerate(objects, 1):
    offsets.append(len(out))
    out += str(i).encode() + b" 0 obj\n" + body + b"\nendobj\n"
xref = len(out)
out += b"xref\n0 " + str(len(objects) + 1).encode() + b"\n0000000000 65535 f \n"
for off in offsets:
    out += ("%010d 00000 n \n" % off).encode()
out += b"trailer\n<< /Size " + str(len(objects) + 1).encode() + b" /Root 1 0 R >>\nstartxref\n" + str(xref).encode() + b"\n%%EOF\n"

with open(out_name, "wb") as f:
    f.write(out)
print(out_name, len(out), "bytes")
