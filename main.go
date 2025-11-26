package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
)

var tl = template.Must(template.ParseGlob("views/*.html"))

type Venda struct {
	Vendedor string  `json:"vendedor"`
	Valor    float64 `json:"valor"`
}

type VendasData struct {
	Vendas []Venda `json:"vendas"`
}

type ComissaoVendedor struct {
	Vendedor       string  `json:"vendedor"`
	TotalVendas    float64 `json:"totalVendas"`
	TotalComissao  float64 `json:"totalComissao"`
	QtdVendas      int     `json:"qtdVendas"`
	VendasDetalhes []VendaDetalhe `json:"vendasDetalhes"`
}

type VendaDetalhe struct {
	Valor         float64 `json:"valor"`
	Comissao      float64 `json:"comissao"`
	PercentualStr string  `json:"percentualStr"`
}

func calcularComissao(valor float64) float64 {
	if valor < 100.00 {
		return 0
	} else if valor < 500.00 {
		return valor * 0.01
	} else {
		return valor * 0.05
	}
}

func obterPercentualStr(valor float64) string {
	if valor < 100.00 {
		return "0%"
	} else if valor < 500.00 {
		return "1%"
	} else {
		return "5%"
	}
}

func processarComissoes() ([]ComissaoVendedor, error) {
	data, err := os.ReadFile("data/vendas.json")
	if err != nil {
		return nil, err
	}

	var vendasData VendasData
	err = json.Unmarshal(data, &vendasData)
	if err != nil {
		return nil, err
	}

	vendedoresMap := make(map[string]*ComissaoVendedor)

	for _, venda := range vendasData.Vendas {
		comissao := calcularComissao(venda.Valor)
		
		if vendedoresMap[venda.Vendedor] == nil {
			vendedoresMap[venda.Vendedor] = &ComissaoVendedor{
				Vendedor:       venda.Vendedor,
				VendasDetalhes: []VendaDetalhe{},
			}
		}

		vendedoresMap[venda.Vendedor].TotalVendas += venda.Valor
		vendedoresMap[venda.Vendedor].TotalComissao += comissao
		vendedoresMap[venda.Vendedor].QtdVendas++
		vendedoresMap[venda.Vendedor].VendasDetalhes = append(
			vendedoresMap[venda.Vendedor].VendasDetalhes,
			VendaDetalhe{
				Valor:         venda.Valor,
				Comissao:      comissao,
				PercentualStr: obterPercentualStr(venda.Valor),
			},
		)
	}

	resultado := make([]ComissaoVendedor, 0, len(vendedoresMap))
	for _, vendedor := range vendedoresMap {
		resultado = append(resultado, *vendedor)
	}

	sort.Slice(resultado, func(i, j int) bool {
		return resultado[i].Vendedor < resultado[j].Vendedor
	})

	return resultado, nil
}

func render(w http.ResponseWriter, file string, data any) {
	err := tl.ExecuteTemplate(w, file, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		render(w, "index.html", nil)
	})

	http.HandleFunc("/comissoes", func(w http.ResponseWriter, r *http.Request) {
		render(w, "comissoes.html", nil)
	})

	http.HandleFunc("/api/comissoes", func(w http.ResponseWriter, r *http.Request) {
		comissoes, err := processarComissoes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comissoes)
	})

	log.Println("Server started on http://localhost:5555")
	http.ListenAndServe(":5555", nil)
}