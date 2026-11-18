#!/usr/bin/env python3
"""Writes a minimal RTF for the battery: two paragraphs, one bold word and one
character as a code page byte. The fixture is generated so the claim and the
file stay next to each other."""
import sys

doc = "{\\rtf1\\ansi\\ansicpg1252\\pard Eins\\par \\b Zwei\\b0 \\'e4\\par}"
with open(sys.argv[1] if len(sys.argv) > 1 else "test.rtf", "w") as f:
    f.write(doc)
