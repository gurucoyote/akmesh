# Ein eigenes apk-Repository

Was ein Repo enthalten muss, damit `apk` daraus installieren kann, und wie das
Vertrauen von Anfang an funktioniert. Notiert am 26.11.2026 aus dem Client-Code
(`internal/apkcore`, `internal/apkcmd`) und aus Messungen im portablen Bündel.

Der Client ist gebaut und geprüft; der Erzeuger fehlt vollständig
(Ticket acmesh-c474). Gemessen ist bisher nur eine von Hand gebaute lokale
`.apk`: `apk add --allow-untrusted hello2.apk` → `(1/1) Installing hello`,
Datei in `root/usr/bin`, `apk list` zeigt `hello-1.0-r0 x86_64 installed`,
`apk del hello` räumt sie wieder ab.

## 1. Ablage und Adresse

    <repo>/<arch>/APKINDEX.tar.gz
    <repo>/<arch>/<name>-<version>.apk

Die Zeile in `etc/apk/repositories` ist die Basis, ohne Arch:

    https://packages.example.org/akmesh/windows
    https://packages.example.org/akmesh/windows/x86_64     (so liest der Client)

Gemessen: auf Windows meldet der Client `x86_64`; ein Paket mit
`arch = x86_64` installiert ohne Zusatz, `arch = windows-amd64` verlangt
`apk --arch windows-amd64`.

Es genügt ein statischer Ordner: http, https oder ein Verzeichnis auf der
Platte. Kein Server-Programm, keine API.

## 2. Der Index

Ein gzip-Tar mit einem Mitglied `APKINDEX`, optional daneben `DESCRIPTION`
(wird nur für `apk policy` gelesen).

Pro Paket eine Folge von Zeilen, jede mit einem Buchstaben und einem Doppelpunkt:

    C:Q1<base64 der Signatur>
    P:<pkgname>
    V:<pkgver>
    A:<arch>
    S:<Größe des data-Blocks in Bytes>
    I:<datahash>
    T:<Beschreibung>
    D:<Abhängigkeiten, Leerzeichen-getrennt>

Der Index ist aus den `.PKGINFO`-Dateien der Pakete erzeugbar; er trägt dieselben
Werte, nur gebündelt. Unser Client liest ihn mit `apkcore.ParseIndex`.

## 3. Signaturen

Zwei Stellen werden signiert, beide nach demselben Muster: ein Tar-Block mit dem
Mitglied `.SIGN.RSA.<keyname>.rsa.pub`, der **vor** dem signierten Inhalt liegt.

- Index: Signaturblock, dann der Indexblock, alles in einem `APKINDEX.tar.gz`.
- Paket: Signaturblock, dann control (`control.tar.gz` mit `.PKGINFO`), dann
  data (`data.tar.gz` mit dem Inhalt, z. B. `usr/bin/werkzeug.exe`).

Unsere Prüfung (`apkcore/verify.go`) akzeptiert RSA mit PKCS1v15 über SHA-1 oder
SHA-256. Ein 2048-Bit-Schlüssel genügt.

Zusätzlich prüft der Client den `datahash` aus dem Index gegen den rohen
data-Block der Datei; beide Hashes sind sha256 in Hex.

## 4. Der Schlüssel beim Client

Der öffentliche Schlüssel liegt als `<keyname>.rsa.pub` in
`<root>/etc/apk/keys/` oder `/etc/apk/keys/`. Der Dateiname muss zum Namen im
Signaturblock passen. Fehlt er, gilt der Index als nicht vertrauenswürdig und
`apk` verlangt `--allow-untrusted`.

Im Bündel heißt das: `root\etc\apk\keys\<keyname>.rsa.pub`, beim Bau
mitgeliefert.

## 5. Vertrauen von Anfang an

Die Frage war: kann `apk add` ein Paket installieren, dem wir ausdrücklich
vertrauen - etwa die CA-Schlüssel - und danach unseren eigenen Paketen trauen?

Ja, und es gibt drei Wege. Der erste ist der saubere:

1. **Schlüssel mitliefern.** Der öffentliche Schlüssel liegt im Bündel unter
   `root\etc\apk\keys\`. Dann ist vom ersten Aufruf an alles verifiziert, ohne
   Ausnahme und ohne Sonderfall. Für ein portables Bündel, das wir selbst
   bauen, ist das der richtige Weg: wir wissen, welchen Schlüssel wir
   ausliefern.
2. **Einmal unsigniert bootstrappen.** Das erste Paket kommt mit
   `--allow-untrusted` herein und bringt die Schlüssel mit; ab dann prüft alles
   andere. Das ist Alpines Weg, wenn ein Repo neu ist. Nachteil: der eine
   unsignierte Schritt bleibt eine Lücke, und `--allow-untrusted` gilt pro
   Aufruf, nicht dauerhaft.
3. **Schlüssel als Datei installieren.** `apk add ./unser-key.rsa.pub` kopiert
   die Datei nach `<root>/etc/apk/keys/`. Das ist eine Erweiterung (Alpine kennt
   nur das Ablegen von Hand), klein zu bauen, und macht den Bootstrap ehrlich:
   man sieht genau, welcher Schlüssel hereinkommt. Ticket bei Bedarf.

Es gibt in `apk` kein "dieses eine Paket ist vertraut": das Vertrauen hängt am
Schlüssel, nicht am Paket. Deshalb ist Weg 1 die einzige Lösung ohne Lücke - und
bei einem Bündel, das wir selbst packen, auch die einfachste.

## 6. Das Bau-Skript

Ein Skript, das ein Werkzeug zu einem Paket und daraus ein Repo macht:

1. `.PKGINFO` schreiben: `pkgname`, `pkgver`, `arch`, `size` (Bytes des
   data-Blocks), `datahash` (sha256 über den rohen gzip-Block der data),
   `depend`, `provides`, `pkgdesc`.
2. `control.tar.gz` aus `.PKGINFO`, `data.tar.gz` aus dem Baum (die exe unter
   `usr/bin`).
3. Signatur über `control.tar.gz` + `data.tar.gz` rechnen und als
   `.SIGN.RSA.<keyname>.rsa.pub` davorsetzen; das Ganze ist die `.apk`.
4. Alle `.PKGINFO` einsammeln, `APKINDEX` schreiben, signieren, als
   `APKINDEX.tar.gz` ablegen.
5. `pkgver` hochziehen, sonst zieht `apk upgrade` nichts nach.

Der Schlüssel selbst entsteht einmalig: `openssl genrsa -out akmesh.rsa 2048`,
`openssl rsa -in akmesh.rsa -pubout -out akmesh.rsa.pub`. Der private bleibt
außerhalb des Repos, der öffentliche geht ins Bündel.

## 7. Offene Entscheidungen

- **Ort:** GitHub Pages (statisch, https, ohne Ratenlimit wie bei raw),
  eigener Ordner im Bündel (dann bleibt alles portabel, eine Zeile in
  `repositories` genügt) oder ein Platz auf testbild.tv. Portabel ist
  konsistent mit dem Bündel; ein Netz-Repo erlaubt Aktualisierungen ohne neues
  Bündel.
- **Arch:** `x86_64` geht ohne Zusatz, `windows-amd64` trennt sauber, verlangt
  aber bei jedem Aufruf `apk --arch windows-amd64`.

## 8. Was fehlt

- Erzeuger: Index, Signatur, Bau-Skript, Schlüssel (acmesh-c474).
- Mehrere Sources: `winget:msstore` wird gelesen, aber nur die erste Zeile
  benutzt (acmesh-mdd8).
- `apk upgrade --all` gegen echtes winget ist nicht gemessen.
