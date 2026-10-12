package i18n

var pt = map[string]string{
	"usage": `O librespeed-route traça a rota deste computador até um servidor, com a ferramenta que o sistema já tem
(tracert no Windows, traceroute no macOS e no Linux, tracepath quando não há traceroute).

Uso: librespeed-route [opções] SERVIDOR

  -4, -6          traça por IPv4 ou IPv6 (padrão: o primeiro endereço público do servidor)
  --json          escreve um objeto JSON na saída padrão e nenhuma tabela
  --quiet         sem tabela
  --timeout T     o máximo a esperar, como duração (padrão 2m)
  --lang L        o idioma das mensagens: en, pt, es ou sv (padrão: o do sistema)
  --version

A tabela vai para o erro padrão e uma linha de texto para a página ("LSR1!...") para a saída padrão, então "librespeed-route SERVIDOR | clip"
copia só a linha. Só endereços públicos vão nela: os da sua rede interna ficam de fora. Só endereços públicos são traçados.

Para a extensão do navegador:
  install-native-host [--all]   copia este programa para uma pasta do seu usuário e avisa o Chrome, o Edge e o Firefox (os que você já usou;
                                --all: todos) de que a extensão pode iniciá-lo. Sem direitos de administrador.
  uninstall-native-host         remove o que o install-native-host escreveu, e só isso
  doctor                        diz o que está onde, e o que não funcionaria

Códigos de saída: 0 ok, 1 uso, 2 o servidor não pode ser traçado, 3 nenhuma ferramenta de traceroute neste sistema, 4 falhou, 130 interrompido.
As opções vão antes do servidor.
`,
	"err.prefix":           "librespeed-route: %[1]s\n",
	"err.host":             "librespeed-route: %[1]s: %[2]s\n",
	"err.interrupted":      "librespeed-route: interrompido\n",
	"err.tool-failed":      "librespeed-route: o %[1]s falhou\n",
	"err.could-not-run":    "librespeed-route: não foi possível executar o %[1]s: %[2]v\n",
	"err.silent":           "librespeed-route: nenhum roteador respondeu (%[1]d saltos mudos): a rede pode estar descartando os pacotes do %[2]s\n",
	"err.not-understood":   "librespeed-route: a saída do %[1]s não foi entendida (%[2]s)",
	"err.stopped-suffix":   ", ele foi interrompido: %[1]s",
	"msg.stopped":          "librespeed-route: o %[1]s foi interrompido (%[2]s); isto é o que ele tinha até ali\n",
	"reason.bad-host":      "não é um nome nem um endereço que possa ser traçado",
	"reason.no-address":    "o nome não tem endereço",
	"reason.not-global":    "o endereço não é global (interno, de documentação, loopback...): só endereços públicos são traçados",
	"reason.no-tool":       "não foi encontrado tracert, traceroute nem tracepath nos lugares onde o sistema os guarda",
	"reason.lsr1-too-long": "a rota não cabe no texto que a página aceita",
	"reason.path":          "a pasta do programa tem um caractere (% \" ^ & !) que o Windows leria como parte de um comando: use outro usuário ou uma pasta sem eles",
	"table.title":          "Rota até %[1]s (%[2]s, %[3]d saltos)\n",
	"table.times":          "%[1]s ms (melhor), %[2]s ms (média)",
	"table.no-answer":      "sem resposta",
	"table.lost":           "%[1]d de %[2]d perdidos",
	"table.internal":       "interno",
	"table.cgnat":          "CGNAT",
	"table.also":           "também",
	"inst.not-used":        "%[1]s: não usado neste computador (nada foi escrito; --all escreve mesmo assim)\n",
	"inst.program-in":      "O programa está em %[1]s\n",
	"inst.no-browser":      "Nenhum navegador foi encontrado: abra o Chrome, o Edge ou o Firefox uma vez e rode isto de novo, ou use --all.\n",
	"inst.quarantine":      "Se o navegador não conseguir iniciá-lo (um arquivo baixado fica em quarentena): xattr -d com.apple.quarantine %[1]q\n",
	"inst.no-exe":          "o caminho do programa em execução não é conhecido",
	"inst.copying":         "copiando o programa: %[1]v",
	"inst.registry":        "registro %[1]s: %[2]v",
	"uninst.removed":       "removido %[1]s\n",
	"doctor.program":       "programa: %[1]s (%[2]s)\n",
	"doctor.there":         "está lá",
	"doctor.not-there":     "não está lá: rode install-native-host",
	"doctor.no-manifest":   "sem manifesto",
	"doctor.points-at":     "aponta para %[1]s, que não está lá",
	"doctor.ok":            "ok -> %[1]s",
	"doctor.container":     "aviso: %[1]s: um navegador em snap ou flatpak não consegue iniciar um programa do usuário; não é suportado\n",
	"doctor.quarantine":    "macOS: se o navegador não conseguir iniciar o programa, limpe a quarentena: xattr -d com.apple.quarantine %[1]q\n",
	"doctor.problems":      "%[1]d problema(s)",
	"win.menu": `librespeed-route

Este programa liga a extensão de navegador "Rota de volta para páginas do LibreSpeed" (Chrome, Edge, Firefox) a este computador: quando você
clica na extensão na página de um servidor, ele roda o tracert do Windows e entrega a rota à página, sem os endereços da sua própria rede.

  [I] Instalar para este usuário (ele é copiado para %LOCALAPPDATA%\librespeed-route e registrado para os navegadores; sem administrador)
  [U] Desinstalar (remove só o que a instalação escreveu)
  [Q] Sair

Para traçar uma rota em um terminal:  librespeed-route.exe SERVIDOR   (ou --help)

Escolha [I/U/Q]: `,
	"win.failed":    "\nNão funcionou: %[1]v\n",
	"win.installed": "\nPronto. Agora adicione a extensão ao navegador e clique no ícone dela na página de um servidor.\n",
	"win.done":      "\nPronto.\n",
	"win.nothing":   "\nNada foi alterado.\n",
	"win.press":     "\nPressione Enter para fechar. ",
}
