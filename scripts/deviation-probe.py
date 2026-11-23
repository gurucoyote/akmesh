#!/usr/bin/env python3
"""Vergleicht akmesh mit GNU im gepinnten Debian-Image, Aufruf fuer Aufruf.

Ein Fall ist (werkzeug, argumente, stdin). Beide Seiten laufen im gleichen
Verzeichnis mit den gleichen Testdateien; verglichen werden Exitcode, stdout und
ob stderr leer ist. Ausgegeben werden nur die Faelle, die abweichen.
"""
import json
import subprocess
import sys

IMAGE = "docker.io/library/debian@sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980"
FIX = "/tmp/devprobe/w"
AKMESH = "/root/akmesh/akmesh"


def run_akmesh(tool, args, stdin):
    p = subprocess.run([AKMESH, tool] + args, cwd=FIX, input=stdin,
                       capture_output=True, text=True, timeout=60)
    return p.returncode, p.stdout, p.stderr


def run_gnu(tool, args, stdin):
    cmd = ["podman", "run", "--rm", "-i", "-v", FIX + ":/w", "-w", "/w", IMAGE,
           "sh", "-c", 'exec "$0" "$@"', tool] + args
    p = subprocess.run(cmd, input=stdin, capture_output=True, text=True, timeout=120)
    return p.returncode, p.stdout, p.stderr


def cmp_case(case):
    tool, args, stdin = case["tool"], case["args"], case.get("stdin", "")
    grc, gout, gerr = run_gnu(tool, args, stdin)
    arc, aout, aerr = run_akmesh(tool, args, stdin)
    same = (grc == arc) and (gout == aout) and (bool(gerr.strip()) == bool(aerr.strip()))
    return {
        "tool": tool, "args": args, "stdin": stdin,
        "gnu_rc": grc, "ak_rc": arc,
        "gnu_out": gout, "ak_out": aout,
        "gnu_err": gerr.strip().split("\n")[0][:120],
        "ak_err": aerr.strip().split("\n")[0][:120],
        "same": same,
    }


def main(cases):
    out = []
    for c in cases:
        try:
            out.append(cmp_case(c))
        except Exception as e:  # ein kaputter Aufruf darf die Messung nicht beenden
            out.append({"tool": c["tool"], "args": c["args"], "error": str(e)[:120], "same": False})
    diff = [r for r in out if not r["same"]]
    for r in diff:
        print(json.dumps(r, ensure_ascii=False))
    print(f"--- {len(out)} Faelle, {len(diff)} Abweichungen ---")


if __name__ == "__main__":
    main(json.load(open(sys.argv[1])))
