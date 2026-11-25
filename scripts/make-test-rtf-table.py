#!/usr/bin/env python3
"""Writes a minimal RTF with a table, a hyperlink and a two-level list.

The battery checks that rtf2md reads all three: before acmesh-crc9 the cells of
a row ran together, a link vanished with its text and a list lost its items.
The fixture is generated so that the claim and the file stay next to each other.
"""
import sys

doc = (
    "{\\rtf1\\ansi\\ansicpg1252\\pard Vorher\\par "
    "\\trowd\\trhdr\\cellx2000\\cellx4000 Name\\cell Wert\\cell\\row "
    "\\trowd\\cellx2000\\cellx4000 Alpha\\cell Eins\\cell\\row "
    "\\pard Ein {\\field{\\*\\fldinst{HYPERLINK \"https://example.com/x\"}}"
    "{\\fldrslt{Verweis}}} im Satz.\\par "
    "\\pard\\li360{\\pntext\\f0 1.\\tab}\\pn\\pnlvlbody\\li360 Eins\\par "
    "\\pard\\li1080{\\pntext\\f0 a)\\tab}\\pn\\pnlvlblt\\li1080 Zwei\\par}"
)

with open(sys.argv[1] if len(sys.argv) > 1 else "table.rtf", "w") as f:
    f.write(doc)
