// Command basic demonstrates template inheritance, components, and escaping.
package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/Keats/tera/tera"
)

//go:embed templates
var templates embed.FS

func main() {
	engine := tera.New()
	engine.RegisterFilter("do_nothing", tera.FilterFunc(func(v tera.Value, _ tera.Kwargs, _ *tera.State) (tera.Value, error) { return v, nil }))
	var sources []tera.RawTemplate
	if err := fs.WalkDir(templates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		body, err := templates.ReadFile(path)
		if err != nil {
			return err
		}
		sources = append(sources, tera.RawTemplate{Name: strings.TrimPrefix(path, "templates/"), Content: string(body)})
		return nil
	}); err != nil {
		log.Fatal(err)
	}
	if err := engine.AddRawTemplates(sources); err != nil {
		log.Fatal(err)
	}
	c := tera.NewContext()
	if err := c.Insert("username", "Bob"); err != nil {
		log.Fatal(err)
	}
	if err := c.Insert("numbers", []int{1, 2, 3}); err != nil {
		log.Fatal(err)
	}
	if err := c.Insert("show_all", false); err != nil {
		log.Fatal(err)
	}
	if err := c.Insert("bio", "<script>alert('pwnd');</script>"); err != nil {
		log.Fatal(err)
	}
	if err := engine.RenderTo("users/profile.html", c, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
