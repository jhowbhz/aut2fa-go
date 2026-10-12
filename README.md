# aut2fa-go

Biblioteca cliente (SDK) de Go para enviar, consultar e verificar códigos 2FA pela API aut2fa.

Requer Go 1.22 ou superior. Usa apenas a biblioteca padrão do Go. Licença MIT.

## Instalação

Instale a versão 0.1.0 no seu módulo:

```sh
go get github.com/jhowbhz/aut2fa-go@v0.1.0
```

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

Respostas 2xx inválidas são identificadas por `ErrInvalidResponse`. Erros de rede permanecem identificáveis com `errors.As`/`errors.Is`, incluindo `net.Error`, `context.Canceled` e `context.DeadlineExceeded`.

## Desenvolvimento

Valide a biblioteca e o exemplo sem acessar a API real:

```sh
gofmt -w .
go vet ./...
go test -race ./...
go build ./...
```

A CI valida formatação, vet, testes com race detector e build no Linux, com Go 1.22 e a versão estável. Não há workflow de publicação automática. A versão Go é distribuída pela tag Git `v0.1.0` após revisão.

Consulte a [documentação da API aut2fa](https://doc.aut2fa.com) para o contrato HTTP.
