# Tera for Go

Tera is a template engine inspired by [Jinja2](https://jinja.palletsprojects.com/) and the [Django template language](https://docs.djangoproject.com/en/6.0/topics/templates/). This repository provides the Go port of Tera 2.4.0 and tera-contrib 0.3.0.

```jinja2
<title>{% block title %}{% endblock title %}</title>
<ul>
{% for user in users %}
  <li><a href="{{ user.url }}">{{ user.username }}</a></li>
{% endfor %}
</ul>
```

The syntax intentionally differs from Jinja2 and Django in several places. See the [template language documentation](docs/content/_index.md).

## Install

Go 1.23 or later is required. The module uses the Go standard library and has no external Go dependencies.

```sh
go get github.com/Keats/tera/tera
```

To install the command:

```sh
go install github.com/Keats/tera/cmd/tera@latest
```

## Use

```go
package main

import (
    "log"
    "os"

    "github.com/Keats/tera/tera"
)

func main() {
    engine := tera.New()
    if err := engine.AddRawTemplate("hello.html", "<h1>Hello {{ name }}!</h1>"); err != nil {
        log.Fatal(err)
    }

    context := tera.NewContext()
    context.Insert("name", "Bob & Alice")
    if err := engine.RenderTo("hello.html", context, os.Stdout); err != nil {
        log.Fatal(err)
    }
}
```

This writes `<h1>Hello Bob &amp; Alice!</h1>`. Templates ending in `.html`, `.htm`, and `.xml` escape values by default.

Load templates from disk with `engine.LoadFromGlob("templates/**/*.html")`, or add several templates together with `engine.AddRawTemplates([]tera.RawTemplate{...})`. Register custom filters, tests, and functions before loading templates that use them. `tera.OneOff(source, context, autoescape)` renders a standalone template; `engine.RenderStr` also has access to the engine's registered templates and extensions.

The [contrib package](tera-contrib/README.md) adds dates, JSON, base64, formatting, regular expressions, URL encoding, slugs, and seeded random values:

```go
import contrib "github.com/Keats/tera/tera-contrib"

// Register all contrib extensions before adding templates.
contrib.Register(engine)
// Or register only the extensions you need.
engine.RegisterFilter("slug", contrib.Slug)
```

## Command

```sh
tera templates page.html context.json
tera --contrib templates page.html context.json
```

Template names are relative to the template directory, and the optional context file must contain a JSON object. Run `tera` without arguments for usage information.

## Examples and development

From the module root:

```sh
go run ./tera/examples/basic
go run ./tera/examples/dashboard
go run ./tera/examples/big_table
go run ./tera/examples/teams -iterations=1

go vet ./...
go test ./...
go test ./tera -run='^$' -bench=. -benchmem
go test ./tera -run='^$' -fuzz=FuzzTemplate -fuzztime=30s
```

The examples embed their template files, so they work from the module root. The teams example defaults to 100,000 renders, matching the original workload; use `-iterations=1` for a quick run. Native fuzz targets also include `FuzzExpression` and `FuzzTag`.

API documentation is available through `go doc github.com/Keats/tera/tera` and `go doc github.com/Keats/tera/tera-contrib`. The template language guide lives in [docs/content/_index.md](docs/content/_index.md). See [CHANGELOG.md](CHANGELOG.md) and the [contrib changelog](tera-contrib/CHANGELOG.md) for the upstream history.
