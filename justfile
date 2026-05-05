# This is for me.
import? 'dev.just'

build TYPE="cli": 
  mkdir -p bin
  go build -o bin/{{ if TYPE == "cli" { "typos" } else if TYPE == "server" { "typos-server" } else { "typos" } }} ./cmd/{{ if TYPE == "cli" { "typos" } else if TYPE == "server" { "typos-server" } else { "typos" } }}

fmt:
  gofmt -w -s .
