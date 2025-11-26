package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
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

// Estruturas para Estoque
type Produto struct {
	CodigoProduto    int    `json:"codigoProduto"`
	DescricaoProduto string `json:"descricaoProduto"`
	Estoque          int    `json:"estoque"`
}

type EstoqueData struct {
	Estoque []Produto `json:"estoque"`
}

type Movimentacao struct {
	ID               int       `json:"id"`
	CodigoProduto    int       `json:"codigoProduto"`
	DescricaoProduto string    `json:"descricaoProduto"`
	Tipo             string    `json:"tipo"`
	Quantidade       int       `json:"quantidade"`
	Descricao        string    `json:"descricao"`
	EstoqueAnterior  int       `json:"estoqueAnterior"`
	EstoqueFinal     int       `json:"estoqueFinal"`
	DataHora         time.Time `json:"dataHora"`
}

type MovimentacaoRequest struct {
	CodigoProduto int    `json:"codigoProduto"`
	Tipo          string `json:"tipo"`
	Quantidade    int    `json:"quantidade"`
	Descricao     string `json:"descricao"`
}

var (
	movimentacoes     []Movimentacao
	proximoID         = 1
	mutexMovimentacao sync.Mutex
)

func init() {
	carregarMovimentacoes()
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

func carregarMovimentacoes() {
	data, err := os.ReadFile("data/movimentacoes.json")
	if err != nil {
		movimentacoes = []Movimentacao{}
		return
	}

	json.Unmarshal(data, &movimentacoes)
	
	for _, m := range movimentacoes {
		if m.ID >= proximoID {
			proximoID = m.ID + 1
		}
	}
}

func salvarMovimentacoes() error {
	data, err := json.MarshalIndent(movimentacoes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("data/movimentacoes.json", data, 0644)
}

func carregarEstoque() (*EstoqueData, error) {
	data, err := os.ReadFile("data/estoque.json")
	if err != nil {
		return nil, err
	}

	var estoque EstoqueData
	err = json.Unmarshal(data, &estoque)
	if err != nil {
		return nil, err
	}

	return &estoque, nil
}

func salvarEstoque(estoque *EstoqueData) error {
	data, err := json.MarshalIndent(estoque, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("data/estoque.json", data, 0644)
}

func processarMovimentacao(req MovimentacaoRequest) (*Movimentacao, error) {
	mutexMovimentacao.Lock()
	defer mutexMovimentacao.Unlock()

	estoqueData, err := carregarEstoque()
	if err != nil {
		return nil, err
	}

	var produtoIndex = -1
	for i, p := range estoqueData.Estoque {
		if p.CodigoProduto == req.CodigoProduto {
			produtoIndex = i
			break
		}
	}

	if produtoIndex == -1 {
		return nil, http.ErrAbortHandler
	}

	produto := &estoqueData.Estoque[produtoIndex]
	estoqueAnterior := produto.Estoque

	// Calcular novo estoque
	novoEstoque := estoqueAnterior
	if req.Tipo == "ENTRADA" {
		novoEstoque += req.Quantidade
	} else if req.Tipo == "SAIDA" {
		novoEstoque -= req.Quantidade
		if novoEstoque < 0 {
			return nil, http.ErrAbortHandler
		}
	}

	produto.Estoque = novoEstoque
	err = salvarEstoque(estoqueData)
	if err != nil {
		return nil, err
	}

	mov := Movimentacao{
		ID:               proximoID,
		CodigoProduto:    produto.CodigoProduto,
		DescricaoProduto: produto.DescricaoProduto,
		Tipo:             req.Tipo,
		Quantidade:       req.Quantidade,
		Descricao:        req.Descricao,
		EstoqueAnterior:  estoqueAnterior,
		EstoqueFinal:     novoEstoque,
		DataHora:         time.Now(),
	}

	proximoID++
	movimentacoes = append(movimentacoes, mov)
	salvarMovimentacoes()

	return &mov, nil
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

	http.HandleFunc("/estoque", func(w http.ResponseWriter, r *http.Request) {
		render(w, "estoque.html", nil)
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

	http.HandleFunc("/api/estoque", func(w http.ResponseWriter, r *http.Request) {
		estoque, err := carregarEstoque()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estoque.Estoque)
	})

	http.HandleFunc("/api/movimentacoes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(movimentacoes)
			return
		}

		if r.Method == "POST" {
			var req MovimentacaoRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				http.Error(w, "Dados inválidos", http.StatusBadRequest)
				return
			}

			mov, err := processarMovimentacao(req)
			if err != nil {
				http.Error(w, "Erro ao processar movimentação", http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mov)
			return
		}

		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	})

	log.Println("Server started on http://localhost:5555")
	http.ListenAndServe(":5555", nil)
}