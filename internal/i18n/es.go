package i18n

var es = map[string]string{
	"usage": `librespeed-route traza la ruta desde este ordenador hasta un servidor, con la herramienta que el sistema ya tiene
(tracert en Windows, traceroute en macOS y Linux, tracepath cuando no hay traceroute).

Uso: librespeed-route [opciones] SERVIDOR

  -4, -6          traza por IPv4 o IPv6 (por defecto: la primera dirección pública del servidor)
  --json          escribe un objeto JSON en la salida estándar y ninguna tabla
  --quiet         sin tabla
  --timeout T     lo máximo que espera, como duración (por defecto 2m)
  --lang L        el idioma de los mensajes: en, pt, es o sv (por defecto: el del sistema)
  --version

La tabla va al error estándar y una línea de texto para la página ("LSR1!...") a la salida estándar, así que "librespeed-route SERVIDOR | clip"
copia solo la línea. Solo las direcciones públicas van en ella: las de tu red interna quedan fuera. Solo se trazan direcciones públicas.

Para la extensión del navegador:
  install-native-host [--all]   copia este programa a una carpeta de tu usuario y avisa a Chrome, Edge y Firefox (los que has usado;
                                --all: todos) de que la extensión puede iniciarlo. Sin derechos de administrador.
  uninstall-native-host         quita lo que escribió install-native-host, y solo eso
  doctor                        dice qué está dónde, y qué no funcionaría

Códigos de salida: 0 bien, 1 uso, 2 el servidor no se puede trazar, 3 ninguna herramienta de traceroute en este sistema, 4 falló, 130 interrumpido.
Las opciones van antes del servidor.
`,
	"err.prefix":           "librespeed-route: %[1]s\n",
	"err.host":             "librespeed-route: %[1]s: %[2]s\n",
	"err.interrupted":      "librespeed-route: interrumpido\n",
	"err.tool-failed":      "librespeed-route: %[1]s falló\n",
	"err.could-not-run":    "librespeed-route: no se pudo ejecutar %[1]s: %[2]v\n",
	"err.silent":           "librespeed-route: ningún router respondió (%[1]d saltos mudos): la red puede estar descartando los paquetes de %[2]s\n",
	"err.not-understood":   "librespeed-route: no se entendió la salida de %[1]s (%[2]s)",
	"err.stopped-suffix":   ", se detuvo: %[1]s",
	"msg.stopped":          "librespeed-route: %[1]s se detuvo (%[2]s); esto es lo que tenía hasta entonces\n",
	"reason.bad-host":      "no es un nombre ni una dirección que se pueda trazar",
	"reason.no-address":    "el nombre no tiene dirección",
	"reason.not-global":    "la dirección no es global (interna, de documentación, loopback...): solo se trazan direcciones públicas",
	"reason.no-tool":       "no se encontró tracert, traceroute ni tracepath en los lugares donde el sistema los guarda",
	"reason.lsr1-too-long": "la ruta no cabe en el texto que acepta la página",
	"reason.path":          "la carpeta del programa tiene un carácter (% \" ^ & !) que Windows leería como parte de un comando: usa otro usuario o una carpeta sin ellos",
	"table.title":          "Ruta a %[1]s (%[2]s, %[3]d saltos)\n",
	"table.times":          "%[1]s ms (mejor), %[2]s ms (media)",
	"table.no-answer":      "sin respuesta",
	"table.lost":           "%[1]d de %[2]d perdidos",
	"table.internal":       "interno",
	"table.cgnat":          "CGNAT",
	"table.also":           "también",
	"inst.not-used":        "%[1]s: no se usa en este ordenador (no se escribió nada; --all lo escribe igualmente)\n",
	"inst.program-in":      "El programa está en %[1]s\n",
	"inst.no-browser":      "No se encontró ningún navegador: abre Chrome, Edge o Firefox una vez y ejecuta esto de nuevo, o usa --all.\n",
	"inst.quarantine":      "Si el navegador no puede iniciarlo (un archivo descargado queda en cuarentena): xattr -d com.apple.quarantine %[1]q\n",
	"inst.no-exe":          "no se conoce la ruta del programa en ejecución",
	"inst.copying":         "copiando el programa: %[1]v",
	"inst.registry":        "registro %[1]s: %[2]v",
	"uninst.removed":       "eliminado %[1]s\n",
	"doctor.program":       "programa: %[1]s (%[2]s)\n",
	"doctor.there":         "está",
	"doctor.not-there":     "no está: ejecuta install-native-host",
	"doctor.no-manifest":   "sin manifiesto",
	"doctor.points-at":     "apunta a %[1]s, que no está",
	"doctor.ok":            "ok -> %[1]s",
	"doctor.container":     "aviso: %[1]s: un navegador en snap o flatpak no puede iniciar un programa del usuario; no es compatible\n",
	"doctor.quarantine":    "macOS: si el navegador no puede iniciar el programa, quita la cuarentena: xattr -d com.apple.quarantine %[1]q\n",
	"doctor.problems":      "%[1]d problema(s)",
	"win.menu": `librespeed-route

Este programa conecta la extensión de navegador "Ruta de vuelta para páginas de LibreSpeed" (Chrome, Edge, Firefox) con este ordenador: cuando
pulsas la extensión en la página de un servidor, ejecuta el tracert de Windows y entrega la ruta a la página, sin las direcciones de tu propia red.

  [I] Instalar para este usuario (se copia a %LOCALAPPDATA%\librespeed-route y se registra para los navegadores; sin administrador)
  [U] Desinstalar (quita solo lo que escribió la instalación)
  [Q] Salir

Para trazar una ruta desde un terminal:  librespeed-route.exe SERVIDOR   (o --help)

Elige [I/U/Q]: `,
	"win.failed":    "\nNo funcionó: %[1]v\n",
	"win.installed": "\nListo. Ahora añade la extensión al navegador y pulsa su icono en la página de un servidor.\n",
	"win.done":      "\nListo.\n",
	"win.nothing":   "\nNo se cambió nada.\n",
	"win.press":     "\nPulsa Intro para cerrar. ",
}
