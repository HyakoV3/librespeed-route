package i18n

var sv = map[string]string{
	"usage": `librespeed-route spårar vägen från den här datorn till en värd, med det verktyg som systemet redan har
(tracert i Windows, traceroute i macOS och Linux, tracepath när traceroute saknas).

Användning: librespeed-route [flaggor] VÄRD

  -4, -6          spåra över IPv4 eller IPv6 (standard: värdens första offentliga adress)
  --json          skriv ett JSON-objekt till standard ut och ingen tabell
  --quiet         ingen tabell
  --timeout T     längsta väntetid, som varaktighet (standard 2m)
  --lang L        språket för meddelandena: en, pt, es eller sv (standard: systemets)
  --version

Tabellen går till standard fel och en textrad för sidan ("LSR1!...") till standard ut, så "librespeed-route VÄRD | clip" kopierar bara
raden. Bara offentliga adresser skrivs i den: de i ditt interna nätverk lämnas utanför. Bara offentliga adresser spåras.

För webbläsartillägget:
  install-native-host [--all]   kopierar det här programmet till en mapp för din användare och talar om för Chrome, Edge och Firefox (de du har
                                använt; --all: alla) att tillägget får starta det. Inga administratörsrättigheter.
  uninstall-native-host         tar bort det som install-native-host skrev, och bara det
  doctor                        säger vad som finns var, och vad som inte skulle fungera

Slutkoder: 0 ok, 1 användning, 2 värden kan inte spåras, 3 inget traceroute-verktyg i det här systemet, 4 misslyckades, 130 avbruten.
Flaggorna står före värden.
`,
	"err.prefix":           "librespeed-route: %[1]s\n",
	"err.host":             "librespeed-route: %[1]s: %[2]s\n",
	"err.interrupted":      "librespeed-route: avbruten\n",
	"err.tool-failed":      "librespeed-route: %[1]s misslyckades\n",
	"err.could-not-run":    "librespeed-route: kunde inte köra %[1]s: %[2]v\n",
	"err.silent":           "librespeed-route: ingen router svarade (%[1]d tysta hopp): nätverket kan släppa paketen från %[2]s\n",
	"err.not-understood":   "librespeed-route: utskriften från %[1]s förstods inte (%[2]s)",
	"err.stopped-suffix":   ", den stoppades: %[1]s",
	"msg.stopped":          "librespeed-route: %[1]s stoppades (%[2]s); det här är vad den hade fått fram till dess\n",
	"reason.bad-host":      "inte ett värdnamn eller en adress som kan spåras",
	"reason.no-address":    "namnet har ingen adress",
	"reason.not-global":    "adressen är inte global (intern, för dokumentation, loopback...): bara offentliga adresser spåras",
	"reason.no-tool":       "varken tracert, traceroute eller tracepath hittades på de ställen där systemet har dem",
	"reason.lsr1-too-long": "vägen ryms inte i den text som sidan tar emot",
	"reason.path":          "programmets mapp har ett tecken (% \" ^ & !) som Windows skulle läsa som en del av ett kommando: använd en annan användare eller en mapp utan dem",
	"table.title":          "Väg till %[1]s (%[2]s, %[3]d hopp)\n",
	"table.times":          "%[1]s ms (bäst), %[2]s ms (medel)",
	"table.no-answer":      "inget svar",
	"table.lost":           "%[1]d av %[2]d förlorade",
	"table.internal":       "intern",
	"table.cgnat":          "CGNAT",
	"table.also":           "även",
	"inst.not-used":        "%[1]s: används inte på den här datorn (inget skrevs; --all skriver det ändå)\n",
	"inst.program-in":      "Programmet finns i %[1]s\n",
	"inst.no-browser":      "Ingen webbläsare hittades: öppna Chrome, Edge eller Firefox en gång och kör det här igen, eller använd --all.\n",
	"inst.quarantine":      "Om webbläsaren inte kan starta det (en nedladdad fil sätts i karantän): xattr -d com.apple.quarantine %[1]q\n",
	"inst.no-exe":          "sökvägen till det program som körs är inte känd",
	"inst.copying":         "kopierar programmet: %[1]v",
	"inst.registry":        "registret %[1]s: %[2]v",
	"uninst.removed":       "borttaget %[1]s\n",
	"doctor.program":       "program: %[1]s (%[2]s)\n",
	"doctor.there":         "finns",
	"doctor.not-there":     "finns inte: kör install-native-host",
	"doctor.no-manifest":   "inget manifest",
	"doctor.points-at":     "pekar på %[1]s, som inte finns",
	"doctor.ok":            "ok -> %[1]s",
	"doctor.container":     "varning: %[1]s: en webbläsare i snap eller flatpak kan inte starta ett program för användaren; det stöds inte\n",
	"doctor.quarantine":    "macOS: om webbläsaren inte kan starta programmet, ta bort karantänen: xattr -d com.apple.quarantine %[1]q\n",
	"doctor.problems":      "%[1]d problem",
	"win.menu": `librespeed-route

Det här programmet kopplar webbläsartillägget "Väg tillbaka för LibreSpeed-sidor" (Chrome, Edge, Firefox) till den här datorn: när du klickar
på tillägget på en servers sida kör det Windows tracert och lämnar vägen till sidan, utan adresserna i ditt eget nätverk.

  [I] Installera för den här användaren (det kopieras till %LOCALAPPDATA%\librespeed-route och registreras för webbläsarna; ingen administratör)
  [U] Avinstallera (tar bara bort det som installationen skrev)
  [Q] Avsluta

För att spåra en väg från en terminal:  librespeed-route.exe VÄRD   (eller --help)

Välj [I/U/Q]: `,
	"win.failed":    "\nDet fungerade inte: %[1]v\n",
	"win.installed": "\nKlart. Lägg nu till tillägget i webbläsaren och klicka på dess ikon på en servers sida.\n",
	"win.done":      "\nKlart.\n",
	"win.nothing":   "\nInget ändrades.\n",
	"win.press":     "\nTryck på Enter för att stänga. ",
}
