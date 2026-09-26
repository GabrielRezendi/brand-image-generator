# Brand Image Generator

TUI em Go + Bubble Tea para geração de artes de marca.

## Requisitos

- Go 1.22+
- Terminal com suporte ANSI
- Chave OpenRouter e/ou OpenAI

## Run

```bash
go run .
# ou
go build -o brand-image-generator .
./brand-image-generator
```

## Fluxo

1. **Setup** (primeira vez): chaves, provider padrão, modelos, guia da marca, assets
2. **Novo pedido**: prompt + anexos opcionais (caminho png/jpg/svg)
3. **Geração**, com progresso por etapa:
   1. Gerando SVG (só estrutura/conteúdo — sem ícones/artes)
   2. Preparando fontes
   3. Gerando fundo (`elements/background.png`)
   4. Montando entrega autocontida (`elements/` + hrefs relativos)
   5. Convertendo SVG → PNG (`final.png` via `rsvg-convert`)
   6. *(opcional)* Revisando arte (vision no PNG final) — **desligada por defeito**

Com a revisão ligada (Definições → Revisão automática), há até 3 tentativas; se a 3.ª falhar, entrega todas e para.

Dependência de sistema: `librsvg2-bin` (`rsvg-convert`).

Cancelamento: `esc` / `ctrl+x` durante a geração.

## Configuração

`~/.config/brand-image-generator/config.yaml`

- Chaves (mascaradas; aceita `${NOME_DA_ENV}`)
- `default_provider`: `openrouter` | `openai`
- Modelos SVG / imagem / geral
- Guia da marca e assets
- Identidade visual: fontes primária/secundária (ficheiro `.ttf`/`.otf`, nome Google Fonts ou URL) e cores hex
- `enable_review`: revisão vision automática (padrão `false`)

Defaults: `openai/gpt-6-astra` (SVG + geral), `openai/gpt-image-2.5-flare` (imagem).

Em **Definições** podes mudar provider, modelos, guia, identidade visual, assets e revisão automática. No pedido, `Ctrl+P` alterna o provider da sessão.

## Saídas

```
outputs/{DD}/{MM}/{YYYY}/{resumo-do-pedido}/
  prompt.md
  meta.json
  conversation.log
  final.svg / final.png / elements/   # se aprovado
  attempt-1/
    structure.svg
    image-prompt.txt
    elements/            # background + outros src (autocontido)
    fonts/               # tipografia embutida no SVG/PNG
    final.svg
    final.png
    review.json
    usage.json
  attempt-2/
  attempt-3/
```

A pasta `outputs/` é relativa ao diretório de onde corres o binário (configurável em `output_dir` no YAML).
