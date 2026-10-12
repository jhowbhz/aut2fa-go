# Changelog

## 0.1.0 (2026-10-11)

- Cliente Go para enviar, consultar o status e verificar códigos 2FA.
- Base URL configurável, timeout positivo e cliente HTTP injetável.
- Chave de idempotência no header, com validação de tamanho e caracteres.
- Respostas tipadas com timestamps e preservação de campos nullable.
- Erros HTTP estruturados com status real, campos de validação e `Retry-After`.
- Redirects e retries automáticos desativados.
- Testes de contrato HTTP local, exemplo compilável e CI para Go 1.22 e estável.
