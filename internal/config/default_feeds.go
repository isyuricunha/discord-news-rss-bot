package config

var domainMapping = map[string]string{
	"g1.globo.com":               "G1",
	"rss.uol.com.br":             "UOL",
	"rss.bs.vibra.digital":       "Band",
	"band.com.br":                "Band",
	"cnnbrasil.com.br":           "CNN Brasil",
	"feeds.folha.uol.com.br":     "Folha",
	"gazetadopovo.com.br":        "Gazeta do Povo",
	"jovempan.com.br":            "Jovem Pan",
	"diariodopoder.com.br":       "Diario do Poder",
	"pragmatismopolitico.com.br": "Pragmatismo Politico",
	"conexaopolitica.com.br":     "Conexao Politica",
	"poder360.com.br":            "Poder 360",
	"crusoe.uol.com.br":          "Crusoe",
	"crusoe.com.br":              "Crusoe",
	"veja.abril.com.br":          "Veja",
	"metropoles.com":             "Metropoles",
	"oantagonista.com":           "O Antagonista",
	"canaltech.com.br":           "Canaltech",
	"olhardigital.com.br":        "Olhar Digital",
	"tecnoblog.net":              "Tecnoblog",
	"meiobit.com":                "Meio Bit",
	"showmetech.com.br":          "ShowMeTech",
	"rss.tecmundo.com.br":        "TecMundo",
	"tecmundo.com.br":            "TecMundo",
	"adrenaline.com.br":          "Adrenaline",
	"hardware.com.br":            "Hardware.com.br",
	"tudocelular.com":            "Tudo Celular",
	"oficinadanet.com.br":        "Oficina da Net",
}

func defaultFeedGroups() []defaultFeedGroup {
	return []defaultFeedGroup{
		{
			category: "📰 General News",
			urls: []string{
				"https://g1.globo.com/dynamo/rss2.xml",
				"https://rss.uol.com.br/feed/noticias.xml",
				"https://rss.bs.vibra.digital/feed.xml?site=portal&size=10",
				"https://www.cnnbrasil.com.br/rss/",
				"https://feeds.folha.uol.com.br/emcimadahora/rss091.xml",
			},
		},
		{
			category: "🏛️ Politics & Conservative",
			urls: []string{
				"https://www.gazetadopovo.com.br/feed/rss/republica.xml",
				"https://jovempan.com.br/feed/",
				"https://www.diariodopoder.com.br/feed/",
				"https://www.pragmatismopolitico.com.br/feed/",
				"https://conexaopolitica.com.br/feed/",
				"https://www.poder360.com.br/feed/",
				"https://crusoe.uol.com.br/rss/",
				"https://veja.abril.com.br/rss/",
				"https://www.metropoles.com/feed",
				"https://oantagonista.com.br/feed/",
			},
		},
		{
			category: "💻 Technology",
			urls: []string{
				"https://canaltech.com.br/rss/",
				"https://olhardigital.com.br/feed/",
				"https://tecnoblog.net/feed/",
				"https://meiobit.com/feed/",
				"https://www.showmetech.com.br/feed/",
				"https://rss.tecmundo.com.br/feed",
				"https://www.adrenaline.com.br/rss/",
				"https://www.hardware.com.br/rss/",
				"https://www.tudocelular.com/rss/",
				"https://www.oficinadanet.com.br/rss/geral",
			},
		},
	}
}
