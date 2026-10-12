<p align="center">
  <a href="https://aut2fa.com">
    <img src="https://raw.githubusercontent.com/jhowbhz/aut2fa-go/main/.github/assets/banner.png" alt="aut2fa para Go: você envia o código, a gente entrega por WhatsApp, SMS, Telegram e e-mail" width="100%">
  </a>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/jhowbhz/aut2fa-go"><img src="https://pkg.go.dev/badge/github.com/jhowbhz/aut2fa-go.svg" alt="Go Reference"></a>
  <a href="https://github.com/jhowbhz/aut2fa-go/releases"><img src="https://img.shields.io/github/v/tag/jhowbhz/aut2fa-go?color=00cfc5&label=vers%C3%A3o" alt="Versão"></a>
  <a href="https://github.com/jhowbhz/aut2fa-go/actions/workflows/ci.yml"><img src="https://github.com/jhowbhz/aut2fa-go/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/jhowbhz/aut2fa-go?color=00add8&logo=go&logoColor=white" alt="Versão do Go">
  <img src="https://img.shields.io/badge/dependências-0-00cfc5" alt="Zero dependências">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/jhowbhz/aut2fa-go?color=00cfc5" alt="Licença MIT"></a>
</p>

<p align="center">
  <b>Envie códigos 2FA por WhatsApp, SMS, Telegram e e-mail com uma chamada de API.</b><br>
  <a href="https://aut2fa.com">Site</a> ·
  <a href="https://doc.aut2fa.com">Documentação</a> ·
  <a href="https://pkg.go.dev/github.com/jhowbhz/aut2fa-go">Referência Go</a> ·
  <a href="https://aut2fa.com">Criar conta grátis</a> ·
  <a href="https://github.com/jhowbhz/aut2fa-go/issues">Reportar problema</a>
</p>

---

## Por que usar

- **Um endpoint, quatro canais.** Você chama `Send`, e a aut2fa entrega por WhatsApp, SMS, Telegram ou e-mail.
- **Sem gerar nem guardar código.** A API pode gerar o código; você só chama `Verify` com o que o usuário digitou.
- **Zero dependências.** Só biblioteca padrão. Seu `go.sum` continua limpo.
- **Go idiomático.** Todo método recebe `context.Context`, os erros funcionam com `errors.As` e `errors.Is`, e o cliente pode ser compartilhado entre goroutines.
- **Seguro por padrão.** Não segue redirects, não faz retries escondidos e não expõe corpos de erro arbitrários.
- **Tipos de verdade.** Campos nullable viram ponteiros e timestamps chegam como `time.Time`.

## Instalar

```sh
go get github.com/jhowbhz/aut2fa-go
```

## Começar em 30 segundos

```go
client, err := aut2fa.New(os.Getenv("AUT2FA_API_KEY"))
if err != nil {
	log.Fatal(err)
}
ctx := context.Background()

// 1. Envia o código para o usuário
send, err := client.Send(ctx, aut2fa.SendParams{To: "+5511999999999"})
if err != nil {
	log.Fatal(err)
}

// 2. Confere o código que o usuário digitou
result, err := client.Verify(ctx, send.ID, "123456")
if err != nil {
	log.Fatal(err)
}
if result.Valid {
	fmt.Println("Login liberado")
}
```

Crie sua conta em [aut2fa.com](https://aut2fa.com) para pegar a chave da aplicação.

## Enviar, consultar e verificar

Crie o cliente com a chave da aplicação e use o ID retornado para consultar ou verificar o envio:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jhowbhz/aut2fa-go"
)

func main() {
	client, err := aut2fa.New(os.Getenv("AUT2FA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	send, err := client.Send(ctx, aut2fa.SendParams{
		To: "+5511999999999",
		Code: "123456",
		Locale: "pt-BR",
		IdempotencyKey: "cadastro-user-123",
	})
	if err != nil {
		log.Fatal(err)
	}

	status, err := client.Status(ctx, send.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(send.ID, status.Status)

	verification, err := client.Verify(ctx, send.ID, "123456")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(verification.Valid, verification.AttemptsLeft)
}
```

`To` é obrigatório. `Code` e `Locale` são opcionais no envio; deixe `Code` vazio para a API gerar o código. A chave de idempotência é opcional, fica somente no header `Idempotency-Key` e aceita até 255 bytes sem caracteres de controle. Use uma chave por operação lógica e preserve a mesma chave e o mesmo corpo ao repetir essa operação.

`Send` e `Status` retornam `*aut2fa.SendResponse`. `Verify` retorna `*aut2fa.VerifyResponse`. Um resultado com `Valid == false` e HTTP 200 é uma tentativa de verificação normal.

Campos nullable são ponteiros (`*string` ou `*time.Time`); um `null` da API vira `nil`. O SDK decodifica timestamps como `time.Time` e aceita campos adicionais do JSON.

O [exemplo completo](examples/basic/main.go) usa `AUT2FA_API_KEY`, `AUT2FA_TO` e, opcionalmente, `AUT2FA_IDEMPOTENCY_KEY`. Rode `go run ./examples/basic` com essas variáveis definidas para fazer chamadas reais. Os testes usam servidores locais e não enviam mensagens.

## Configuração

Substitua a construção do cliente abaixo e acrescente os imports `net/http` e `time` ao seu programa:

```go
client, err := aut2fa.New(os.Getenv("AUT2FA_API_KEY"),
	aut2fa.WithBaseURL("http://localhost:8080"),
	aut2fa.WithTimeout(5*time.Second),
	aut2fa.WithHTTPClient(&http.Client{}),
)
```

A base padrão é `https://api.aut2fa.com`, com timeout total de 10 segundos por chamada. `WithTimeout` exige duração positiva. A URL precisa ser HTTP ou HTTPS, com host e sem credenciais, query ou fragmento; também pode conter um prefixo de caminho. IDs aceitam apenas letras ASCII, números, `_` e `-`. Credenciais vazias ou com caracteres de controle são rejeitadas ao construir o cliente.

`WithHTTPClient` copia o cliente fornecido e preserva seu transporte e cookie jar. O timeout do SDK e a proibição de redirects prevalecem sobre os valores do cliente fornecido, sem modificá-lo. O SDK clona transportes padrão e desativa keep-alive para impedir retries transparentes em conexões reutilizadas. Um transporte personalizado deve implementar `http.RoundTripper` sem retries e ser seguro para uso concorrente.

O SDK não segue redirects e não faz retries automáticos. Cada método aceita `context.Context`; cancelamentos e deadlines do chamador são preservados. O cliente pode ser compartilhado entre goroutines.

## Erros

Após uma chamada, importe `errors` e identifique a rejeição HTTP ou uma resposta inválida:

```go
var problem *aut2fa.ApiError
if errors.As(err, &problem) {
	fmt.Println(problem.Status, problem.Code, problem.Detail)
	fmt.Println(problem.Errors, problem.RetryAfter)
}
if errors.Is(err, aut2fa.ErrInvalidResponse) {
	fmt.Println("A API retornou uma resposta inválida.")
}
```

Respostas HTTP fora de 2xx geram `*ApiError` com o status HTTP real, `Code`, `Detail`, `Errors` e o `RetryAfter` bruto. O header pode conter segundos ou uma data HTTP. Corpos de erro inesperados usam `http_error`; o SDK não expõe o corpo arbitrário.

Respostas 2xx inválidas são identificadas por `ErrInvalidResponse`. Identifique timeouts do cliente com `errors.As` para `net.Error` e seu método `Timeout()`. Cancelamentos e deadlines do contexto do chamador preservam `context.Canceled` e `context.DeadlineExceeded`, verificáveis com `errors.Is`.

## Desenvolvimento

Valide a biblioteca e o exemplo sem acessar a API real:

```sh
gofmt -w .
go vet ./...
go test -race ./...
go build ./...
```

A CI valida formatação, vet, testes com race detector e build no Linux, com Go 1.22 e a versão estável.

## Publicar novas versões

Módulos Go são distribuídos pelas tags Git. Para lançar uma versão, crie e envie uma tag semântica, como `v0.1.1`. O [proxy do Go](https://proxy.golang.org) e o [pkg.go.dev](https://pkg.go.dev/github.com/jhowbhz/aut2fa-go) indexam a nova versão na primeira vez que alguém roda `go get` com ela.

## Outras linguagens

| Linguagem | Pacote | Repositório |
| --- | --- | --- |
| Go | [`github.com/jhowbhz/aut2fa-go`](https://pkg.go.dev/github.com/jhowbhz/aut2fa-go) | [aut2fa-go](https://github.com/jhowbhz/aut2fa-go) |
| Node.js | [`@aut2fa/node`](https://www.npmjs.com/package/@aut2fa/node) | [aut2fa-node](https://github.com/jhowbhz/aut2fa-node) |
| Python | [`aut2fa`](https://pypi.org/project/aut2fa/) | [aut2fa-python](https://github.com/jhowbhz/aut2fa-python) |
| PHP | [`aut2fa/aut2fa-php`](https://packagist.org/packages/aut2fa/aut2fa-php) | [aut2fa-php](https://github.com/jhowbhz/aut2fa-php) |

## Contribuir

Encontrou um bug ou tem uma ideia? Abra uma [issue](https://github.com/jhowbhz/aut2fa-go/issues) ou envie um pull request. Consulte o [histórico de alterações](CHANGELOG.md) para acompanhar as versões.

Se este SDK te ajudou, deixe uma ⭐ no repositório. Isso ajuda outros devs a encontrá-lo.

---

<p align="center">
  <a href="https://doc.aut2fa.com">Documentação da API</a> · Licença <a href="LICENSE">MIT</a> · Feito pela <a href="https://aut2fa.com">aut2fa</a>
</p>
